package api

import (
	"io/fs"
	"net/http"
)

// apiBodyLimit caps API request bodies at 1 MiB. API payloads are small JSON
// documents, so anything larger is rejected instead of being buffered.
const apiBodyLimit = 1 << 20

// Register mounts all API routes on the provided mux and attaches the static
// frontend fallback handler. API routes go through a size-capped sub-mux so
// oversized request bodies are rejected before they reach a handler.
func (h *Handler) Register(mux *http.ServeMux, frontendFS fs.FS) {
	apiMux := http.NewServeMux()
	apiMux.Handle("/api/heartbeat", http.HandlerFunc(h.HandleHeartbeat))
	apiMux.Handle("/api/shutdown", http.HandlerFunc(h.HandleShutdown))
	apiMux.Handle("/api/presets", http.HandlerFunc(h.HandlePresetsList))
	apiMux.Handle("/api/presets/", http.HandlerFunc(h.HandlePresetByID))
	apiMux.Handle("/api/favourites", http.HandlerFunc(h.HandleFavouritesList))
	apiMux.Handle("/api/favourites/", http.HandlerFunc(h.HandleFavouriteByID))
	apiMux.Handle("/api/decks", http.HandlerFunc(h.HandleDecksList))
	apiMux.Handle("/api/decks/", http.HandlerFunc(h.HandleDeckByName))
	apiMux.Handle("/api/run", http.HandlerFunc(h.HandleRunDirect))
	apiMux.Handle("/api/run/", http.HandlerFunc(h.HandleRunByID))
	mux.Handle("/api/", limitedBody(apiMux, apiBodyLimit))
	mux.Handle("/", noCache(http.FileServer(http.FS(frontendFS))))
}

// limitedBody rejects requests whose declared body exceeds max with 413 and
// caps the readable body for requests without an honest Content-Length.
func limitedBody(next http.Handler, max int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > max {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.MaxBytesHandler(next, max).ServeHTTP(w, r)
	})
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
