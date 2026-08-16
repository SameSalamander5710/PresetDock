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
