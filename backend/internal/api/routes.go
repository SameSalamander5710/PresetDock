package api

import (
	"io/fs"
	"net/http"
)

// Register mounts all API routes on the provided mux and attaches the static
// frontend fallback handler.
func (h *Handler) Register(mux *http.ServeMux, frontendFS fs.FS) {
	mux.Handle("/api/heartbeat", http.HandlerFunc(h.HandleHeartbeat))
	mux.Handle("/api/shutdown", http.HandlerFunc(h.HandleShutdown))
	mux.Handle("/api/presets", http.HandlerFunc(h.HandlePresetsList))
	mux.Handle("/api/presets/", http.HandlerFunc(h.HandlePresetByID))
	mux.Handle("/api/favourites", http.HandlerFunc(h.HandleFavouritesList))
	mux.Handle("/api/favourites/", http.HandlerFunc(h.HandleFavouriteByID))
	mux.Handle("/api/decks", http.HandlerFunc(h.HandleDecksList))
	mux.Handle("/api/decks/", http.HandlerFunc(h.HandleDeckByName))
	mux.Handle("/api/run", http.HandlerFunc(h.HandleRunDirect))
	mux.Handle("/api/run/", http.HandlerFunc(h.HandleRunByID))
	mux.Handle("/", noCache(http.FileServer(http.FS(frontendFS))))
}

// noCache wraps an HTTP handler with cache-busting headers so the embedded
// frontend is never served from a stale cache.
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}
