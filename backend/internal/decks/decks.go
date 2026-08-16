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

// DecksForPreset returns the names of all decks in list that contain presetID.
// The result preserves the order of list.
func DecksForPreset(list []Deck, presetID string) []string {
	names := make([]string, 0)
	for _, d := range list {
		for _, id := range d.PresetIDs {
			if id == presetID {
				names = append(names, d.Name)
				break
			}
		}
	}
	return names
}

// SetPresetDecks is the single central writer for preset-deck membership. It
// ensures presetID is present exactly in the named decks (matched
// case-insensitively by name) and removes it from every other deck. Deck names
// that do not exist are ignored. The file is rewritten only when membership
// actually changes.
func SetPresetDecks(presetsDir, presetID string, deckNames []string) error {
	if presetID == "" {
		return nil
	}
	list, err := Load(presetsDir)
	if err != nil {
		return err
	}
	target := make(map[string]bool, len(deckNames))
	for _, name := range deckNames {
		target[strings.ToLower(name)] = true
	}
	changed := false
	for i, d := range list {
		inTarget := target[strings.ToLower(d.Name)]
		hasPreset := false
		for _, id := range d.PresetIDs {
			if id == presetID {
				hasPreset = true
				break
			}
		}
		switch {
		case inTarget && !hasPreset:
			d.PresetIDs = append(d.PresetIDs, presetID)
			changed = true
		case !inTarget && hasPreset:
			ids := make([]string, 0, len(d.PresetIDs)-1)
			for _, id := range d.PresetIDs {
				if id != presetID {
					ids = append(ids, id)
				}
			}
			d.PresetIDs = ids
			changed = true
		}
		list[i] = d
	}
	if !changed {
		return nil
	}
	return Save(presetsDir, list)
}

// RemovePreset strips presetID from every deck and persists the change. It is
// used when a preset is deleted so decks.json never keeps dead references.
func RemovePreset(presetsDir, presetID string) error {
	if presetID == "" {
		return nil
	}
	list, err := Load(presetsDir)
	if err != nil {
		return err
	}
	changed := false
	for i := range list {
		ids := make([]string, 0, len(list[i].PresetIDs))
		for _, id := range list[i].PresetIDs {
			if id != presetID {
				ids = append(ids, id)
			}
		}
		if len(ids) != len(list[i].PresetIDs) {
			list[i].PresetIDs = ids
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return Save(presetsDir, list)
}
