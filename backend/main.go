package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"presetdock/backend/internal/api"
	"presetdock/backend/internal/launcher"
	"presetdock/backend/internal/presets"
	"presetdock/backend/internal/runtime"
)

//go:embed frontend/*
var embeddedFrontend embed.FS

// version is set at build time via -ldflags "-X presetdock/backend.version=...".
// It stays "dev" for local `go run`/`go build` with no ldflags.
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("PresetDock", version)
		return
	}

	exeDir, err := executableDir()
	if err != nil {
		log.Fatal(err)
	}

	presetsDir := resolvePresetsDir(exeDir)
	// Migrate legacy preset files to the <slug>-<uid>.json layout and seed
	// a default example preset when the directory holds none.
	if err := presets.Migrate(presetsDir); err != nil {
		log.Fatalf("Failed to migrate presets directory: %v", err)
	}
	log.Printf("Preset storage initialized at %s", presetsDir)

	frontendFS, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	heartbeat := runtime.NewHeartbeat()
	dispatcher := launcher.NewDispatcher()

	// Start the stale batch-script sweeper (safety net for edge cases where
	// normal cleanup can't run, e.g. app crash).
	sweeperStop := make(chan struct{})
	launcher.StartStaleScriptSweeper(5*time.Minute, 30*time.Minute, sweeperStop)
	defer close(sweeperStop)

	mux := http.NewServeMux()

	var runner *runtime.Runner
	shutdown := func() {
		if runner != nil {
			runner.Shutdown()
		}
	}

	handler := api.NewHandler(presetsDir, dispatcher, heartbeat, shutdown)
	handler.Register(mux, frontendFS)

	listener, err := net.Listen("tcp", "127.0.0.1:8765")
	if err != nil {
		if runtime.LaunchIfAlreadyRunning() {
			return
		}
		log.Fatal(err)
	}

	if err := runtime.OpenBrowser("http://127.0.0.1:8765"); err != nil {
		log.Printf("browser launch failed: %v", err)
	}

	server := &http.Server{Handler: mux}
	runner = runtime.NewRunner(server)
	go runner.MonitorHeartbeat(heartbeat)

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func executableDir() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	resolvedPath, err := filepath.EvalSymlinks(executablePath)
	if err == nil {
		executablePath = resolvedPath
	}

	return filepath.Dir(executablePath), nil
}

func resolvePresetsDir(exeDir string) string {
	if strings.Contains(strings.ToLower(exeDir), "go-build") {
		if workingDir, err := os.Getwd(); err == nil {
			workingPresetsDir := filepath.Join(workingDir, "presets")
			if pathExists(workingPresetsDir) {
				return workingPresetsDir
			}
		}
	}

	return filepath.Join(exeDir, "presets")
}

func pathExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
