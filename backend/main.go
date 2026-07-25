package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	heartbeatInterval = 30 * time.Second
	heartbeatTimeout  = 2 * time.Minute
	shutdownTimeout   = 5 * time.Second
)

//go:embed frontend/*
var embeddedFrontend embed.FS

type Preset struct {
	Name        string   `json:"name"`
	Engine      string   `json:"engine,omitempty"`
	Model       string   `json:"model"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
}

type PresetView struct {
	ID string `json:"id"`
	Preset
	IsFavourite bool `json:"is_favourite"`
}

type Deck struct {
	Name      string   `json:"name"`
	PresetIDs []string `json:"preset_ids"`
}

type heartbeatState struct {
	mu       sync.Mutex
	lastBeat time.Time
}

func newHeartbeatState() *heartbeatState {
	return &heartbeatState{lastBeat: time.Now()}
}

func (state *heartbeatState) Touch() {
	state.mu.Lock()
	state.lastBeat = time.Now()
	state.mu.Unlock()
}

func (state *heartbeatState) Stale(timeout time.Duration) bool {
	state.mu.Lock()
	lastBeat := state.lastBeat
	state.mu.Unlock()
	return time.Since(lastBeat) > timeout
}

func main() {
	exeDir, err := executableDir()
	if err != nil {
		log.Fatal(err)
	}

	presetsDir := resolvePresetsDir(exeDir)
	if err := ensurePresetsDir(presetsDir); err != nil {
		log.Fatal(err)
	}

	frontendFS, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	heartbeat := newHeartbeatState()
	var requestShutdown func()

	mux := http.NewServeMux()
	mux.Handle("/api/heartbeat", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		heartbeat.Touch()
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.Handle("/api/shutdown", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		if requestShutdown != nil {
			go requestShutdown()
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	mux.Handle("/api/presets", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			presets, err := loadPresets(presetsDir)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}

			writeJSON(w, http.StatusOK, presets)
		case http.MethodPost:
			var preset Preset
			if err := json.NewDecoder(r.Body).Decode(&preset); err != nil {
				httpError(w, http.StatusBadRequest, "invalid preset JSON")
				return
			}

			savedPreset, err := savePreset(presetsDir, "", preset, false)
			if err != nil {
				httpError(w, http.StatusBadRequest, err.Error())
				return
			}

			writeJSON(w, http.StatusCreated, savedPreset)
		default:
			methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
		}
	}))
	mux.Handle("/api/presets/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/presets/")
		if id == "" || strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}

		switch r.Method {
		case http.MethodPut:
			var preset Preset
			if err := json.NewDecoder(r.Body).Decode(&preset); err != nil {
				httpError(w, http.StatusBadRequest, "invalid preset JSON")
				return
			}

			savedPreset, err := savePreset(presetsDir, id, preset, true)
			if err != nil {
				httpError(w, http.StatusBadRequest, err.Error())
				return
			}

			writeJSON(w, http.StatusOK, savedPreset)
		case http.MethodDelete:
			if err := deletePreset(presetsDir, id); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					http.NotFound(w, r)
					return
				}
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}

			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w, http.MethodPut+", "+http.MethodDelete)
		}
	}))
	// Favourites API
	mux.Handle("/api/favourites", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			favs, err := loadFavourites(presetsDir)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, favs)
		case http.MethodPost:
			var payload struct {
				PresetID string `json:"preset_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				httpError(w, http.StatusBadRequest, "invalid JSON")
				return
			}
			if payload.PresetID == "" {
				httpError(w, http.StatusBadRequest, "preset_id is required")
				return
			}
			favs, err := loadFavourites(presetsDir)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			for _, id := range favs {
				if id == payload.PresetID {
					writeJSON(w, http.StatusOK, favs)
					return
				}
			}
			favs = append(favs, payload.PresetID)
			if err := saveFavourites(presetsDir, favs); err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, favs)
		default:
			methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
		}
	}))
	mux.Handle("/api/favourites/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/favourites/")
		if id == "" || strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}

		if r.Method != http.MethodDelete {
			methodNotAllowed(w, http.MethodDelete)
			return
		}

		favs, err := loadFavourites(presetsDir)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}

		newFavs := make([]string, 0, len(favs))
		found := false
		for _, fid := range favs {
			if fid == id {
				found = true
				continue
			}
			newFavs = append(newFavs, fid)
		}
		if !found {
			http.NotFound(w, r)
			return
		}

		if err := saveFavourites(presetsDir, newFavs); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, newFavs)
	}))

	// Decks API
	mux.Handle("/api/decks", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			decks, err := loadDecks(presetsDir)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, decks)
		case http.MethodPost:
			var deck Deck
			if err := json.NewDecoder(r.Body).Decode(&deck); err != nil {
				httpError(w, http.StatusBadRequest, "invalid JSON")
				return
			}
			if strings.TrimSpace(deck.Name) == "" {
				httpError(w, http.StatusBadRequest, "deck name is required")
				return
			}
			decks, err := loadDecks(presetsDir)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			for _, d := range decks {
				if strings.EqualFold(d.Name, deck.Name) {
					httpError(w, http.StatusBadRequest, "deck with that name already exists")
					return
				}
			}
			if deck.PresetIDs == nil {
				deck.PresetIDs = []string{}
			}
			decks = append(decks, deck)
			if err := saveDecks(presetsDir, decks); err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, deck)
		default:
			methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
		}
	}))
	mux.Handle("/api/decks/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/decks/")
		if name == "" || strings.Contains(name, "/") {
			http.NotFound(w, r)
			return
		}

		decks, err := loadDecks(presetsDir)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}

		deckIdx := -1
		for i, d := range decks {
			if strings.EqualFold(d.Name, name) {
				deckIdx = i
				break
			}
		}
		if deckIdx == -1 {
			http.NotFound(w, r)
			return
		}

		switch r.Method {
		case http.MethodPut:
			var updated Deck
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				httpError(w, http.StatusBadRequest, "invalid JSON")
				return
			}
			if strings.TrimSpace(updated.Name) == "" {
				httpError(w, http.StatusBadRequest, "deck name is required")
				return
			}
			for i, d := range decks {
				if i != deckIdx && strings.EqualFold(d.Name, updated.Name) {
					httpError(w, http.StatusBadRequest, "deck with that name already exists")
					return
				}
			}
			if updated.PresetIDs == nil {
				updated.PresetIDs = []string{}
			}
			decks[deckIdx] = updated
			if err := saveDecks(presetsDir, decks); err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, updated)
		case http.MethodDelete:
			decks = append(decks[:deckIdx], decks[deckIdx+1:]...)
			if err := saveDecks(presetsDir, decks); err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w, http.MethodPut+", "+http.MethodDelete)
		}
	}))

	mux.Handle("/api/run/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/run/")
		if id == "" || strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}

		preset, err := loadPresetByID(presetsDir, id)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.NotFound(w, r)
				return
			}
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}

		command := exec.Command("cmd", "/c", "start", "", "cmd", "/k", preset.Command)
		if err := command.Start(); err != nil {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to launch command: %v", err))
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
	}))
	mux.Handle("/", noCache(http.FileServer(http.FS(frontendFS))))

	listener, err := net.Listen("tcp", "127.0.0.1:8765")
	if err != nil {
		if launchIfAlreadyRunning() {
			return
		}
		log.Fatal(err)
	}

	if err := openBrowser("http://127.0.0.1:8765"); err != nil {
		log.Printf("browser launch failed: %v", err)
	}

	server := &http.Server{Handler: mux}
	requestShutdown = func() {
		time.Sleep(150 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("shutdown error: %v", err)
		}
	}
	go monitorHeartbeat(server, heartbeat)

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func monitorHeartbeat(server *http.Server, state *heartbeatState) {
	ticker := time.NewTicker(heartbeatInterval / 2)
	defer ticker.Stop()

	for range ticker.C {
		if !state.Stale(heartbeatTimeout) {
			continue
		}

		log.Println("No heartbeat received for 2 minutes; shutting down.")
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		_ = server.Shutdown(ctx)
		cancel()
		return
	}
}

