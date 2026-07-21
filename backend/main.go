package main

import (
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
)

//go:embed frontend/*
var embeddedFrontend embed.FS

type Preset struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Model       string   `json:"model"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
}

func main() {
	exeDir, err := executableDir()
	if err != nil {
		log.Fatal(err)
	}

	presetsDir := filepath.Join(exeDir, "presets")
	if err := ensurePresetsDir(presetsDir); err != nil {
		log.Fatal(err)
	}

	frontendFS, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/presets", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		presets, err := loadPresets(presetsDir)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, presets)
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
		log.Fatal(err)
	}

	if err := openBrowser("http://127.0.0.1:8765"); err != nil {
		log.Printf("browser launch failed: %v", err)
	}

	log.Fatal(http.Serve(listener, mux))
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

func loadPresets(presetsDir string) ([]Preset, error) {
	entries, err := os.ReadDir(presetsDir)
	if err != nil {
		return nil, err
	}

	presets := make([]Preset, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}

		preset, err := readPreset(filepath.Join(presetsDir, entry.Name()))
		if err != nil {
			log.Printf("skipping preset %s: %v", entry.Name(), err)
			continue
		}

		preset.ID = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		presets = append(presets, preset)
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
