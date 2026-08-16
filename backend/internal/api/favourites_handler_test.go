package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"presetdock/backend/internal/favourites"
)

// --- GET /api/favourites ---

func TestFavouritesListEmpty(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doJSON(t, srv, http.MethodGet, "/api/favourites", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/favourites = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var list []string
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("empty favourite list expected, got %v", list)
	}
}

// --- POST /api/favourites ---

func TestAddFavourite(t *testing.T) {
	srv, dir := newTestServer(t)
	rec := doJSON(t, srv, http.MethodPost, "/api/favourites", map[string]any{"preset_id": "abc-1234567"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/favourites = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !favContains(t, dir, "abc-1234567") {
		t.Errorf("favourite should be persisted on disk")
	}
}

func TestAddFavouriteIdempotent(t *testing.T) {
	srv, dir := newTestServer(t)
	for i := 0; i < 2; i++ {
		rec := doJSON(t, srv, http.MethodPost, "/api/favourites", map[string]any{"preset_id": "abc-1234567"})
		if rec.Code != http.StatusOK {
			t.Fatalf("POST /api/favourites (attempt %d) = %d, want 200: %s", i+1, rec.Code, rec.Body.String())
		}
	}
	favs, err := favourites.Load(dir)
	if err != nil {
		t.Fatalf("load favourites: %v", err)
	}
	if len(favs) != 1 {
		t.Errorf("duplicate POST should not append twice, got %v", favs)
	}
}

func TestAddFavouriteRequiresPresetID(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doJSON(t, srv, http.MethodPost, "/api/favourites", map[string]any{"preset_id": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST empty preset_id = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestAddFavouriteInvalidJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	req := newRawRequest(t, http.MethodPost, "/api/favourites", "{not json")
	rec := srvRoundTrip(t, srv, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// --- DELETE /api/favourites/:id ---

func TestRemoveFavourite(t *testing.T) {
	srv, dir := newTestServer(t)
	if rec := doJSON(t, srv, http.MethodPost, "/api/favourites", map[string]any{"preset_id": "keep-1"}); rec.Code != http.StatusOK {
		t.Fatalf("setup POST keep-1 = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodPost, "/api/favourites", map[string]any{"preset_id": "drop-2"}); rec.Code != http.StatusOK {
		t.Fatalf("setup POST drop-2 = %d: %s", rec.Code, rec.Body.String())
	}

	rec := doJSON(t, srv, http.MethodDelete, "/api/favourites/drop-2", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE /api/favourites/drop-2 = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var remaining []string
	if err := json.Unmarshal(rec.Body.Bytes(), &remaining); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(remaining) != 1 || remaining[0] != "keep-1" {
		t.Errorf("response = %v, want [keep-1]", remaining)
	}
	if favContains(t, dir, "drop-2") {
		t.Errorf("removed favourite should not remain on disk")
	}
}

func TestRemoveFavouriteUnknown404(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doJSON(t, srv, http.MethodDelete, "/api/favourites/nope", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("DELETE unknown favourite = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestFavouriteByIDPathTraversal404(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doJSON(t, srv, http.MethodDelete, "/api/favourites/a/b", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("path with slash = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestFavouritesMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doJSON(t, srv, http.MethodDelete, "/api/favourites", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /api/favourites = %d, want 405: %s", rec.Code, rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); allow != "GET, POST" {
		t.Errorf("Allow header = %q, want %q", allow, "GET, POST")
	}

	rec = doJSON(t, srv, http.MethodGet, "/api/favourites/abc", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/favourites/abc = %d, want 405: %s", rec.Code, rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); allow != "DELETE" {
		t.Errorf("Allow header = %q, want %q", allow, "DELETE")
	}
}
