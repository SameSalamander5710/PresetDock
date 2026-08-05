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

`backend/main.go` is a thin wiring layer. It resolves paths, creates the embedded
frontend filesystem, constructs dependencies, registers routes, starts the HTTP
server, and coordinates shutdown. It should not contain route handlers, HTTP
logic, or lifecycle state.

### API Layer

`backend/internal/api/` owns the HTTP boundary.

- `handler.go` — `Handler` struct and `NewHandler()` constructor.
- `routes.go` — `Register()` mounts all routes on `*http.ServeMux` and attaches
  the static frontend fallback with cache-busting headers.
- `responses.go` — `writeJSON()`, `httpError()`, and `methodNotAllowed()` helpers.
- `presets_handler.go` — `GET /api/presets`, `GET /api/presets/:id`,
  `POST /api/presets`, `PATCH /api/presets/:id`, `DELETE /api/presets/:id`.
- `favourites_handler.go` — `GET /api/favourites`,
  `POST /api/favourites/:presetId`, `DELETE /api/favourites/:presetId`.
- `decks_handler.go` — `GET /api/decks`, `GET /api/decks/:name`,
  `POST /api/decks`, `PATCH /api/decks/:name`, `DELETE /api/decks/:name`,
  `POST /api/decks/:name/presets/:presetId`, `DELETE /api/decks/:name/presets/:presetId`.
- `run_handler.go` — `POST /api/run` (direct command), `POST /api/run/:id` (preset).
- `runtime_handler.go` — `POST /api/heartbeat`, `POST /api/shutdown`.

The API layer depends on storage helpers, the `Launcher` interface, the
`Heartbeat` state, and a shutdown callback. It does not own process lifecycle.

### Domain and Storage Layer

`backend/internal/presets/`, `backend/internal/favourites/`, and
`backend/internal/decks/` should own:

1. Reading and writing JSON files.
2. Validation for their own data.
3. ID generation and slugging for presets.
4. Membership propagation helpers where needed.

### Launcher Layer

`backend/internal/launcher/` owns the command execution abstraction.

1. `launcher.go` defines the `Launcher` interface (`Prepare(CommandPayload) (*exec.Cmd, error)`) and the `CommandPayload` struct.
2. `windows.go` provides `WindowsCmdLauncher`, the default implementation using `cmd.exe /C start`.
3. `windows_test.go` validates empty-command rejection, title sanitization, default title fallback, and `SysProcAttr` flags.
4. HTTP handlers depend on the `Launcher` interface, not the concrete Windows implementation.
5. Future launchers (e.g., PowerShell, WSL) are added as new struct implementations without touching HTTP handlers.

### Runtime Layer

`backend/internal/runtime/` owns process lifecycle helpers.

- `heartbeat.go` — `Heartbeat` state with `LastActive()`, `Touch()`, and
  `IsStale(bool)` methods.
- `runner.go` — `Runner` struct that wraps `*http.Server`, exposes
  `MonitorHeartbeat()` (background goroutine) and `Shutdown()` (graceful
  server shutdown).
- `browser.go` — `OpenBrowser()` launches the default browser via `cmd.exe`.
- `already_running.go` — `LaunchIfAlreadyRunning()` redirects to the existing
  instance when port 8765 is already in use.

## Frontend Boundaries

The shipped frontend lives in `backend/frontend/` and is embedded into the binary.
Scripts are loaded in dependency order via `defer` in `index.html`.

### Module Dependency Order

```
state.js  →  dom.js  →  api  →  heartbeat.js  →  panes.js  →  dialogs.js  →  actions.js  →  app.js
```

### Bootstrap

`backend/frontend/app.js` is the thin bootstrap entry point. It validates required
DOM nodes, creates pane states, wires events, composes callback factories from
actions, loads initial data, triggers the first render, and starts the heartbeat.
It contains no business logic, rendering, or API calls.

### Shared State

`backend/frontend/state.js` owns all shared arrays (`presets`, `decks`,
`favourites`) and their mutation helpers (`setPresets`, `setDecks`,
`setFavourites`, `addPreset`, `updatePreset`, `removePreset`, `addDeck`,
`updateDeck`, `removeDeck`, `addFavourite`, `removeFavourite`). It also owns
the `setStatus()` helper for updating the status bar.

### DOM Helpers

`backend/frontend/dom.js` owns safe DOM lookup (`getElement`, `assertElement`)
and DOM creation helpers (`createSelectOption`). `assertElement` throws a
descriptive error when a required node is missing, which fails fast during
bootstrap instead of producing silent `null` errors later.

### API Helpers

`backend/frontend/api` (extensionless to avoid Go embed content-type issues)
owns all `fetch` calls and endpoint helpers: `loadServerInfo`, `loadPresets`,
`loadDecks`, `loadFavourites`, `toggleFavourite`, `runPreset`, `duplicatePreset`,
`deletePreset`, `savePreset`, `createPreset`, `saveDeck`, `createDeckApi`,
`deleteDeckApi`, `runCommand`, `shutdownServer`.

### Heartbeat

`backend/frontend/heartbeat.js` owns the client-side liveness loop.
`startHeartbeat()` posts to `/api/heartbeat` every 5s. `stopHeartbeat()`
cleans up the interval. `HEARTBEAT_INTERVAL` and `HEARTBEAT_TIMEOUT` are
configurable constants.

### Rendering

`backend/frontend/panes.js` owns pane state, rendering, filtering, suggestions,
and view mode. Key exports: `createPaneState()`, `wirePaneEvents()`,
`renderAllPanes()`, `renderPane()`, `filterPresets()`, `buildSuggestions()`,
`showSuggestions()`/`hideSuggestions()`, `syncDeckSelectors()`,
`setViewModeUI()`, `openCreateEditor()`.

### Dialogs

`backend/frontend/dialogs.js` owns the preset editor dialog and decks management
dialog. `wireDialogs()` connects all dialog buttons to callback functions.
`openEditorForPreset()` pre-fills the editor. Deck helpers manage the deck
list, editor form, and preset checkboxes.

### Actions

`backend/frontend/actions.js` owns user actions, button wiring, and orchestration.
`createCardActions()` returns card-level callbacks (toggle favourite, run, edit,
duplicate, delete). `createEditorCallbacks()` returns editor save/run callbacks.
`createDeckCallbacks()` returns deck save/delete callbacks. `wireGlobalActions()`
attaches global button handlers. `wireOutsideClick()` closes suggestions when
clicking outside a pane.

## Source of Truth Rules

1. `presets/` is the source of truth for user data.
2. `backend/frontend/` is the sole frontend directory, the source of truth for the shipped UI, and is embedded into the binary.
3. `backend/internal/` is the sole backend package path and owns all Go domain logic.
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
