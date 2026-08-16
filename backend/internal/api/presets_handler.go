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
			Shell:       req.Shell,
		}

		savedPreset, err := presets.Save(h.presetsDir, preset)
		if err != nil {
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}

		// If this is a duplicate (source_preset_id provided), propagate deck and favourite membership
		if req.SourcePresetID != "" {
			h.propagatePresetMembership(req.SourcePresetID, savedPreset.ID)
		}

		// An explicit deck_names list takes final say over the propagated
		// membership (central membership writer). A nil list leaves
		// membership as propagated (or empty for a plain create).
		if req.DeckNames != nil {
			if err := decks.SetPresetDecks(h.presetsDir, savedPreset.ID, *req.DeckNames); err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
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
		var req presets.UpdatePresetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpError(w, http.StatusBadRequest, "invalid preset JSON")
			return
		}

		savedPreset, err := presets.Update(h.presetsDir, id, req.Preset)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.NotFound(w, r)
				return
			}
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Apply the requested deck membership (central membership writer).
		// A nil DeckNames means the client left membership untouched. The new
		// ID is used so a rename and a membership change in the same save
		// compose with the ID remap done by presets.Update.
		if req.DeckNames != nil {
			if err := decks.SetPresetDecks(h.presetsDir, savedPreset.ID, *req.DeckNames); err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
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
		// Drop the dead reference from decks.json and favourites.json so the
		// central relation files never keep IDs of deleted presets.
		if err := decks.RemovePreset(h.presetsDir, id); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := favourites.RemoveID(h.presetsDir, id); err != nil {
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

	// Propagate to decks via the central membership writer.
	if sourcePresetID != newPresetID {
		dk, err := decks.Load(h.presetsDir)
		if err == nil {
			names := decks.DecksForPreset(dk, sourcePresetID)
			if len(names) > 0 {
				_ = decks.SetPresetDecks(h.presetsDir, newPresetID, names)
			}
		}
	}
}
