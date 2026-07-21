# PresetDock Build Notes

## Current State

PresetDock is a local Windows launcher for llama.cpp command presets. The app serves a browser UI from a Go binary, reads preset JSON files from `presets/` next to the executable, and launches the saved command in a new terminal window.

## Current Behavior

- The root `presets/` folder is the source of truth for saved presets.
- The preset filename is the preset `id`.
- The JSON content stores the user-facing fields such as `name`, `engine`, `model`, `tags`, `description`, and `command`.
- The UI shows presets as wide vertical cards for readability.
- The UI supports Create, Edit, Delete, and Run actions from the browser.
- A heartbeat keeps the server alive while the page is open; the server shuts down after about 2 minutes without a heartbeat.

## Preset Schema

Example:

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

## Build And Run

- Dev loop: `go run ./backend`
- Release build: `go build -ldflags "-H=windowsgui" -o PresetDock.exe ./backend`

## Notes

- The app binds only to `127.0.0.1:8765`.
- The browser opens automatically after startup.
- The UI is embedded into the binary with `go:embed`.
- The project intentionally stays standard-library only on the Go side.
