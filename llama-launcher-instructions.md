# Local llama.cpp launcher — build instructions

## Goal
A small personal Windows tool that replaces manually copy-pasting llama.cpp commands from a text file into a terminal. The user double-clicks one executable; it opens a browser tab showing saved "presets" (model + full command + tags); clicking Run launches that exact command in a new terminal window.

## Hard constraints (do not deviate)
- Language: Go, standard library only unless something is genuinely awkward without a dependency.
- Frontend: plain HTML/CSS/JS, no framework, no build step — embedded into the binary via `go:embed`.
- No Python, PyInstaller, Node, Electron, or any embedded browser runtime (WebView2 etc.). The app opens the user's normal default browser.
- Presets are external JSON files on disk (next to the exe), NOT embedded — must be editable without rebuilding.
- No `.bat` files anywhere in the design. JSON is the sole source of truth; Go executes the command string directly.
- Single-user, local-only tool. Never bind the server to anything but `127.0.0.1`.

## Folder structure to create
```
project-root/
  frontend/
    index.html
    styles.css
    app.js
  backend/
    main.go
  presets/
    example.json
  go.mod
  build.bat
```

## Preset file format
One JSON file per preset in `presets/`. Filename (minus `.json`) is its `id`.
```json
{
  "name": "Gemma 2 9B Q4",
  "engine": "llama-cli",
  "model": "gemma-2-9b-it-Q4_K_M.gguf",
  "tags": ["gemma", "9b", "q4"],
  "description": "General purpose, balanced speed/quality",
  "command": "llama-cli.exe -m models/gemma-2-9b-it-Q4_K_M.gguf -p \"You are a helpful assistant\" -n 512"
}
```
Scan `presets/*.json` fresh on each request — no caching needed at this scale.

## Backend requirements (`backend/main.go`)
- Locate the `presets/` folder relative to the **executable's own path** (`os.Executable()`), not the current working directory, so it works regardless of how the exe is launched. If missing, create it and write the example preset above so first run isn't empty.
- Serve the embedded `frontend/` directory at `/`.
- Add `POST /api/heartbeat` and track the last heartbeat timestamp in memory.
- Start a background shutdown monitor: if no heartbeat arrives for about 2 minutes, the server exits cleanly.
- `GET /api/presets` → JSON array of all parsed presets, each including its `id`.
- `POST /api/presets` → create a new preset JSON file from a submitted preset object.
- `PUT /api/presets/{id}` → update an existing preset JSON file, including rename support if the `id` changes.
- `DELETE /api/presets/{id}` → remove the preset JSON file.
- `POST /api/run/{id}` → look up the preset, run:
  `cmd /c start cmd /k <command>` — opens a new terminal window that stays open, so llama.cpp's streaming output is visible and interactive.
- Bind to `127.0.0.1:8765` (fixed port).
- After the listener starts, auto-open the default browser to `http://127.0.0.1:8765` via `cmd /c start <url>`.

## Frontend requirements
- Send a lightweight heartbeat to `POST /api/heartbeat` every 30 seconds while the page is open.
- On load, `fetch('/api/presets')` and render one card per preset: name, model, tags, description, the full command shown in a monospace block (visible, not hidden/truncated), and a Run button.
- Keep the main list in the old card-style UI. Add a `Create` button that opens a preset editor only when pressed, and an `Edit` button next to each preset's Run button that opens the same editor prefilled for that preset.
- Render the cards in a single vertical column so each card is wide enough to read long commands comfortably.
- Run button calls `POST /api/run/{id}`; show simple success/fail feedback (e.g. button label change or small toast) — no complex state needed.
- The create/edit editor should include all preset fields supported by the JSON format except `id`, and remain hidden until explicitly opened.
- Add a delete button on each preset card that calls `DELETE /api/presets/{id}`.
- Clean but simple styling. No frameworks, no bundler — must be servable as-is via `go:embed`.

## Build & run
- Dev loop: `go run ./backend` (recompiles fast; no separate step needed while iterating on Go or HTML/CSS/JS).
- Release build: `go build -o LlamaLauncher.exe ./backend`
  - Optional: add `-ldflags "-H=windowsgui"` to suppress the console flash on launch.
- `build.bat`: a one-line wrapper around the release build command.

## Explicit non-goals for this version
- No authentication, no network exposure beyond localhost.
- No `.bat` export/fallback (can be added later as a separate feature if wanted).

## Definition of done
Double-clicking the built `.exe` starts the local server, auto-opens the browser to it, displays every preset found in `presets/`, and clicking Run launches the exact saved command in its own terminal window.
