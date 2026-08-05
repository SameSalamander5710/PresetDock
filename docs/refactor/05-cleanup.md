# Stage 05 - Documentation and Legacy Cleanup

This stage removes the last sources of confusion once the modular split is in place.

## Goal

Make the architecture doc the main reference for future AI agents and retire the duplicate top-level frontend copy when it is no longer needed.

## Status: ✅ COMPLETE

## Changes Applied

### 1. Removed Stale `internal/` Directory
- Deleted the top-level `internal/` directory that was an incomplete mirror of `backend/internal/`.
- The mirror was missing entire packages (api, launcher, runtime) and contained inconsistent cross-imports into `backend/internal/`.
- No Go code referenced the stale path.

### 2. Removed Legacy `frontend/` Directory
- Deleted the top-level `frontend/` directory that was a 1:1 mirror of `backend/frontend/`.
- The canonical frontend lives in `backend/frontend/` and is embedded into the binary via `//go:embed`.

### 3. Updated `docs/architecture.md`
- Consolidated "Source of Truth Rules" to remove the legacy frontend mirror note.
- Added explicit rule: `backend/internal/` is the sole backend package path.
- Merged redundant frontend rules into a single canonical statement.

## Steps Completed

1. ✅ Verified stage docs matched the real folder structure after the code refactor.
2. ✅ Deleted the top-level `frontend/` directory (legacy mirror, no longer needed).
3. ✅ Deleted the top-level `internal/` directory (stale incomplete mirror).
4. ✅ Removed stale references from `docs/architecture.md`.

## Validation

1. ✅ Docs point to the canonical architecture file.
2. ✅ No ambiguity about which frontend tree is shipped (`backend/frontend/` is sole).
3. ✅ `go build ./backend` succeeds after cleanup.

## Keep Working After Completion

1. ✅ Repo map is easy for local AI agents to scan quickly.
2. ✅ Shipped frontend path is explicit (`backend/frontend/`).
3. ✅ Docs are aligned with the actual code layout.