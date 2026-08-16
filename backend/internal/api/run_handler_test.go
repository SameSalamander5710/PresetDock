package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"presetdock/backend/internal/launcher"
	"presetdock/backend/internal/presets"
	"presetdock/backend/internal/runtime"
)

// fakeLauncher records the payload it is given and returns a real *exec.Cmd
// that exits immediately (the test binary itself with no matching tests), so
// the handler's Start/Wait/cleanup path is exercised without spawning a real
// console window.
type fakeLauncher struct {
	mu         sync.Mutex
	payload    launcher.CommandPayload
	gotPayload bool
	prepareErr error
	cleanupCh  chan struct{}
}

func (f *fakeLauncher) Prepare(p launcher.CommandPayload) (*exec.Cmd, func(), error) {
	f.mu.Lock()
	f.payload = p
	f.gotPayload = true
	prepareErr := f.prepareErr
	cleanupCh := f.cleanupCh
	f.mu.Unlock()

	if prepareErr != nil {
		return nil, nil, prepareErr
	}
	cmd := exec.Command(os.Args[0], "-test.run=NONE")
	cleanup := func() {}
	if cleanupCh != nil {
		cleanup = func() { close(cleanupCh) }
	}
	return cmd, cleanup, nil
}

func (f *fakeLauncher) lastPayload() launcher.CommandPayload {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.payload
}

func (f *fakeLauncher) called() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.gotPayload
}

// newRunTestServer is like newTestServer but wires the given launcher.
func newRunTestServer(t *testing.T, l launcher.Launcher) (http.Handler, string) {
	t.Helper()
	dir := t.TempDir()
	h := NewHandler(dir, l, runtime.NewHeartbeat(), func() {})
	mux := http.NewServeMux()
	h.Register(mux, fstest.MapFS{})
	return mux, dir
}

func assertStarted(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("run request = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var out map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out["status"] != "started" {
		t.Errorf("status = %q, want %q", out["status"], "started")
	}
}

// --- POST /api/run (direct command) ---

func TestRunDirectSuccess(t *testing.T) {
	fake := &fakeLauncher{}
	srv, _ := newRunTestServer(t, fake)

	rec := doJSON(t, srv, http.MethodPost, "/api/run", map[string]any{
		"name":    "My Title",
		"command": "echo hi",
	})
	assertStarted(t, rec)

	got := fake.lastPayload()
	if got.Title != "My Title" || got.Command != "echo hi" || got.Shell != "" {
		t.Errorf("payload = %+v, want title %q, command %q, empty shell", got, "My Title", "echo hi")
	}
}

func TestRunDirectEmptyCommand(t *testing.T) {
	fake := &fakeLauncher{}
	srv, _ := newRunTestServer(t, fake)
	for _, command := range []string{"", "   "} {
		rec := doJSON(t, srv, http.MethodPost, "/api/run", map[string]any{"command": command})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("POST /api/run command=%q = %d, want 400: %s", command, rec.Code, rec.Body.String())
		}
	}
	if fake.called() {
		t.Errorf("launcher must not be called for an empty command")
	}
}

func TestRunDirectInvalidJSON(t *testing.T) {
	fake := &fakeLauncher{}
	srv, _ := newRunTestServer(t, fake)
	req := newRawRequest(t, http.MethodPost, "/api/run", "{not json")
	rec := srvRoundTrip(t, srv, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestRunDirectPrepareError(t *testing.T) {
	fake := &fakeLauncher{prepareErr: errors.New("boom")}
	srv, _ := newRunTestServer(t, fake)
	rec := doJSON(t, srv, http.MethodPost, "/api/run", map[string]any{"command": "echo hi"})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Prepare error = %d, want 500: %s", rec.Code, rec.Body.String())
	}
}

func TestRunDirectMethodNotAllowed(t *testing.T) {
	fake := &fakeLauncher{}
	srv, _ := newRunTestServer(t, fake)
	rec := doJSON(t, srv, http.MethodGet, "/api/run", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/run = %d, want 405: %s", rec.Code, rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); allow != "POST" {
		t.Errorf("Allow header = %q, want %q", allow, "POST")
	}
}

// --- POST /api/run/:id (saved preset) ---

func TestRunByIDUnknown(t *testing.T) {
	fake := &fakeLauncher{}
	srv, _ := newRunTestServer(t, fake)
	rec := doJSON(t, srv, http.MethodPost, "/api/run/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("POST /api/run/does-not-exist = %d, want 404: %s", rec.Code, rec.Body.String())
	}
	if fake.called() {
		t.Errorf("launcher must not be called for an unknown preset")
	}
}

func TestRunByIDPathTraversal404(t *testing.T) {
	fake := &fakeLauncher{}
	srv, _ := newRunTestServer(t, fake)
	rec := doJSON(t, srv, http.MethodPost, "/api/run/a/b", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("path with slash = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestRunByIDMethodNotAllowed(t *testing.T) {
	fake := &fakeLauncher{}
	srv, _ := newRunTestServer(t, fake)
	rec := doJSON(t, srv, http.MethodGet, "/api/run/abc", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/run/abc = %d, want 405: %s", rec.Code, rec.Body.String())
	}
}

func TestRunByIDPassesPresetFields(t *testing.T) {
	fake := &fakeLauncher{}
	srv, dir := newRunTestServer(t, fake)

	view, err := presets.Save(dir, presets.Preset{
		Name:    "PS Model",
		Model:   "some/model",
		Command: "pwsh -NoProfile",
		Shell:   "powershell",
	})
	if err != nil {
		t.Fatalf("presets.Save: %v", err)
	}

	rec := doJSON(t, srv, http.MethodPost, "/api/run/"+view.ID, nil)
	assertStarted(t, rec)

	got := fake.lastPayload()
	if got.Title != "PS Model" || got.Command != "pwsh -NoProfile" || got.Shell != "powershell" {
		t.Errorf("payload = %+v, want title %q, command %q, shell %q",
			got, "PS Model", "pwsh -NoProfile", "powershell")
	}
}

// --- fire-and-forget cleanup ---

func TestRunCleanupInvokedAfterExit(t *testing.T) {
	fake := &fakeLauncher{cleanupCh: make(chan struct{})}
	srv, _ := newRunTestServer(t, fake)

	rec := doJSON(t, srv, http.MethodPost, "/api/run", map[string]any{"command": "echo hi"})
	assertStarted(t, rec)

	select {
	case <-fake.cleanupCh:
		// cleanup ran after the command exited.
	case <-time.After(10 * time.Second):
		t.Fatalf("cleanup callback was not invoked after the command exited")
	}
}
