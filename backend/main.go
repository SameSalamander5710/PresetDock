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
	"strings"
	"sync"
	"syscall"
	"time"

	"presetdock/backend/internal/decks"
	"presetdock/backend/internal/favourites"
	"presetdock/backend/internal/presets"
)

const (
	heartbeatInterval = 30 * time.Second
	heartbeatTimeout  = 2 * time.Minute
	shutdownTimeout   = 5 * time.Second
	createNoWindow    = 0x08000000
)

//go:embed frontend/*
var embeddedFrontend embed.FS

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
	if err := presets.EnsureDir(presetsDir); err != nil {
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
			presetList, err := presets.Load(presetsDir)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}

			writeJSON(w, http.StatusOK, presetList)
		case http.MethodPost:
			var req presets.CreatePresetRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, "invalid preset JSON")
				return
			}

			preset := presets.Preset{
				Name:        req.Name,
				Engine:      req.Engine,
				Model:       req.Model,
				Tags:        req.Tags,
				Description: req.Description,
				Command:     req.Command,
			}

			savedPreset, err := presets.Save(presetsDir, "", preset, false)
			if err != nil {
				httpError(w, http.StatusBadRequest, err.Error())
				return
			}

			// If this is a duplicate (source_preset_id provided), propagate deck and favourite membership
			if req.SourcePresetID != "" {
				propagatePresetMembership(presetsDir, req.SourcePresetID, savedPreset.ID)
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
			var preset presets.Preset
			if err := json.NewDecoder(r.Body).Decode(&preset); err != nil {
				httpError(w, http.StatusBadRequest, "invalid preset JSON")
				return
			}

			savedPreset, err := presets.Save(presetsDir, id, preset, true)
			if err != nil {
				httpError(w, http.StatusBadRequest, err.Error())
				return
			}

			writeJSON(w, http.StatusOK, savedPreset)
		case http.MethodDelete:
			if err := presets.Delete(presetsDir, id); err != nil {
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
			favs, err := favourites.Load(presetsDir)
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
			favs, err := favourites.Load(presetsDir)
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
			if err := favourites.Save(presetsDir, favs); err != nil {
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

		favs, err := favourites.Load(presetsDir)
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

		if err := favourites.Save(presetsDir, newFavs); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, newFavs)
	}))

	// Decks API
	mux.Handle("/api/decks", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			decksList, err := decks.Load(presetsDir)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, decksList)
		case http.MethodPost:
			var deck decks.Deck
			if err := json.NewDecoder(r.Body).Decode(&deck); err != nil {
				httpError(w, http.StatusBadRequest, "invalid JSON")
				return
			}
			if strings.TrimSpace(deck.Name) == "" {
				httpError(w, http.StatusBadRequest, "deck name is required")
				return
			}
			decksList, err := decks.Load(presetsDir)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			for _, d := range decksList {
				if strings.EqualFold(d.Name, deck.Name) {
					httpError(w, http.StatusBadRequest, "deck with that name already exists")
					return
				}
			}
			if deck.PresetIDs == nil {
				deck.PresetIDs = []string{}
			}
			decksList = append(decksList, deck)
			if err := decks.Save(presetsDir, decksList); err != nil {
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

		decksList, err := decks.Load(presetsDir)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}

		deckIdx := -1
		for i, d := range decksList {
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
			var updated decks.Deck
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				httpError(w, http.StatusBadRequest, "invalid JSON")
				return
			}
			if strings.TrimSpace(updated.Name) == "" {
				httpError(w, http.StatusBadRequest, "deck name is required")
				return
			}
			for i, d := range decksList {
				if i != deckIdx && strings.EqualFold(d.Name, updated.Name) {
					httpError(w, http.StatusBadRequest, "deck with that name already exists")
					return
				}
			}
			if updated.PresetIDs == nil {
				updated.PresetIDs = []string{}
			}
			decksList[deckIdx] = updated
			if err := decks.Save(presetsDir, decksList); err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, updated)
		case http.MethodDelete:
			decksList = append(decksList[:deckIdx], decksList[deckIdx+1:]...)
			if err := decks.Save(presetsDir, decksList); err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w, http.MethodPut+", "+http.MethodDelete)
		}
	}))

	// /api/run (POST) — run a preset directly from a JSON payload (used by the editor "Run" button)
	mux.Handle("/api/run", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		var payload struct {
			Name    string `json:"name"`
			Command string `json:"command"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			httpError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if strings.TrimSpace(payload.Command) == "" {
			httpError(w, http.StatusBadRequest, "command is required")
			return
		}

		command, err := launchPresetCommand(payload.Name, payload.Command)
		if err != nil {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to launch command: %v", err))
			return
		}
		if err := command.Start(); err != nil {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to launch command: %v", err))
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
	}))
	// /api/run/{id} (POST) — run a saved preset by ID
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

		preset, err := presets.LoadByID(presetsDir, id)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.NotFound(w, r)
				return
			}
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}

		command, err := launchPresetCommand(preset.Name, preset.Command)
		if err != nil {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to launch command: %v", err))
			return
		}
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

func launchPresetCommand(name, commandLine string) (*exec.Cmd, error) {
	trimmed := strings.TrimSpace(commandLine)
	if trimmed == "" {
		return nil, errors.New("preset command is empty")
	}

	title := strings.ReplaceAll(strings.TrimSpace(name), `"`, "'")
	if title == "" {
		title = "PresetDock"
	}

	rawCmdLine := fmt.Sprintf(`cmd /C start "%s" cmd /K %s`, title, trimmed)

	command := exec.Command("cmd")
	command.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       rawCmdLine,
		CreationFlags: createNoWindow,
		HideWindow:    true,
	}
	return command, nil
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

// propagatePresetMembership adds the newPresetID to all decks and favourites
// that the sourcePresetID belongs to when duplicating a preset.
func propagatePresetMembership(presetsDir, sourcePresetID, newPresetID string) {
	// Propagate to favourites
	if sourcePresetID != newPresetID {
		favs, err := favourites.Load(presetsDir)
		if err == nil {
			isFav := false
			for _, id := range favs {
				if id == sourcePresetID {
					isFav = true
					break
				}
			}
			if isFav {
				alreadyFav := false
				for _, id := range favs {
					if id == newPresetID {
						alreadyFav = true
						break
					}
				}
				if !alreadyFav {
					favs = append(favs, newPresetID)
					_ = favourites.Save(presetsDir, favs)
				}
			}
		}
	}

	// Propagate to decks
	if sourcePresetID != newPresetID {
		dk, err := decks.Load(presetsDir)
		if err == nil {
			modified := false
			for i, deck := range dk {
				for _, id := range deck.PresetIDs {
					if id == sourcePresetID {
						alreadyThere := false
						for _, id2 := range deck.PresetIDs {
							if id2 == newPresetID {
								alreadyThere = true
								break
							}
						}
						if !alreadyThere {
							deck.PresetIDs = append(deck.PresetIDs, newPresetID)
							dk[i] = deck
							modified = true
						}
						break
					}
				}
			}
			if modified {
				_ = decks.Save(presetsDir, dk)
			}
		}
	}
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}
