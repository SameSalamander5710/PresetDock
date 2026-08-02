# Stage 03 - API and Runtime Split

This stage moves route registration and lifecycle logic out of `backend/main.go` so the composition root becomes thin.

## Goal

Extract the HTTP handlers into `backend/internal/api/` and the heartbeat and shutdown wiring into `backend/internal/runtime/`.

## Steps

1. Create an API package that owns route registration, request decoding, response encoding, and HTTP status handling.
2. Keep the API package dependent on storage helpers and the launcher interface only.
3. Move heartbeat tracking, stale shutdown, browser launch, and already-running detection into `backend/internal/runtime/`.
4. Reduce `backend/main.go` to wiring, startup, and shutdown orchestration.
5. Confirm the server still starts on `127.0.0.1:8765` and the browser still opens automatically.

## Keep Working After Completion

1. Keep the local-only runtime model intact.
2. Keep the shutdown path working from the UI.
3. Keep the health and heartbeat behavior unchanged.

## Validation

1. Run `go test ./backend/...`.
2. Start the app and confirm the browser opens.
3. Confirm the UI can still load, heartbeat, and shut down cleanly.
