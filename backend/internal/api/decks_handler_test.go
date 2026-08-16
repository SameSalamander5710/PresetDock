package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newRawRequest builds a request with a raw (possibly invalid) body so
// malformed-JSON paths can be exercised.
func newRawRequest(t *testing.T, method, path, raw string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(raw)))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func srvRoundTrip(t *testing.T, srv http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// --- GET /api/decks ---

func TestDecksListEmpty(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doJSON(t, srv, http.MethodGet, "/api/decks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/decks = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("empty deck list expected, got %v", list)
	}
}

// --- POST /api/decks ---

func TestCreateDeck(t *testing.T) {
	srv, dir := newTestServer(t)
	rec := doJSON(t, srv, http.MethodPost, "/api/decks", map[string]any{"name": "Alpha"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/decks = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	// Nil preset_ids must be persisted as an empty array, not null.
	got := loadDecks(t, dir)
	if len(got) != 1 || got[0].Name != "Alpha" {
		t.Fatalf("decks on disk = %+v, want one deck named Alpha", got)
	}
	if got[0].PresetIDs == nil || len(got[0].PresetIDs) != 0 {
		t.Errorf("PresetIDs = %v, want empty non-nil slice", got[0].PresetIDs)
	}
}

func TestCreateDeckRequiresName(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, name := range []string{"", "   "} {
		rec := doJSON(t, srv, http.MethodPost, "/api/decks", map[string]any{"name": name})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("POST /api/decks name=%q = %d, want 400: %s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestCreateDeckRejectsDuplicateCaseInsensitive(t *testing.T) {
	srv, _ := newTestServer(t)
	if rec := doJSON(t, srv, http.MethodPost, "/api/decks", map[string]any{"name": "Alpha"}); rec.Code != http.StatusCreated {
		t.Fatalf("setup POST = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, srv, http.MethodPost, "/api/decks", map[string]any{"name": "alpha"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("duplicate deck (case-insensitive) = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateDeckInvalidJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	req := newRawRequest(t, http.MethodPost, "/api/decks", "{not json")
	rec := srvRoundTrip(t, srv, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// --- PUT /api/decks/:name ---

func TestUpdateDeckRenameCaseInsensitive(t *testing.T) {
	srv, dir := newTestServer(t)
	seedDecks(t, dir, "Alpha")

	// Lookup is case-insensitive; body name wins.
	rec := doJSON(t, srv, http.MethodPut, "/api/decks/alpha", map[string]any{
		"name":       "Renamed",
		"preset_ids": []string{"p1"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/decks/alpha = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	got := loadDecks(t, dir)
	if len(got) != 1 || got[0].Name != "Renamed" {
		t.Fatalf("decks on disk = %+v, want single deck named Renamed", got)
	}
	if len(got[0].PresetIDs) != 1 || got[0].PresetIDs[0] != "p1" {
		t.Errorf("PresetIDs = %v, want [p1]", got[0].PresetIDs)
	}
}

func TestUpdateDeckRejectsRenameOntoExisting(t *testing.T) {
	srv, dir := newTestServer(t)
	seedDecks(t, dir, "Alpha", "Beta")

	rec := doJSON(t, srv, http.MethodPut, "/api/decks/Alpha", map[string]any{"name": "beta"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("rename onto existing deck = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateDeckUnknown404(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doJSON(t, srv, http.MethodPut, "/api/decks/Nope", map[string]any{"name": "Nope"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("PUT unknown deck = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateDeckRequiresName(t *testing.T) {
	srv, dir := newTestServer(t)
	seedDecks(t, dir, "Alpha")
	rec := doJSON(t, srv, http.MethodPut, "/api/decks/Alpha", map[string]any{"name": "  "})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("PUT empty name = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// --- DELETE /api/decks/:name ---

func TestDeleteDeck(t *testing.T) {
	srv, dir := newTestServer(t)
	seedDecks(t, dir, "Alpha", "Beta")

	rec := doJSON(t, srv, http.MethodDelete, "/api/decks/alpha", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/decks/alpha = %d, want 204: %s", rec.Code, rec.Body.String())
	}
	got := loadDecks(t, dir)
	if len(got) != 1 || got[0].Name != "Beta" {
		t.Errorf("decks on disk = %+v, want only Beta", got)
	}
}

func TestDeleteDeckUnknown404(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doJSON(t, srv, http.MethodDelete, "/api/decks/Nope", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("DELETE unknown deck = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

// --- Routing guards ---

func TestDeckByNamePathTraversal404(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doJSON(t, srv, http.MethodDelete, "/api/decks/a/b", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("path with slash = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestDecksMethodNotAllowed(t *testing.T) {
	srv, dir := newTestServer(t)
	rec := doJSON(t, srv, http.MethodDelete, "/api/decks", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /api/decks = %d, want 405: %s", rec.Code, rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); allow != "GET, POST" {
		t.Errorf("Allow header = %q, want %q", allow, "GET, POST")
	}

	// The by-name handler resolves the deck before checking the method, so
	// the deck must exist for the request to reach the 405 branch.
	seedDecks(t, dir, "Alpha")
	rec = doJSON(t, srv, http.MethodGet, "/api/decks/Alpha", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/decks/Alpha = %d, want 405: %s", rec.Code, rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); allow != "PUT, DELETE" {
		t.Errorf("Allow header = %q, want %q", allow, "PUT, DELETE")
	}
}
