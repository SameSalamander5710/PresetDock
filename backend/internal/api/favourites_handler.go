package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"presetdock/backend/internal/favourites"
)

// HandleFavouritesList serves GET /api/favourites and POST /api/favourites.
func (h *Handler) HandleFavouritesList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		favs, err := favourites.Load(h.presetsDir)
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
		favs, err := favourites.Load(h.presetsDir)
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
		if err := favourites.Save(h.presetsDir, favs); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, favs)

	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

// HandleFavouriteByID serves DELETE /api/favourites/:id.
func (h *Handler) HandleFavouriteByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/favourites/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodDelete)
		return
	}

	favs, err := favourites.Load(h.presetsDir)
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

	if err := favourites.Save(h.presetsDir, newFavs); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, newFavs)
}
