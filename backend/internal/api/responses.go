package api

import (
	"encoding/json"
	"net/http"
)

// writeJSON marshals the given value to JSON and writes it with the provided status code.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

// httpError responds with a JSON error object.
func httpError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// methodNotAllowed responds with 405 and sets the Allow header.
func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	httpError(w, http.StatusMethodNotAllowed, "method not allowed")
}
