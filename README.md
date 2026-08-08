# PresetDock

PresetDock is a local-only Windows launcher and storage layer for llama.cpp command presets.


> "A glorified text file to save your frequently used commands." - _SameSalamander5710, probably._

It is designed to keep saved llama.cpp commands in plain JSON files, then expose them through a small browser UI so you can create, edit, run, and save code presets without copying commands in and out of a terminal.

<p align="center">
	<img src="docs/images/Screenshot.gif" alt="PresetDock UI" width="65%" />
</p>

This is not a general-purpose llama.cpp wrapper. PresetDock simply keeps your own llama.cpp command lines organized and launchable.

Preset files live in `presets/` next to the executable, and the filename is the preset id. The browser UI is just a front end over those files.

## Installation

- Simply download and run the latest `PresetDock.exe` binary.

## Run

- Dev loop: `go run ./backend`
- Release build: `go build -ldflags "-H=windowsgui" -o PresetDock.exe ./backend`
- Running a preset opens a `cmd.exe` window and streams the command output there.

For build and implementation notes, see [docs/build-notes.md](docs/build-notes.md).
