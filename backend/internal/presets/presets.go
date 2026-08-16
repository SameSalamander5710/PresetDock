package presets

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"presetdock/backend/internal/decks"
	"presetdock/backend/internal/favourites"
	"presetdock/backend/internal/storeutil"
)

// Preset represents a single saved model launch configuration.
//
// UID is a short stable identifier (7 hex chars) embedded in the file name so
// same-named presets can coexist and renaming a preset can never collide with
// another file. It is persisted on disk but not part of the request DTO.
type Preset struct {
	Name        string   `json:"name"`
	Engine      string   `json:"engine,omitempty"`
	Model       string   `json:"model"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
	Shell       string   `json:"shell,omitempty"` // "cmd" (default) or "powershell"
	UID         string   `json:"uid,omitempty"`
}

// CreatePresetRequest is the API DTO for creating a new preset.
// DeckNames is a pointer so an absent field (nil) means "leave deck
// membership untouched" — which is what duplicates rely on to inherit the
// source preset's decks — while an explicit list (including an empty one)
// sets the membership exactly.
type CreatePresetRequest struct {
	Name           string    `json:"name"`
	Engine         string    `json:"engine,omitempty"`
	Model          string    `json:"model"`
	Tags           []string  `json:"tags"`
	Description    string    `json:"description"`
	Command        string    `json:"command"`
	Shell          string    `json:"shell"`
	SourcePresetID string    `json:"source_preset_id,omitempty"`
	DeckNames      *[]string `json:"deck_names"`
}

// UpdatePresetRequest is the API DTO for updating an existing preset.
// DeckNames is a pointer so an absent field (nil) means "leave deck
// membership untouched" while an explicit empty slice means "remove from all
// decks".
type UpdatePresetRequest struct {
	Preset
	DeckNames *[]string `json:"deck_names"`
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

// Save writes a new preset to a file named <slug>-<uid>.json, where the slug is
// derived from the preset's current name. A new uid is generated when
// preset.UID is empty, so creating same-named presets never collides.
func Save(presetsDir string, preset Preset) (PresetView, error) {
	if err := Validate(preset); err != nil {
		return PresetView{}, err
	}
	slug := Slugify(preset.Name)
	if slug == "" {
		slug = "preset"
	}
	if preset.UID == "" {
		preset.UID = NewUID()
	}
	id := slug + "-" + preset.UID
	if err := ValidateID(id); err != nil {
		return PresetView{}, err
	}
	if strings.TrimSpace(preset.Shell) == "" {
		preset.Shell = "cmd"
	}
	if err := writePreset(filepath.Join(presetsDir, id+".json"), preset); err != nil {
		return PresetView{}, err
	}
	return PresetView{ID: id, Preset: preset}, nil
}

// Update re-saves an existing preset under a file name derived from its
// current name, keeping the preset's uid stable. When the file name changes,
// the old file is removed and favourites.json / decks.json references are
// rewritten to the new ID. This is what makes renaming a preset in the UI
// rename its storage file.
func Update(presetsDir string, oldID string, preset Preset) (PresetView, error) {
	if err := ValidateID(oldID); err != nil {
		return PresetView{}, err
	}
	if err := Validate(preset); err != nil {
		return PresetView{}, err
	}
	oldPath := filepath.Join(presetsDir, oldID+".json")
	if _, err := os.Stat(oldPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PresetView{}, fmt.Errorf("preset %s not found: %w", oldID, os.ErrNotExist)
		}
		return PresetView{}, err
	}

	slug := Slugify(preset.Name)
	if slug == "" {
		slug = "preset"
	}
	if preset.UID == "" {
		preset.UID = uidFromID(oldID)
	}
	id := slug + "-" + preset.UID
	if err := ValidateID(id); err != nil {
		return PresetView{}, err
	}
	if strings.TrimSpace(preset.Shell) == "" {
		preset.Shell = "cmd"
	}
	if err := writePreset(filepath.Join(presetsDir, id+".json"), preset); err != nil {
		return PresetView{}, err
	}
	if id != oldID {
		if err := os.Remove(oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return PresetView{}, err
		}
		if err := remapIDs(presetsDir, map[string]string{oldID: id}); err != nil {
			return PresetView{}, err
		}
	}
	return PresetView{ID: id, Preset: preset}, nil
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

		path := filepath.Join(presetsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("skipping preset %s: %v", entry.Name(), err)
			continue
		}
		var preset Preset
		if err := json.Unmarshal(data, &preset); err != nil {
			// A corrupt preset file is quarantined (not deleted) so the user
			// can recover it, and the rest of the store stays usable.
			if q := storeutil.Quarantine(path); q != "" {
				log.Printf("presets: quarantined corrupt %s to %s", entry.Name(), q)
			}
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

// --- Migration ---

// Migrate upgrades legacy preset files (named <slug>.json) to the current
// <slug>-<uid>.json layout, rewrites favourites.json / decks.json references
// to the new IDs, and seeds a default example preset when the directory holds
// none. It is idempotent and safe to call on every start.
func Migrate(presetsDir string) error {
	if err := os.MkdirAll(presetsDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(presetsDir)
	if err != nil {
		return fmt.Errorf("read presets dir: %w", err)
	}

	remap := map[string]string{}
	presetCount := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		name := entry.Name()
		base := strings.TrimSuffix(strings.ToLower(name), ".json")
		if base == "favourites" || base == "decks" {
			continue
		}
		path := filepath.Join(presetsDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("presets: skipping %s: %v", name, err)
			continue
		}
		var preset Preset
		if err := json.Unmarshal(data, &preset); err != nil {
			if q := storeutil.Quarantine(path); q != "" {
				log.Printf("presets: quarantined corrupt %s to %s", name, q)
			}
			continue
		}
		// Only count files that actually parsed, so a directory whose preset
		// files are all corrupt still counts as empty and gets seeded.
		presetCount++
		// Legacy files carry no uid (the old code never wrote one), so an empty
		// uid is a reliable migration trigger regardless of the file name.
		if preset.UID != "" {
			continue
		}
		preset.UID = NewUID()
		slug := Slugify(preset.Name)
		if slug == "" {
			slug = "preset"
		}
		oldID := strings.TrimSuffix(name, filepath.Ext(name))
		newID := slug + "-" + preset.UID
		if err := writePreset(filepath.Join(presetsDir, newID+".json"), preset); err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		remap[oldID] = newID
	}

	if len(remap) > 0 {
		if err := remapIDs(presetsDir, remap); err != nil {
			return err
		}
	}

	if presetCount == 0 {
		if _, err := Save(presetsDir, examplePreset()); err != nil {
			return err
		}
	}

	return nil
}

// uidFromID extracts the uid from a preset ID. Migrated IDs carry the uid as
// their last dash-separated segment (7 chars); legacy IDs become their own uid.
func uidFromID(id string) string {
	i := strings.LastIndex(id, "-")
	if i >= 0 && len(id[i+1:]) == 7 {
		return id[i+1:]
	}
	return id
}

// remapIDs rewrites old preset IDs to new ones in favourites.json and
// decks.json so references stay valid after a rename or migration.
func remapIDs(presetsDir string, remap map[string]string) error {
	favs, err := favourites.Load(presetsDir)
	if err != nil {
		return err
	}
	changed := false
	for i, id := range favs {
		if n, ok := remap[id]; ok {
			favs[i] = n
			changed = true
		}
	}
	if changed {
		if err := favourites.Save(presetsDir, favs); err != nil {
			return err
		}
	}

	decksList, err := decks.Load(presetsDir)
	if err != nil {
		return err
	}
	changed = false
	for di := range decksList {
		for pi, id := range decksList[di].PresetIDs {
			if n, ok := remap[id]; ok {
				decksList[di].PresetIDs[pi] = n
				changed = true
			}
		}
	}
	if changed {
		if err := decks.Save(presetsDir, decksList); err != nil {
			return err
		}
	}

	return nil
}

func writePreset(path string, preset Preset) error {
	data, err := json.MarshalIndent(preset, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return storeutil.WriteFileAtomic(path, data, 0o644)
}

func examplePreset() Preset {
	return Preset{
		Name:        "Gemma 4 E4B - Q4",
		Engine:      "llama-server",
		Model:       "unsloth/gemma-4-E4B-it-qat-GGUF:UD-Q4_K_XL",
		Tags:        []string{"gemma", "E4B", "QAT", "Q4"},
		Description: "Example preset",
		Command:     "llama-server -hf unsloth/gemma-4-E4B-it-qat-GGUF:UD-Q4_K_XL --spec-type draft-mtp --spec-draft-n-max 2 -fit on -ngl 999 --flash-attn on -c 8192 --temp 1.0 --top-p 0.95 --top-k 64 --mlock --host 127.0.0.1 --port 8080",
	}
}

// --- Validation ---

// NewUID returns a short unique identifier (7 lowercase hex chars) used as the
// per-preset uid in file names.
func NewUID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is effectively impossible; fall back to a
		// fixed value rather than crashing.
		return "0000000"
	}
	return hex.EncodeToString(b)[:7]
}

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
