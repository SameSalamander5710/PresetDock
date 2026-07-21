# PresetDock

PresetDock is a local-only Windows launcher for llama.cpp command presets.

## Run

- Dev loop: `go run ./backend`
- Release build: `go build -ldflags "-H=windowsgui" -o PresetDock.exe ./backend`
- Optional wrapper: `build.bat`

## Presets

Preset JSON files live in `presets/` next to the executable. Each file name becomes the preset id, and the backend scans the folder fresh on every request.
