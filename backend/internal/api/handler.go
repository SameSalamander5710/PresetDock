package api

import (
	"sync"

	"presetdock/backend/internal/launcher"
	"presetdock/backend/internal/runtime"
)

// Handler holds the dependencies for all HTTP handlers.
type Handler struct {
	presetsDir string
	launcher   launcher.Launcher
	heartbeat  *runtime.Heartbeat
	shutdown   func()

	// mu serializes mutation requests. The JSON stores use read-modify-write
	// (load, change, save) without on-disk locking, so concurrent writes
	// could otherwise interleave and lose updates.
	mu sync.Mutex
}

// NewHandler returns a new Handler wired with the given dependencies.
func NewHandler(presetsDir string, l launcher.Launcher, hb *runtime.Heartbeat, shutdown func()) *Handler {
	return &Handler{
		presetsDir: presetsDir,
		launcher:   l,
		heartbeat:  hb,
		shutdown:   shutdown,
	}
}
