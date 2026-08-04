# PresetDock Architecture

This document is the canonical map for the repository. It is written for future human and AI contributors who need a compact, local understanding of how the app is structured and how to extend it safely.

## Repository Model

PresetDock is a small local-only Windows app with three main concerns:

1. Backend runtime and HTTP API.
2. Embedded browser frontend.
3. File-backed preset storage.

The repo should stay explicit and modular, not framework-like.

## Runtime Flow

1. `backend/main.go` starts the process and resolves the executable directory.
2. The preset directory is resolved and created if needed.
3. The embedded frontend from `backend/frontend/` is mounted.
4. The HTTP server binds to `127.0.0.1:8765`.
5. The browser opens to the local UI.
6. The UI talks to local API endpoints.
7. Presets, decks, and favourites are stored as JSON files in `presets/`.
8. Running a preset launches a Windows console window through the launcher layer.

## Backend Boundaries

### Composition Root

`backend/main.go` should only wire dependencies and start the process. It owns:

1. Path resolution.
2. Embedded frontend setup.
3. HTTP server startup and shutdown.
4. Browser launch.
5. Runtime lifecycle wiring.

### API Layer

`backend/internal/api/` should own:

1. Request decoding and response encoding.
2. Route registration.
3. Status codes and validation at the HTTP boundary.
4. Calling storage, runtime, and launcher helpers.

### Domain and Storage Layer

`backend/internal/presets/`, `backend/internal/favourites/`, and `backend/internal/decks/` should own:

1. Reading and writing JSON files.
2. Validation for their own data.
3. ID generation and slugging for presets.
4. Membership propagation helpers where needed.

### Launcher Layer

`backend/internal/launcher/` owns the command execution abstraction.

1. `launcher.go` defines the `Launcher` interface (`Prepare(CommandPayload) (*exec.Cmd, error)`) and the `CommandPayload` struct.
2. `windows.go` provides `WindowsCmdLauncher`, the default implementation using `cmd.exe /C start`.
3. `windows_test.go` validates empty-command rejection, title sanitization, default title fallback, and `SysProcAttr` flags.
4. HTTP handlers in `main.go` depend on the `Launcher` interface, not the concrete Windows implementation.
5. Future launchers (e.g., PowerShell, WSL) are added as new struct implementations without touching HTTP handlers.

### Runtime Layer

`backend/internal/runtime/` should own process lifecycle helpers.

1. Heartbeat tracking.
2. Stale shutdown.
3. Already-running detection.
4. Browser launch coordination.

## Frontend Boundaries

The shipped frontend lives in `backend/frontend/` and is embedded into the binary.

### Bootstrap

`backend/frontend/app.js` should initialize the page and compose the other frontend modules.

### Shared State

`backend/frontend/state.js` should own shared arrays and mutation helpers.

### API Helpers

`backend/frontend/api.js` should own fetch wrappers and endpoint names.

### Rendering

`backend/frontend/panes.js` should own pane state and rendering.

### Dialogs

`backend/frontend/dialogs.js` should own editor and deck dialog logic.

### Actions

`backend/frontend/actions.js` should own button wiring and user actions.

### Heartbeat

`backend/frontend/heartbeat.js` should own the client-side liveness loop.

### DOM Helpers

`backend/frontend/dom.js` should own safe DOM lookup and assertions.

## Source of Truth Rules

1. `presets/` is the source of truth for user data.
2. `backend/frontend/` is the source of truth for the shipped UI.
3. The top-level `frontend/` directory is legacy and should be treated as a mirror until removed.
4. The preset JSON schema should stay stable unless a migration is explicitly planned.

## Extension Rules

When adding a feature, use the smallest owning module.

1. If it changes command launching, start in `backend/internal/launcher/`.
2. If it changes persistence, start in the relevant storage package.
3. If it changes HTTP behavior, keep the logic in `backend/internal/api/`.
4. If it changes UI state or rendering, keep the logic in `backend/frontend/`.

## Refactor Guidance

The repository should be refactored in small steps so it remains runnable after each step.

1. Extract the least coupled logic first.
2. Keep each stage focused on one responsibility.
3. Validate the touched slice before moving to the next one.
4. Update docs as the structure changes.
