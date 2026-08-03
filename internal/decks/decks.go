package decks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Deck represents a named collection of preset IDs.
type Deck struct {
	Name      string   `json:"name"`
	PresetIDs []string `json:"preset_ids"`
}

// Path returns the resolved path for decks.json.
func Path(presetsDir string) string {
	return filepath.Join(presetsDir, "decks.json")
}

// Load reads the deck list from disk, sorted alphabetically by name (case-insensitive).
// Returns an empty slice (not nil) when the file does not exist.
func Load(presetsDir string) ([]Deck, error) {
	data, err := os.ReadFile(Path(presetsDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Deck{}, nil
		}
		return nil, err
	}
	var decks []Deck
	if err := json.Unmarshal(data, &decks); err != nil {
		return nil, err
	}
	sort.Slice(decks, func(i, j int) bool {
		return strings.ToLower(decks[i].Name) < strings.ToLower(decks[j].Name)
	})
	return decks, nil
}

// Save writes the deck list to disk.
func Save(presetsDir string, decks []Deck) error {
	data, err := json.MarshalIndent(decks, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(Path(presetsDir), data, 0o644)
}
