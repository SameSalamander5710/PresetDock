package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"presetdock/backend/internal/decks"
	"presetdock/backend/internal/favourites"
	"presetdock/backend/internal/runtime"
)

// newTestServer builds a Handler backed by a temp presets dir and returns it
// behind a real mux so route wiring is exercised too.
func newTestServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	dir := t.TempDir()
	h := NewHandler(dir, nil, runtime.NewHeartbeat(), func() {})
	mux := http.NewServeMux()
	h.Register(mux, fstest.MapFS{})
	return mux, dir
}

func doJSON(t *testing.T, srv http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func loadDecks(t *testing.T, dir string) []decks.Deck {
	t.Helper()
	list, err := decks.Load(dir)
	if err != nil {
		t.Fatalf("decks.Load: %v", err)
	}
	return list
}

func deckContains(t *testing.T, dir, deckName, presetID string) bool {
	t.Helper()
	for _, d := range loadDecks(t, dir) {
		if d.Name == deckName {
			for _, id := range d.PresetIDs {
				if id == presetID {
					return true
				}
			}
			return false
		}
	}
	t.Fatalf("deck %q not found", deckName)
	return false
}

func favContains(t *testing.T, dir, presetID string) bool {
	t.Helper()
	favs, err := favourites.Load(dir)
	if err != nil {
		t.Fatalf("favourites.Load: %v", err)
	}
	for _, id := range favs {
		if id == presetID {
			return true
		}
	}
	return false
}

func seedDecks(t *testing.T, dir string, names ...string) {
	t.Helper()
	list := make([]decks.Deck, 0, len(names))
	for _, n := range names {
		list = append(list, decks.Deck{Name: n, PresetIDs: []string{}})
	}
	if err := decks.Save(dir, list); err != nil {
		t.Fatalf("seed decks: %v", err)
	}
}

func postPreset(t *testing.T, srv http.Handler, body map[string]any) map[string]any {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPost, "/api/presets", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/presets = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func putPreset(t *testing.T, srv http.Handler, id string, body map[string]any) map[string]any {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPut, "/api/presets/"+id, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/presets/%s = %d, want 200: %s", id, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func TestCreatePresetWithDeckNames(t *testing.T) {
	srv, dir := newTestServer(t)
	seedDecks(t, dir, "Alpha", "Beta")

	out := postPreset(t, srv, map[string]any{
		"name":       "Test Model",
		"command":    "echo hi",
		"deck_names": []string{"Alpha", "Nope"},
	})
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatalf("response missing id: %v", out)
	}

	if !deckContains(t, dir, "Alpha", id) {
		t.Errorf("new preset should be in Alpha")
	}
	if deckContains(t, dir, "Beta", id) {
		t.Errorf("new preset should not be in Beta")
	}
	for _, d := range loadDecks(t, dir) {
		if d.Name == "Nope" {
			t.Errorf("unknown deck name should be ignored, got deck %+v", d)
		}
	}
}

func TestUpdatePresetDeckNames(t *testing.T) {
	srv, dir := newTestServer(t)
	seedDecks(t, dir, "Alpha", "Beta")

	created := postPreset(t, srv, map[string]any{
		"name":       "Test Model",
		"command":    "echo hi",
		"deck_names": []string{"Alpha"},
	})
	id, _ := created["id"].(string)

	// Move membership from Alpha to Beta.
	putPreset(t, srv, id, map[string]any{
		"name":       "Test Model",
		"command":    "echo hi",
		"deck_names": []string{"Beta"},
	})
	if deckContains(t, dir, "Alpha", id) {
		t.Errorf("preset should have been removed from Alpha")
	}
	if !deckContains(t, dir, "Beta", id) {
		t.Errorf("preset should have been added to Beta")
	}

	// Absent deck_names leaves membership untouched.
	putPreset(t, srv, id, map[string]any{
		"name":    "Test Model",
		"command": "echo hi",
	})
	if !deckContains(t, dir, "Beta", id) {
		t.Errorf("membership should be untouched when deck_names is absent")
	}

	// Explicit empty list removes from all decks.
	putPreset(t, srv, id, map[string]any{
		"name":       "Test Model",
		"command":    "echo hi",
		"deck_names": []string{},
	})
	if deckContains(t, dir, "Beta", id) {
		t.Errorf("empty deck_names should remove the preset from all decks")
	}
}

func TestUpdatePresetRenameAndDeckNamesCompose(t *testing.T) {
	srv, dir := newTestServer(t)
	seedDecks(t, dir, "Alpha", "Beta")

	created := postPreset(t, srv, map[string]any{
		"name":       "Old Name",
		"command":    "echo hi",
		"deck_names": []string{"Alpha"},
	})
	oldID, _ := created["id"].(string)

	// Rename and change membership in the same save.
	out := putPreset(t, srv, oldID, map[string]any{
		"name":       "New Name",
		"command":    "echo hi",
		"deck_names": []string{"Beta"},
	})
	newID, _ := out["id"].(string)
	if newID == "" || newID == oldID {
		t.Fatalf("expected a new ID after rename, got %q (old %q)", newID, oldID)
	}

	// The remapped reference must end up in Beta, not Alpha.
	if !deckContains(t, dir, "Beta", newID) {
		t.Errorf("renamed preset should be in Beta under its new ID")
	}
	if deckContains(t, dir, "Alpha", newID) {
		t.Errorf("renamed preset should not remain in Alpha")
	}
	if deckContains(t, dir, "Alpha", oldID) {
		t.Errorf("old ID should not remain in Alpha")
	}
}

func TestDeletePresetCleansUpRelations(t *testing.T) {
	srv, dir := newTestServer(t)
	seedDecks(t, dir, "Alpha")

	created := postPreset(t, srv, map[string]any{
		"name":       "Test Model",
		"command":    "echo hi",
		"deck_names": []string{"Alpha"},
	})
	id, _ := created["id"].(string)

	if rec := doJSON(t, srv, http.MethodPost, "/api/favourites", map[string]any{"preset_id": id}); rec.Code != http.StatusOK {
		t.Fatalf("POST /api/favourites = %d: %s", rec.Code, rec.Body.String())
	}
	if !favContains(t, dir, id) {
		t.Fatalf("setup: preset should be a favourite")
	}

	rec := doJSON(t, srv, http.MethodDelete, "/api/presets/"+id, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/presets/%s = %d, want 204: %s", id, rec.Code, rec.Body.String())
	}

	if deckContains(t, dir, "Alpha", id) {
		t.Errorf("deleted preset should be removed from decks.json")
	}
	if favContains(t, dir, id) {
		t.Errorf("deleted preset should be removed from favourites.json")
	}
}

func TestDuplicatePropagatesDeckMembership(t *testing.T) {
	srv, dir := newTestServer(t)
	seedDecks(t, dir, "Alpha", "Beta")

	source := postPreset(t, srv, map[string]any{
		"name":       "Source Model",
		"command":    "echo hi",
		"deck_names": []string{"Alpha", "Beta"},
	})
	sourceID, _ := source["id"].(string)

	// Plain duplicate: membership is copied.
	dup := postPreset(t, srv, map[string]any{
		"name":             "Copy Model",
		"command":          "echo hi",
		"source_preset_id": sourceID,
	})
	dupID, _ := dup["id"].(string)
	if !deckContains(t, dir, "Alpha", dupID) || !deckContains(t, dir, "Beta", dupID) {
		t.Errorf("duplicated preset should inherit both decks")
	}

	// Explicit deck_names takes final say over the propagated set.
	dup2 := postPreset(t, srv, map[string]any{
		"name":             "Copy 2 Model",
		"command":          "echo hi",
		"source_preset_id": sourceID,
		"deck_names":       []string{"Alpha"},
	})
	dup2ID, _ := dup2["id"].(string)
	if !deckContains(t, dir, "Alpha", dup2ID) {
		t.Errorf("explicit deck_names should keep the preset in Alpha")
	}
	if deckContains(t, dir, "Beta", dup2ID) {
		t.Errorf("explicit deck_names should override propagated Beta membership")
	}
}
