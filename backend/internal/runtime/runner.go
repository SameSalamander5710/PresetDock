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
	// DefaultShutdownTimeout is the grace period for server shutdown.
	DefaultShutdownTimeout = 5 * time.Second
)

// Runner owns the runtime lifecycle: heartbeat monitoring and graceful shutdown.
type Runner struct {
	server *http.Server
}

// NewRunner returns a Runner wired to the given HTTP server.
func NewRunner(server *http.Server) *Runner {
	return &Runner{server: server}
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
// becomes stale. Blocks until the server is stopped externally.
func (r *Runner) MonitorHeartbeat(hb *Heartbeat) {
	ticker := time.NewTicker(DefaultHeartbeatInterval / 2)
	defer ticker.Stop()

	for range ticker.C {
		if !hb.Stale(DefaultHeartbeatTimeout) {
			continue
		}

		log.Println("No heartbeat received for 2 minutes; shutting down.")
		ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
		_ = r.server.Shutdown(ctx)
		cancel()
		return
	}
}
