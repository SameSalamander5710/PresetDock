package presets

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"presetdock/backend/internal/decks"
	"presetdock/backend/internal/favourites"
)

var uidPattern = regexp.MustCompile(`^[0-9a-f]{7}$`)

// --- helpers ---

func writeJSON(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustSave(t *testing.T, dir string, p Preset) PresetView {
	t.Helper()
	v, err := Save(dir, p)
	if err != nil {
		t.Fatalf("Save(%q): %v", p.Name, err)
	}
	return v
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// --- Slugify ---

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Example":         "example",
		"Qwen 3.5 9B Q6":  "qwen-3-5-9b-q6",
		"MTP Benchmark":   "mtp-benchmark",
		"--Hello_World--": "hello-world",
		"  spaced  out  ": "spaced-out",
		"GEMMA!!":         "gemma",
		"":                "",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- NewUID ---

func TestNewUID(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		uid := NewUID()
		if !uidPattern.MatchString(uid) {
			t.Fatalf("NewUID() = %q, want 7 lowercase hex chars", uid)
		}
		if seen[uid] {
			t.Fatalf("NewUID() produced a duplicate: %q", uid)
		}
		seen[uid] = true
	}
}

// --- Save: same-named presets never collide (core bug) ---

func TestSaveSameNameNoCollision(t *testing.T) {
	dir := t.TempDir()
	v1 := mustSave(t, dir, Preset{Name: "Preset X", Command: "cmd1"})
	v2 := mustSave(t, dir, Preset{Name: "Preset X", Command: "cmd2"})

	if v1.ID == v2.ID {
		t.Fatalf("expected distinct IDs for same-named presets, both = %q", v1.ID)
	}
	if !strings.HasPrefix(v1.ID, "preset-x-") {
		t.Errorf("expected ID prefix %q, got %q", "preset-x-", v1.ID)
	}
	if !fileExists(filepath.Join(dir, v1.ID+".json")) {
		t.Errorf("file for %q missing on disk", v1.ID)
	}
	if !fileExists(filepath.Join(dir, v2.ID+".json")) {
		t.Errorf("file for %q missing on disk", v2.ID)
	}
}

func TestSaveRespectsExplicitUID(t *testing.T) {
	dir := t.TempDir()
	v := mustSave(t, dir, Preset{Name: "Explicit", Command: "cmd", UID: "abcd123"})
	if v.ID != "explicit-abcd123" {
		t.Errorf("expected explicit-abcd123, got %q", v.ID)
	}
	if !fileExists(filepath.Join(dir, "explicit-abcd123.json")) {
		t.Errorf("file explicit-abcd123.json missing on disk")
	}
}

// --- Update: renaming a preset renames its storage file ---

func TestUpdateRenamesFile(t *testing.T) {
	dir := t.TempDir()
	v := mustSave(t, dir, Preset{Name: "Old Name", Command: "cmd"})
	oldPath := filepath.Join(dir, v.ID+".json")
	if !fileExists(oldPath) {
		t.Fatalf("expected %s to exist before rename", oldPath)
	}

	v2, err := Update(dir, v.ID, Preset{Name: "New Name", Command: "cmd"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if v2.ID == v.ID {
		t.Fatalf("expected ID to change on rename, still %q", v2.ID)
	}
	if !strings.HasPrefix(v2.ID, "new-name-") {
		t.Errorf("expected new ID prefix %q, got %q", "new-name-", v2.ID)
	}
	if fileExists(oldPath) {
		t.Errorf("old file %q should be removed after rename", oldPath)
	}
	if !fileExists(filepath.Join(dir, v2.ID+".json")) {
		t.Errorf("new file for %q missing on disk", v2.ID)
	}
	if v2.UID != v.UID {
		t.Errorf("uid changed across rename: %q -> %q", v.UID, v2.UID)
	}
}

func TestUpdateSameNameKeepsID(t *testing.T) {
	dir := t.TempDir()
	v := mustSave(t, dir, Preset{Name: "Stable", Command: "old cmd"})
	v2, err := Update(dir, v.ID, Preset{Name: "Stable", Command: "new cmd"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if v2.ID != v.ID {
		t.Errorf("ID should stay stable, %q -> %q", v.ID, v2.ID)
	}
	loaded, err := LoadByID(dir, v2.ID)
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded.Command != "new cmd" {
		t.Errorf("command not persisted: %q", loaded.Command)
	}
}

func TestUpdateRemapsFavourites(t *testing.T) {
	dir := t.TempDir()
	v := mustSave(t, dir, Preset{Name: "Original", Command: "cmd"})
	if err := favourites.Save(dir, []string{v.ID}); err != nil {
		t.Fatalf("favourites.Save: %v", err)
	}
	v2, err := Update(dir, v.ID, Preset{Name: "Renamed", Command: "cmd"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	favs, err := favourites.Load(dir)
	if err != nil {
		t.Fatalf("favourites.Load: %v", err)
	}
	if len(favs) != 1 || favs[0] != v2.ID {
		t.Errorf("favourite not remapped: got %v, want [%s]", favs, v2.ID)
	}
}

// --- Duplicate-of-a-preset scenario (the reported GUI bug) ---

func TestDuplicateSameNameCoexist(t *testing.T) {
	dir := t.TempDir()
	original := mustSave(t, dir, Preset{Name: "Preset X", Command: "cmd"})
	// The GUI used to append "(Copy)" and collide with an existing copy.
	dup1 := mustSave(t, dir, Preset{Name: "Preset X (Copy)", Command: "cmd"})
	dup2 := mustSave(t, dir, Preset{Name: "Preset X (Copy)", Command: "cmd"})

	ids := make(map[string]bool)
	for _, id := range []string{original.ID, dup1.ID, dup2.ID} {
		if ids[id] {
			t.Fatalf("duplicate ID %q", id)
		}
		ids[id] = true
	}
	views, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(views) != 3 {
		t.Errorf("expected 3 coexisting presets, got %d", len(views))
	}
}

// --- Migrate: legacy <slug>.json files -> <slug>-<uid>.json ---

func TestMigrateLegacyFiles(t *testing.T) {
	dir := t.TempDir()
	// Legacy file: no uid field in the JSON body.
	writeJSON(t, filepath.Join(dir, "gemma-4-12b.json"), `{
		"name": "Gemma 4 12B",
		"model": "some/model",
		"command": "cmd"
	}`)
	if err := favourites.Save(dir, []string{"gemma-4-12b"}); err != nil {
		t.Fatal(err)
	}
	if err := decks.Save(dir, []decks.Deck{{Name: "MyDeck", PresetIDs: []string{"gemma-4-12b"}}}); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(dir); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if fileExists(filepath.Join(dir, "gemma-4-12b.json")) {
		t.Errorf("legacy file should be removed after migration")
	}

	views, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 preset after migration, got %d", len(views))
	}
	id := views[0].ID
	if !strings.HasPrefix(id, "gemma-4-12b-") {
		t.Errorf("expected migrated ID prefix gemma-4-12b-, got %q", id)
	}
	if !uidPattern.MatchString(strings.TrimPrefix(id, "gemma-4-12b-")) {
		t.Errorf("expected a 7-hex uid suffix, got %q", id)
	}

	favs, _ := favourites.Load(dir)
	if len(favs) != 1 || favs[0] != id {
		t.Errorf("favourite reference not remapped: %v", favs)
	}
	dl, _ := decks.Load(dir)
	if len(dl) != 1 || len(dl[0].PresetIDs) != 1 || dl[0].PresetIDs[0] != id {
		t.Errorf("deck reference not remapped: %+v", dl)
	}
}

// --- Migrate: seeds an example preset into an empty directory ---

func TestMigrateSeedsExample(t *testing.T) {
	dir := t.TempDir()
	if err := Migrate(dir); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	views, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected a seeded example preset, got %d", len(views))
	}
	if views[0].UID == "" {
		t.Errorf("seeded preset should carry a uid, got empty")
	}
}

// --- Migrate: idempotent, stable IDs across repeated runs ---

func TestMigrateIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "example.json"), `{
		"name": "Example",
		"command": "cmd"
	}`)
	if err := Migrate(dir); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	first, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 preset, got %d", len(first))
	}
	firstID := first[0].ID

	if err := Migrate(dir); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	second, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 {
		t.Fatalf("expected still 1 preset after double migrate, got %d", len(second))
	}
	if second[0].ID != firstID {
		t.Errorf("ID changed across re-migration: %q -> %q", firstID, second[0].ID)
	}
}

// --- Validate / ValidateID / Delete / LoadByID ---

func TestValidateRequiresNameAndCommand(t *testing.T) {
	if err := Validate(Preset{Command: "cmd"}); err == nil {
		t.Errorf("Validate should reject empty name")
	}
	if err := Validate(Preset{Name: "x"}); err == nil {
		t.Errorf("Validate should reject empty command")
	}
	if err := Validate(Preset{Name: "x", Command: "cmd"}); err != nil {
		t.Errorf("Validate should accept valid preset: %v", err)
	}
}

func TestValidateID(t *testing.T) {
	valid := []string{"abc-123", "preset-x", "a.b_c", "ABC-999"}
	for _, id := range valid {
		if err := ValidateID(id); err != nil {
			t.Errorf("ValidateID(%q) unexpected error: %v", id, err)
		}
	}
	invalid := []string{"", "   ", "bad/slash", "bad space", "bad*star", "bad:colon"}
	for _, id := range invalid {
		if err := ValidateID(id); err == nil {
			t.Errorf("ValidateID(%q) should have failed", id)
		}
	}
}

func TestDeleteRemovesFile(t *testing.T) {
	dir := t.TempDir()
	v := mustSave(t, dir, Preset{Name: "Doomed", Command: "cmd"})
	if err := Delete(dir, v.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if fileExists(filepath.Join(dir, v.ID+".json")) {
		t.Errorf("file should be removed after Delete")
	}
}

// --- Corrupt files are quarantined, not fatal ---

func countQuarantined(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	n := 0
	for _, e := range entries {
		if strings.Contains(e.Name(), ".corrupt-") {
			n++
		}
	}
	return n
}

func TestLoadCorruptPresetIsQuarantined(t *testing.T) {
	dir := t.TempDir()
	mustSave(t, dir, Preset{Name: "Good", Command: "cmd"})
	writeJSON(t, filepath.Join(dir, "broken-abcdef1.json"), "{not json")

	views, err := Load(dir)
	if err != nil {
		t.Fatalf("Load should not fail on a corrupt file, got: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected the 1 good preset, got %d", len(views))
	}
	if views[0].Name != "Good" {
		t.Errorf("unexpected preset %q", views[0].Name)
	}
	if n := countQuarantined(t, dir); n != 1 {
		t.Errorf("expected 1 quarantined file, got %d", n)
	}
}

func TestMigrateCorruptFileIsQuarantinedAndSeeds(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "broken-abcdef1.json"), "{not json")

	if err := Migrate(dir); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// The corrupt file must not count as a preset, so the directory is still
	// seeded with the example preset.
	views, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected the seeded example preset, got %d", len(views))
	}
	if n := countQuarantined(t, dir); n != 1 {
		t.Errorf("expected 1 quarantined file, got %d", n)
	}
}
