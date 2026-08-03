package presets

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"presetdock/backend/internal/favourites"
)

// Preset represents a single saved model launch configuration.
type Preset struct {
	Name        string   `json:"name"`
	Engine      string   `json:"engine,omitempty"`
	Model       string   `json:"model"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
}

// CreatePresetRequest is the API DTO for creating a new preset.
type CreatePresetRequest struct {
	Name           string   `json:"name"`
	Engine         string   `json:"engine,omitempty"`
	Model          string   `json:"model"`
	Tags           []string `json:"tags"`
	Description    string   `json:"description"`
	Command        string   `json:"command"`
	SourcePresetID string   `json:"source_preset_id,omitempty"`
}

// PresetView combines a preset with its file-derived ID and favourite status.
type PresetView struct {
	ID string `json:"id"`
	Preset
	IsFavourite bool `json:"is_favourite"`
}

// --- Storage ---

// Read loads a single preset from the given file path.
func Read(path string) (Preset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Preset{}, err
	}
	var preset Preset
	if err := json.Unmarshal(data, &preset); err != nil {
		return Preset{}, err
	}
	return preset, nil
}

// Save writes a preset to disk. When targetID is empty it will be derived from
// the preset name via Slugify. When overwrite is false the call fails if the
// target file already exists.
func Save(presetsDir string, targetID string, preset Preset, overwrite bool) (PresetView, error) {
	if targetID == "" {
		targetID = Slugify(preset.Name)
	}
	if err := ValidateID(targetID); err != nil {
		return PresetView{}, err
	}
	if err := Validate(preset); err != nil {
		return PresetView{}, err
	}

	targetPath := filepath.Join(presetsDir, targetID+".json")
	if !overwrite {
		if _, err := os.Stat(targetPath); err == nil {
			return PresetView{}, fmt.Errorf("preset %s already exists", targetID)
		} else if !errors.Is(err, os.ErrNotExist) {
			return PresetView{}, err
		}
	}

	data, err := json.MarshalIndent(preset, "", "  ")
	if err != nil {
		return PresetView{}, err
	}
	data = append(data, '\n')

	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return PresetView{}, err
	}

	return PresetView{ID: targetID, Preset: preset}, nil
}

// Load reads all preset files from the directory and returns them enriched with
// favourite status. System files (favourites.json, decks.json) are skipped.
func Load(presetsDir string) ([]PresetView, error) {
	entries, err := os.ReadDir(presetsDir)
	if err != nil {
		return nil, err
	}

	favSet, _ := favourites.LoadSet(presetsDir)

	presets := make([]PresetView, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}

		baseName := strings.TrimSuffix(strings.ToLower(entry.Name()), ".json")
		if baseName == "favourites" || baseName == "decks" {
			continue
		}

		preset, err := Read(filepath.Join(presetsDir, entry.Name()))
		if err != nil {
			log.Printf("skipping preset %s: %v", entry.Name(), err)
			continue
		}

		presetID := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		pv := PresetView{
			ID:          presetID,
			Preset:      preset,
			IsFavourite: favSet[presetID],
		}
		presets = append(presets, pv)
	}

	sort.Slice(presets, func(i, j int) bool {
		if presets[i].IsFavourite != presets[j].IsFavourite {
			return presets[i].IsFavourite
		}
		return strings.ToLower(presets[i].Name) < strings.ToLower(presets[j].Name)
	})

	return presets, nil
}

// LoadByID loads a single preset by its ID.
func LoadByID(presetsDir, id string) (Preset, error) {
	return Read(filepath.Join(presetsDir, id+".json"))
}

// Delete removes a preset file from disk.
func Delete(presetsDir, id string) error {
	if err := ValidateID(id); err != nil {
		return err
	}
	return os.Remove(filepath.Join(presetsDir, id+".json"))
}

// --- Validation ---

// Validate checks that the required fields are populated.
func Validate(p Preset) error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("preset name is required")
	}
	if strings.TrimSpace(p.Command) == "" {
		return errors.New("preset command is required")
	}
	return nil
}

// ValidateID ensures the ID only contains allowed characters.
func ValidateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("preset id is required")
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return errors.New("preset id may only contain letters, numbers, dots, underscores, and dashes")
		}
	}
	return nil
}

// Slugify converts a human-readable name to a URL-safe preset ID.
func Slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	var builder strings.Builder
	lastWasDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastWasDash = false
		case r == '-' || r == '_' || r == '.':
			if !lastWasDash {
				builder.WriteRune('-')
				lastWasDash = true
			}
		default:
			if !lastWasDash {
				builder.WriteRune('-')
				lastWasDash = true
			}
		}
	}

	return strings.Trim(builder.String(), "-")
}

// --- Bootstrap ---

// EnsureDir creates the presets directory if it does not exist and seeds the
// default example preset when no example file is found yet.
func EnsureDir(presetsDir string) error {
	if err := os.MkdirAll(presetsDir, 0o755); err != nil {
		return err
	}

	examplePath := filepath.Join(presetsDir, "Example.json")
	if _, err := os.Stat(examplePath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	example := Preset{
		Name:        "Gemma 4 E4B - Q4",
		Engine:      "llama-server",
		Model:       "unsloth/gemma-4-E4B-it-qat-GGUF:UD-Q4_K_XL",
		Tags:        []string{"gemma", "E4B", "QAT", "Q4"},
		Description: "Example preset",
		Command:     "llama-server -hf unsloth/gemma-4-E4B-it-qat-GGUF:UD-Q4_K_XL --spec-type draft-mtp --spec-draft-n-max 2 -fit on -ngl 999 --flash-attn on -c 8192 --temp 1.0 --top-p 0.95 --top-k 64 --mlock --host 127.0.0.1 --port 8080",
	}

	data, err := json.MarshalIndent(example, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(examplePath, data, 0o644)
}
