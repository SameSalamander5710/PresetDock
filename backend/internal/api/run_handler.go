package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"presetdock/backend/internal/launcher"
	"presetdock/backend/internal/presets"
)

// HandleRunDirect serves POST /api/run (direct command payload).
func (h *Handler) HandleRunDirect(w http.ResponseWriter, r *http.Request) {
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

	command, err := h.launcher.Prepare(launcher.CommandPayload{
		Title:   payload.Name,
		Command: payload.Command,
	})
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to launch command: %v", err))
		return
	}
	if err := command.Start(); err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to launch command: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// HandleRunByID serves POST /api/run/:id (run a saved preset by ID).
func (h *Handler) HandleRunByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/run/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}

	preset, err := presets.LoadByID(h.presetsDir, id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	command, err := h.launcher.Prepare(launcher.CommandPayload{
		Title:   preset.Name,
		Command: preset.Command,
	})
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to launch command: %v", err))
		return
	}
	if err := command.Start(); err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to launch command: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}
