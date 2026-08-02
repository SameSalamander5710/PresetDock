# Stage 01 - Storage and Validation

This stage extracts preset, favourite, and deck persistence helpers from `backend/main.go` into focused packages while keeping the app runnable.

## Goal

Move file-backed data logic into `backend/internal/presets/`, `backend/internal/favourites/`, and `backend/internal/decks/` without changing the JSON schema or runtime behavior.

## Steps

1. Create the storage packages and move the pure helpers first: JSON read and write, path helpers, and default empty-list behavior.
2. Move preset validation, preset ID validation, slugging, and example preset creation into `backend/internal/presets/`.
3. Move favourite list helpers into `backend/internal/favourites/`.
4. Move deck list helpers into `backend/internal/decks/`.
5. Update the call sites in `backend/main.go` to use the new packages.
6. Run the backend and confirm presets still load, favourites still persist, and decks still save.

## Keep Working After Completion

1. Preserve the existing JSON file names and fields.
2. Preserve the current API responses.
3. Keep `backend/main.go` compiling as the composition root.

## Validation

1. Run `go test ./backend/...`.
2. Start the app and confirm the preset list loads.
3. Edit or create a preset and confirm it still writes to disk.
