package runtime

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"
)

const (
	// DefaultHeartbeatInterval is the period between heartbeat stale checks.
	DefaultHeartbeatInterval = 30 * time.Second
	// DefaultHeartbeatTimeout is how long without a heartbeat before shutdown.
	DefaultHeartbeatTimeout = 2 * time.Minute
	// DefaultWakeGrace is how long to wait for a fresh heartbeat after
	// detecting that the check loop itself was suspended (e.g. system
	// sleep). It mirrors the client-side heartbeat interval in
	// frontend/heartbeat.js, so an open tab has one beat window to
	// re-assert liveness after wake.
	DefaultWakeGrace = 30 * time.Second
	// DefaultShutdownTimeout is the grace period for server shutdown.
	DefaultShutdownTimeout = 5 * time.Second
)

// Runner owns the runtime lifecycle: heartbeat monitoring and graceful shutdown.
type Runner struct {
	server        *http.Server
	checkInterval time.Duration
	staleTimeout  time.Duration
	wakeGrace     time.Duration
	lastCheck     time.Time
}

// NewRunner returns a Runner wired to the given HTTP server.
func NewRunner(server *http.Server) *Runner {
	return &Runner{
		server:        server,
		checkInterval: DefaultHeartbeatInterval / 2,
		staleTimeout:  DefaultHeartbeatTimeout,
		wakeGrace:     DefaultWakeGrace,
		lastCheck:     time.Now(),
	}
}

// Shutdown performs a graceful shutdown with the default timeout.
func (r *Runner) Shutdown() {
	time.Sleep(150 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()
	if err := r.server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("shutdown error: %v", err)
	}
}

// MonitorHeartbeat runs a ticker that shuts down the server when the heartbeat
// becomes stale. If the check loop itself was suspended (system sleep), it
// waits one wake-grace window for a fresh heartbeat before shutting down, so
// a still-open browser tab survives a sleep. Blocks until the server is
// stopped externally.
func (r *Runner) MonitorHeartbeat(hb *Heartbeat) {
	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		checkGap := now.Sub(r.lastCheck)
		r.lastCheck = now

		if !hb.Stale(r.staleTimeout) {
			continue
		}

		if needsWakeGrace(checkGap, r.checkInterval) {
			time.Sleep(r.wakeGrace)
			r.lastCheck = time.Now()
			if !hb.Stale(r.staleTimeout) {
				continue
			}
		}

		log.Printf("No heartbeat received for %s; shutting down.", r.staleTimeout)
		ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
		_ = r.server.Shutdown(ctx)
		cancel()
		return
	}
}

// needsWakeGrace reports whether a stale heartbeat is most likely the result
// of the check loop being suspended (system sleep) rather than a closed
// client. A normally-running loop checks every checkInterval, so a gap much
// larger than that means the process itself was asleep.
func needsWakeGrace(checkGap, checkInterval time.Duration) bool {
	return checkGap > 2*checkInterval
}
