package decks

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func mustSave(t *testing.T, dir string, list []Deck) {
	t.Helper()
	if err := Save(dir, list); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func mustLoad(t *testing.T, dir string) []Deck {
	t.Helper()
	list, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return list
}

func hasPreset(t *testing.T, dir, deckName, presetID string) bool {
	t.Helper()
	for _, d := range mustLoad(t, dir) {
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

// --- DecksForPreset ---

func TestDecksForPreset(t *testing.T) {
	list := []Deck{
		{Name: "Alpha", PresetIDs: []string{"p1", "p2"}},
		{Name: "Beta", PresetIDs: []string{"p3"}},
		{Name: "Gamma", PresetIDs: []string{"p2", "p4"}},
	}
	got := DecksForPreset(list, "p2")
	if len(got) != 2 || got[0] != "Alpha" || got[1] != "Gamma" {
		t.Errorf("DecksForPreset(p2) = %v, want [Alpha Gamma]", got)
	}
	if got := DecksForPreset(list, "missing"); len(got) != 0 {
		t.Errorf("DecksForPreset(missing) = %v, want empty", got)
	}
}

// --- SetPresetDecks ---

func TestSetPresetDecksAddsAndRemoves(t *testing.T) {
	dir := t.TempDir()
	mustSave(t, dir, []Deck{
		{Name: "Alpha", PresetIDs: []string{"p1"}},
		{Name: "Beta", PresetIDs: []string{"p1", "p2"}},
		{Name: "Gamma", PresetIDs: []string{"p3"}},
	})

	// p1 moves from [Alpha, Beta] to [Beta, Gamma].
	if err := SetPresetDecks(dir, "p1", []string{"Beta", "Gamma"}); err != nil {
		t.Fatalf("SetPresetDecks: %v", err)
	}
	if hasPreset(t, dir, "Alpha", "p1") {
		t.Errorf("p1 should have been removed from Alpha")
	}
	if !hasPreset(t, dir, "Beta", "p1") {
		t.Errorf("p1 should stay in Beta")
	}
	if !hasPreset(t, dir, "Gamma", "p1") {
		t.Errorf("p1 should have been added to Gamma")
	}
	// Unrelated memberships untouched.
	if !hasPreset(t, dir, "Beta", "p2") {
		t.Errorf("p2 should stay in Beta")
	}
	if !hasPreset(t, dir, "Gamma", "p3") {
		t.Errorf("p3 should stay in Gamma")
	}
}

func TestSetPresetDecksCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	mustSave(t, dir, []Deck{{Name: "MyDeck", PresetIDs: []string{}}})
	if err := SetPresetDecks(dir, "p1", []string{"mydeck"}); err != nil {
		t.Fatalf("SetPresetDecks: %v", err)
	}
	if !hasPreset(t, dir, "MyDeck", "p1") {
		t.Errorf("p1 should have been added to MyDeck via case-insensitive match")
	}
}

func TestSetPresetDecksIgnoresUnknownNames(t *testing.T) {
	dir := t.TempDir()
	mustSave(t, dir, []Deck{{Name: "Alpha", PresetIDs: []string{"p1"}}})
	if err := SetPresetDecks(dir, "p1", []string{"Alpha", "Nope"}); err != nil {
		t.Fatalf("SetPresetDecks: %v", err)
	}
	if !hasPreset(t, dir, "Alpha", "p1") {
		t.Errorf("p1 should stay in Alpha")
	}
	for _, d := range mustLoad(t, dir) {
		if d.Name == "Nope" {
			t.Errorf("unknown deck name should not create a deck, got %+v", d)
		}
	}
}

func TestSetPresetDecksEmptyListRemovesFromAll(t *testing.T) {
	dir := t.TempDir()
	mustSave(t, dir, []Deck{
		{Name: "Alpha", PresetIDs: []string{"p1", "p2"}},
		{Name: "Beta", PresetIDs: []string{"p1"}},
	})
	if err := SetPresetDecks(dir, "p1", []string{}); err != nil {
		t.Fatalf("SetPresetDecks: %v", err)
	}
	if hasPreset(t, dir, "Alpha", "p1") {
		t.Errorf("p1 should have been removed from Alpha")
	}
	if hasPreset(t, dir, "Beta", "p1") {
		t.Errorf("p1 should have been removed from Beta")
	}
	if !hasPreset(t, dir, "Alpha", "p2") {
		t.Errorf("p2 should stay in Alpha")
	}
}

func TestSetPresetDecksNoChangeDoesNotRewrite(t *testing.T) {
	dir := t.TempDir()
	mustSave(t, dir, []Deck{{Name: "Alpha", PresetIDs: []string{"p1"}}})
	before, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	if err := SetPresetDecks(dir, "p1", []string{"Alpha"}); err != nil {
		t.Fatalf("SetPresetDecks: %v", err)
	}
	after, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("file should not be rewritten when membership is unchanged")
	}
}

func TestSetPresetDecksEmptyPresetIDIsNoop(t *testing.T) {
	dir := t.TempDir()
	mustSave(t, dir, []Deck{{Name: "Alpha", PresetIDs: []string{"p1"}}})
	if err := SetPresetDecks(dir, "", []string{"Alpha"}); err != nil {
		t.Fatalf("SetPresetDecks: %v", err)
	}
	if hasPreset(t, dir, "Alpha", "") {
		t.Errorf("empty preset ID should never be added")
	}
}

// --- RemovePreset ---

func TestRemovePresetStripsFromAllDecks(t *testing.T) {
	dir := t.TempDir()
	mustSave(t, dir, []Deck{
		{Name: "Alpha", PresetIDs: []string{"p1", "p2"}},
		{Name: "Beta", PresetIDs: []string{"p1"}},
		{Name: "Gamma", PresetIDs: []string{"p3"}},
	})
	if err := RemovePreset(dir, "p1"); err != nil {
		t.Fatalf("RemovePreset: %v", err)
	}
	if hasPreset(t, dir, "Alpha", "p1") {
		t.Errorf("p1 should have been removed from Alpha")
	}
	if hasPreset(t, dir, "Beta", "p1") {
		t.Errorf("p1 should have been removed from Beta")
	}
	if !hasPreset(t, dir, "Alpha", "p2") {
		t.Errorf("p2 should stay in Alpha")
	}
	if !hasPreset(t, dir, "Gamma", "p3") {
		t.Errorf("p3 should stay in Gamma")
	}
}

func TestRemovePresetNoChangeDoesNotRewrite(t *testing.T) {
	dir := t.TempDir()
	mustSave(t, dir, []Deck{{Name: "Alpha", PresetIDs: []string{"p1"}}})
	before, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	if err := RemovePreset(dir, "missing"); err != nil {
		t.Fatalf("RemovePreset: %v", err)
	}
	after, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("file should not be rewritten when the ID is not present")
	}
}

// --- Load: a corrupt file is quarantined, not fatal ---

func TestLoadCorruptFileIsQuarantined(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(Path(dir), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("seed corrupt file: %v", err)
	}

	list, err := Load(dir)
	if err != nil {
		t.Fatalf("Load should not fail on a corrupt file, got: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("Load = %v, want empty", list)
	}
	if _, err := os.Stat(Path(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("corrupt decks.json should be moved aside, stat err = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 || !strings.Contains(entries[0].Name(), ".corrupt-") {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected one quarantined file, dir = %v", names)
	}
}
