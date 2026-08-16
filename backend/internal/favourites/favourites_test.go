package favourites

import (
	"os"
	"testing"
)

func TestRemoveIDStripsFromList(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, []string{"p1", "p2", "p3"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := RemoveID(dir, "p2"); err != nil {
		t.Fatalf("RemoveID: %v", err)
	}
	favs, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(favs) != 2 || favs[0] != "p1" || favs[1] != "p3" {
		t.Errorf("RemoveID(p2) = %v, want [p1 p3]", favs)
	}
}

func TestRemoveIDMissingDoesNotRewrite(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, []string{"p1"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	before, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	if err := RemoveID(dir, "missing"); err != nil {
		t.Fatalf("RemoveID: %v", err)
	}
	after, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("file should not be rewritten when the ID is not present")
	}
}

func TestRemoveIDOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	if err := RemoveID(dir, "p1"); err != nil {
		t.Fatalf("RemoveID on missing file should be a no-op, got: %v", err)
	}
}
