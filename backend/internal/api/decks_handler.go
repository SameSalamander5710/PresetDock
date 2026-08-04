package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"presetdock/backend/internal/decks"
)

// HandleDecksList serves GET /api/decks and POST /api/decks.
func (h *Handler) HandleDecksList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		decksList, err := decks.Load(h.presetsDir)
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
		decksList, err := decks.Load(h.presetsDir)
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
		if err := decks.Save(h.presetsDir, decksList); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, deck)

	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

// HandleDeckByName serves PUT /api/decks/:name and DELETE /api/decks/:name.
func (h *Handler) HandleDeckByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/decks/")
	if name == "" || strings.Contains(name, "/") {
		http.NotFound(w, r)
		return
	}

	decksList, err := decks.Load(h.presetsDir)
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
		if err := decks.Save(h.presetsDir, decksList); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		decksList = append(decksList[:deckIdx], decksList[deckIdx+1:]...)
		if err := decks.Save(h.presetsDir, decksList); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		methodNotAllowed(w, http.MethodPut+", "+http.MethodDelete)
	}
}
