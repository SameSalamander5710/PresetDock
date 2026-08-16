package storeutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	want := []byte(`{"hello":"world"}` + "\n")

	if err := WriteFileAtomic(path, want, 0o644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content = %q, want %q", got, want)
	}

	// No temp files may be left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "data.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("dir entries = %v, want only [data.json]", names)
	}
}

func TestWriteFileAtomicOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	if err := WriteFileAtomic(path, []byte("v1"), 0o644); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("v2"), 0o644); err != nil {
		t.Fatalf("second write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "v2" {
		t.Errorf("content = %q, want %q", got, "v2")
	}
}

func TestQuarantineMovesFileAside(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "decks.json")
	if err := os.WriteFile(path, []byte("corrupt"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	quarantined := Quarantine(path)
	if quarantined == "" {
		t.Fatal("Quarantine returned empty path for an existing file")
	}
	if !strings.Contains(quarantined, ".corrupt-") {
		t.Errorf("quarantined path %q should contain .corrupt-", quarantined)
	}
	if fileExists(path) {
		t.Errorf("original file should be gone after quarantine")
	}
	data, err := os.ReadFile(quarantined)
	if err != nil {
		t.Fatalf("read quarantined file: %v", err)
	}
	if string(data) != "corrupt" {
		t.Errorf("quarantined content = %q, want the preserved original", data)
	}
}

func TestQuarantineMissingFile(t *testing.T) {
	dir := t.TempDir()
	if q := Quarantine(filepath.Join(dir, "nope.json")); q != "" {
		t.Errorf("Quarantine of missing file = %q, want empty", q)
	}
}
