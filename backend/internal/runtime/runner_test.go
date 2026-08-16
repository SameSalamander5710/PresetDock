package runtime

import (
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

// startTestServer binds a real HTTP server to an ephemeral localhost port and
// returns the server, the listener, and a channel that receives the result of
// Serve (sent when the server stops).
func startTestServer(t *testing.T) (*http.Server, net.Listener, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(ln)
	}()
	t.Cleanup(func() {
		// Best-effort stop in case the test did not shut the server down.
		_ = server.Close()
	})
	return server, ln, serveErr
}

func TestShutdownStopsServer(t *testing.T) {
	server, ln, serveErr := startTestServer(t)

	// The server must actually be accepting connections before shutdown.
	resp, err := http.Get("http://" + ln.Addr().String())
	if err != nil {
		t.Fatalf("probe server: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("probe status = %d, want 200", resp.StatusCode)
	}

	runner := NewRunner(server)
	runner.Shutdown()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Serve returned %v, want http.ErrServerClosed", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("server did not stop after Shutdown")
	}
}

func TestNeedsWakeGrace(t *testing.T) {
	cases := []struct {
		name          string
		checkGap      time.Duration
		checkInterval time.Duration
		want          bool
	}{
		{"normal tick", 15 * time.Second, 15 * time.Second, false},
		{"slightly late tick", 20 * time.Second, 15 * time.Second, false},
		{"exactly double interval", 30 * time.Second, 15 * time.Second, false},
		{"gap after suspension", 90 * time.Second, 15 * time.Second, true},
		{"zero gap", 0, 15 * time.Second, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := needsWakeGrace(tc.checkGap, tc.checkInterval); got != tc.want {
				t.Errorf("needsWakeGrace(%v, %v) = %v, want %v", tc.checkGap, tc.checkInterval, got, tc.want)
			}
		})
	}
}

// ageHeartbeat sets the last beat to d in the past, as if no heartbeat had
// arrived for that long (e.g. the machine slept).
func ageHeartbeat(hb *Heartbeat, d time.Duration) {
	hb.mu.Lock()
	hb.lastBeat = time.Now().Add(-d)
	hb.mu.Unlock()
}

// startBeater touches the heartbeat every interval until stop is closed.
func startBeater(t *testing.T, hb *Heartbeat, interval time.Duration, stop <-chan struct{}) {
	t.Helper()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				hb.Touch()
			case <-stop:
				return
			}
		}
	}()
}

// TestMonitorHeartbeatSurvivesWakeAfterSleep simulates a system sleep: the
// heartbeat is stale and the check loop has a large gap (as if it had been
// suspended), then heartbeats resume during the wake grace. The server must
// keep running.
func TestMonitorHeartbeatSurvivesWakeAfterSleep(t *testing.T) {
	server, _, _ := startTestServer(t)

	hb := NewHeartbeat()
	ageHeartbeat(hb, time.Minute)

	runner := NewRunner(server)
	runner.checkInterval = 20 * time.Millisecond
	runner.staleTimeout = 150 * time.Millisecond
	runner.wakeGrace = 200 * time.Millisecond
	runner.lastCheck = time.Now().Add(-time.Minute) // check loop "was asleep"

	done := make(chan struct{})
	go func() {
		runner.MonitorHeartbeat(hb)
		close(done)
	}()

	stop := make(chan struct{})
	startBeater(t, hb, 30*time.Millisecond, stop)

	select {
	case <-done:
		t.Fatal("server shut down even though heartbeats resumed during the wake grace")
	case <-time.After(600 * time.Millisecond):
	}

	// Stop the beater; the monitor must now observe a stale heartbeat and
	// shut the server down (also proves the monitor is still active).
	close(stop)
	ageHeartbeat(hb, time.Hour)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down after heartbeats stopped")
	}
}

// TestMonitorHeartbeatShutsDownWhenNoWake is the control case: stale heartbeat
// plus a suspended-loop gap, but no heartbeat arrives during the wake grace.
func TestMonitorHeartbeatShutsDownWhenNoWake(t *testing.T) {
	server, _, serveErr := startTestServer(t)

	hb := NewHeartbeat()
	ageHeartbeat(hb, time.Minute)

	runner := NewRunner(server)
	runner.checkInterval = 20 * time.Millisecond
	runner.staleTimeout = 150 * time.Millisecond
	runner.wakeGrace = 100 * time.Millisecond
	runner.lastCheck = time.Now().Add(-time.Minute)

	go runner.MonitorHeartbeat(hb)

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Serve returned %v, want http.ErrServerClosed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down after stale heartbeat with no wake")
	}
}
