package favourites

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Path returns the resolved path for favourites.json.
func Path(presetsDir string) string {
	return filepath.Join(presetsDir, "favourites.json")
}

// Load reads the favourite preset IDs from disk.
// Returns an empty slice (not nil) when the file does not exist.
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
		return nil, err
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

// Save writes the favourite preset IDs to disk.
func Save(presetsDir string, favs []string) error {
	data, err := json.MarshalIndent(favs, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(Path(presetsDir), data, 0o644)
}
