# Stage 02 - Command Launcher Abstraction

This stage separates command execution from preset CRUD so future launchers can be added without rewriting the HTTP layer.

## Goal

Move the Windows command-launching logic out of `backend/main.go` and behind a launcher interface in `backend/internal/launcher/`.

## Steps

1. Define a small launcher interface that can run a command payload.
2. Move the current `cmd.exe` implementation into `backend/internal/launcher/` as the default launcher.
3. Change `/api/run` to depend on the launcher interface rather than building process commands itself.
4. Keep the current console-window behavior for launched presets.
5. Add any new shell support later as a new launcher implementation instead of modifying HTTP handlers.

## Keep Working After Completion

1. Keep Windows as the default path.
2. Keep `cmd.exe` behavior unchanged for existing presets.
3. Do not change preset files or API payloads.

## Validation

1. Run `go test ./backend/...`.
2. Start the app and run a preset from the UI.
3. Confirm the console window still opens and streams output.
