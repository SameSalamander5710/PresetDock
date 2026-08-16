// Package storeutil provides small helpers shared by the file-backed JSON
// stores: atomic writes and quarantine of corrupt files.
package storeutil

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WriteFileAtomic writes data to path by writing a temp file in the same
// directory and renaming it into place, so a crash or power loss mid-write can
// never leave a truncated file at path.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := f.Name()
	defer os.Remove(tmpPath) // no-op once the rename has succeeded

	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		// Windows refuses to replace an existing file with Rename; remove
		// the destination and retry.
		if rmErr := os.Remove(path); rmErr != nil {
			return fmt.Errorf("rename temp file: %w", err)
		}
		if retryErr := os.Rename(tmpPath, path); retryErr != nil {
			return fmt.Errorf("rename temp file: %w", retryErr)
		}
	}
	return nil
}

// Quarantine renames path to a timestamped <path>.corrupt-<ts> sibling so a
// corrupt store file gets out of the way without being destroyed. It returns
// the new path, or an empty string when the file no longer exists.
func Quarantine(path string) string {
	stamp := time.Now().UTC().Format("20060102T150405.000000000")
	quarantined := fmt.Sprintf("%s.corrupt-%s", path, stamp)
	if err := os.Rename(path, quarantined); err != nil {
		return ""
	}
	return quarantined
}
