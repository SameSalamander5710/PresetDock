package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"presetdock/backend/internal/decks"
	"presetdock/backend/internal/favourites"
	"presetdock/backend/internal/presets"
)

// HandlePresetsList serves GET /api/presets and POST /api/presets.
func (h *Handler) HandlePresetsList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		presetList, err := presets.Load(h.presetsDir)
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

		savedPreset, err := presets.Save(h.presetsDir, "", preset, false)
		if err != nil {
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}

		// If this is a duplicate (source_preset_id provided), propagate deck and favourite membership
		if req.SourcePresetID != "" {
			h.propagatePresetMembership(req.SourcePresetID, savedPreset.ID)
		}

		writeJSON(w, http.StatusCreated, savedPreset)

	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

// HandlePresetByID serves PUT /api/presets/:id and DELETE /api/presets/:id.
func (h *Handler) HandlePresetByID(w http.ResponseWriter, r *http.Request) {
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

		savedPreset, err := presets.Save(h.presetsDir, id, preset, true)
		if err != nil {
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, savedPreset)

	case http.MethodDelete:
		if err := presets.Delete(h.presetsDir, id); err != nil {
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
}

// propagatePresetMembership adds the newPresetID to all decks and favourites
// that the sourcePresetID belongs to when duplicating a preset.
func (h *Handler) propagatePresetMembership(sourcePresetID, newPresetID string) {
	// Propagate to favourites
	if sourcePresetID != newPresetID {
		favs, err := favourites.Load(h.presetsDir)
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
					_ = favourites.Save(h.presetsDir, favs)
				}
			}
		}
	}

	// Propagate to decks
	if sourcePresetID != newPresetID {
		dk, err := decks.Load(h.presetsDir)
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
				_ = decks.Save(h.presetsDir, dk)
			}
		}
	}
}
