package api

import (
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"presetdock/backend/internal/runtime"
)

// registerTestMux mounts the given Handler on a fresh mux, mirroring
// newTestServer but with a caller-built Handler.
func registerTestMux(t *testing.T, h *Handler) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	h.Register(mux, fstest.MapFS{})
	return mux
}

func TestHeartbeatHandler(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doJSON(t, srv, http.MethodPost, "/api/heartbeat", nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("POST /api/heartbeat = %d, want 204: %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodGet, "/api/heartbeat", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/heartbeat = %d, want 405: %s", rec.Code, rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); allow != "POST" {
		t.Errorf("Allow header = %q, want %q", allow, "POST")
	}
}

func TestShutdownHandlerInvokesCallback(t *testing.T) {
	dir := t.TempDir()
	shutdownCh := make(chan struct{})
	h := NewHandler(dir, nil, runtime.NewHeartbeat(), func() { close(shutdownCh) })
	srv := registerTestMux(t, h)

	rec := doJSON(t, srv, http.MethodPost, "/api/shutdown", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST /api/shutdown = %d, want 204: %s", rec.Code, rec.Body.String())
	}

	select {
	case <-shutdownCh:
		// shutdown callback ran (in a background goroutine).
	case <-time.After(5 * time.Second):
		t.Fatalf("shutdown callback was not invoked")
	}
}

func TestShutdownHandlerNilCallbackDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	h := NewHandler(dir, nil, runtime.NewHeartbeat(), nil)
	srv := registerTestMux(t, h)

	rec := doJSON(t, srv, http.MethodPost, "/api/shutdown", nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("POST /api/shutdown with nil callback = %d, want 204", rec.Code)
	}
}

func TestShutdownHandlerMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doJSON(t, srv, http.MethodGet, "/api/shutdown", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/shutdown = %d, want 405: %s", rec.Code, rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); allow != "POST" {
		t.Errorf("Allow header = %q, want %q", allow, "POST")
	}
}
