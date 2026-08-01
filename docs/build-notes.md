# PresetDock Build Notes

## Current State

PresetDock is a local Windows launcher for llama.cpp command presets. The app serves a browser UI from a Go binary, reads preset JSON files from `presets/` next to the executable, and launches the saved command through `cmd.exe` so stdout/stderr stay visible in the terminal window.

## Current Behavior

- The root `presets/` folder is the source of truth for saved presets.
- The preset filename is the preset `id`.
- The JSON content stores the user-facing fields such as `name`, `engine`, `model`, `tags`, `description`, and `command`.
- The UI shows presets as wide vertical cards for readability.
- The UI supports Create, Edit, Delete, and Run actions from the browser.
- Run launches the preset through a visible `cmd.exe` session and keeps the output on screen.
- The UI also includes a small shutdown action that stops the local server on demand.
- A heartbeat keeps the server alive while the page is open; the server shuts down after about 2 minutes without a heartbeat.

## Preset Schema

Example:

```json
{
  "name": "Gemma 4 E4B - Q4",
  "engine": "llama-server",
  "model": "unsloth/gemma-4-E4B-it-qat-GGUF:UD-Q4_K_XL",
  "tags": ["gemma", "E4B", "QAT", "Q4"],
  "description": "General purpose, balanced speed/quality",
  "command": "llama-server -hf unsloth/gemma-4-E4B-it-qat-GGUF:UD-Q4_K_XL --spec-type draft-mtp --spec-draft-n-max 2 -fit on -ngl 999 --flash-attn on -c 8192 --temp 1.0 --top-p 0.95 --top-k 64 --mlock --host 127.0.0.1 --port 8080"
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
