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

	mux := http.NewServeMux()
	mux.Handle("/api/heartbeat", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		heartbeat.Touch()
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
	mux.Handle("/", http.FileServer(http.FS(frontendFS)))

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

	presets := make([]PresetView, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}

		preset, err := readPreset(filepath.Join(presetsDir, entry.Name()))
		if err != nil {
			log.Printf("skipping preset %s: %v", entry.Name(), err)
			continue
		}

		presets = append(presets, PresetView{
			ID:     strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			Preset: preset,
		})
	}

	sort.Slice(presets, func(i, j int) bool {
		return presets[i].ID < presets[j].ID
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
