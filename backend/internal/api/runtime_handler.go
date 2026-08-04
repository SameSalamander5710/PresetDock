package api

import "net/http"

// HandleHeartbeat serves POST /api/heartbeat.
func (h *Handler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	h.heartbeat.Touch()
	w.WriteHeader(http.StatusNoContent)
}

// HandleShutdown serves POST /api/shutdown.
func (h *Handler) HandleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	if h.shutdown != nil {
		go h.shutdown()
	}

	w.WriteHeader(http.StatusNoContent)
}
