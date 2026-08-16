package favourites

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"

	"presetdock/backend/internal/storeutil"
)

// Path returns the resolved path for favourites.json.
func Path(presetsDir string) string {
	return filepath.Join(presetsDir, "favourites.json")
}

// Load reads the favourite preset IDs from disk.
// Returns an empty slice (not nil) when the file does not exist or is corrupt;
// a corrupt file is quarantined to favourites.json.corrupt-<timestamp>.
func Load(presetsDir string) ([]string, error) {
	data, err := os.ReadFile(Path(presetsDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	var favs []string
	if err := json.Unmarshal(data, &favs); err != nil {
		// A corrupt favourites.json must not take the whole UI down: move the
		// file aside (preserving it for recovery) and start fresh.
		if q := storeutil.Quarantine(Path(presetsDir)); q != "" {
			log.Printf("favourites: quarantined corrupt favourites.json to %s", q)
		}
		return []string{}, nil
	}
	return favs, nil
}

// LoadSet reads the favourite preset IDs and returns them as a membership set.
func LoadSet(presetsDir string) (map[string]bool, error) {
	favs, err := Load(presetsDir)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(favs))
	for _, id := range favs {
		set[id] = true
	}
	return set, nil
}

// Save writes the favourite preset IDs to disk atomically.
func Save(presetsDir string, favs []string) error {
	data, err := json.MarshalIndent(favs, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return storeutil.WriteFileAtomic(Path(presetsDir), data, 0o644)
}

// RemoveID strips presetID from the favourites list and persists the change.
// It is used when a preset is deleted so favourites.json never keeps dead
// references. The file is rewritten only when membership actually changes.
func RemoveID(presetsDir, presetID string) error {
	if presetID == "" {
		return nil
	}
	favs, err := Load(presetsDir)
	if err != nil {
		return err
	}
	changed := false
	kept := make([]string, 0, len(favs))
	for _, id := range favs {
		if id == presetID {
			changed = true
			continue
		}
		kept = append(kept, id)
	}
	if !changed {
		return nil
	}
	return Save(presetsDir, kept)
}