func launchIfAlreadyRunning() bool {
	client := &http.Client{Timeout: 750 * time.Millisecond}
	response, err := client.Get("http://127.0.0.1:8765/api/presets")
	if err != nil {
		return false
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return false
	}

	if err := openBrowser("http://127.0.0.1:8765"); err != nil {
		log.Printf("browser launch failed: %v", err)
	}
	log.Println("PresetDock is already running; reopened the browser.")
	return true
}

func executableDir() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	resolvedPath, err := filepath.EvalSymlinks(executablePath)
	if err == nil {
		executablePath = resolvedPath
	}

	return filepath.Dir(executablePath), nil
}

func resolvePresetsDir(exeDir string) string {
	if strings.Contains(strings.ToLower(exeDir), "go-build") {
		if workingDir, err := os.Getwd(); err == nil {
			workingPresetsDir := filepath.Join(workingDir, "presets")
			if pathExists(workingPresetsDir) {
				return workingPresetsDir
			}
		}
	}

	return filepath.Join(exeDir, "presets")
}

func pathExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func ensurePresetsDir(presetsDir string) error {
	if err := os.MkdirAll(presetsDir, 0o755); err != nil {
		return err
	}

	examplePath := filepath.Join(presetsDir, "example.json")
	if _, err := os.Stat(examplePath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	example := Preset{
		Name:        "Gemma 2 9B Q4",
		Engine:      "llama-cli",
		Model:       "gemma-2-9b-it-Q4_K_M.gguf",
		Tags:        []string{"gemma", "9b", "q4"},
		Description: "General purpose, balanced speed/quality",
		Command:     "llama-cli.exe -m models/gemma-2-9b-it-Q4_K_M.gguf -p \"You are a helpful assistant\" -n 512",
	}

	data, err := json.MarshalIndent(example, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')
	return os.WriteFile(examplePath, data, 0o644)
}

func savePreset(presetsDir string, targetID string, preset Preset, overwrite bool) (PresetView, error) {
	if targetID == "" {
		targetID = slugifyPresetID(preset.Name)
	}
	if err := validatePresetID(targetID); err != nil {
		return PresetView{}, err
	}
	if err := validatePreset(preset); err != nil {
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

func loadPresets(presetsDir string) ([]PresetView, error) {
	entries, err := os.ReadDir(presetsDir)
	if err != nil {
		return nil, err
	}

	// Load favourites set
	favSet, _ := loadFavouritesSet(presetsDir)

	presets := make([]PresetView, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}

		// Skip favourites.json and decks.json
		baseName := strings.TrimSuffix(strings.ToLower(entry.Name()), ".json")
		if baseName == "favourites" || baseName == "decks" {
			continue
		}

		preset, err := readPreset(filepath.Join(presetsDir, entry.Name()))
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

	// Sort: favourites first, then alphabetically by name
	sort.Slice(presets, func(i, j int) bool {
		if presets[i].IsFavourite != presets[j].IsFavourite {
			return presets[i].IsFavourite
		}
		return strings.ToLower(presets[i].Name) < strings.ToLower(presets[j].Name)
	})

	return presets, nil
}

func loadPresetByID(presetsDir, id string) (Preset, error) {
	presetPath := filepath.Join(presetsDir, id+".json")
	return readPreset(presetPath)
}

func deletePreset(presetsDir, id string) error {
	if err := validatePresetID(id); err != nil {
		return err
	}
	return os.Remove(filepath.Join(presetsDir, id+".json"))
}

func validatePreset(preset Preset) error {
	if strings.TrimSpace(preset.Name) == "" {
		return errors.New("preset name is required")
	}
	if strings.TrimSpace(preset.Command) == "" {
		return errors.New("preset command is required")
	}
	return nil
}

func slugifyPresetID(value string) string {
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

func validatePresetID(id string) error {
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

func readPreset(path string) (Preset, error) {
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

// --- Favourites ---

func favouritesPath(presetsDir string) string {
	return filepath.Join(presetsDir, "favourites.json")
}

func loadFavourites(presetsDir string) ([]string, error) {
	path := favouritesPath(presetsDir)
	data, err := os.ReadFile(path)
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

func loadFavouritesSet(presetsDir string) (map[string]bool, error) {
	favs, err := loadFavourites(presetsDir)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(favs))
	for _, id := range favs {
		set[id] = true
	}
	return set, nil
}

func saveFavourites(presetsDir string, favs []string) error {
	path := favouritesPath(presetsDir)
	data, err := json.MarshalIndent(favs, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// --- Decks ---

func decksPath(presetsDir string) string {
	return filepath.Join(presetsDir, "decks.json")
}

func loadDecks(presetsDir string) ([]Deck, error) {
	path := decksPath(presetsDir)
	data, err := os.ReadFile(path)
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
	return decks, nil
}

func saveDecks(presetsDir string, decks []Deck) error {
	path := decksPath(presetsDir)
	data, err := json.MarshalIndent(decks, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func openBrowser(url string) error {
	return exec.Command("cmd", "/c", "start", "", url).Start()
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func httpError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	httpError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}
