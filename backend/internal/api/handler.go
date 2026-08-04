package api

import (
	"presetdock/backend/internal/launcher"
	"presetdock/backend/internal/runtime"
)

// Handler holds the dependencies for all HTTP handlers.
type Handler struct {
	presetsDir string
	launcher   launcher.Launcher
	heartbeat  *runtime.Heartbeat
	shutdown   func()
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
