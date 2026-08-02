# PresetDock Blueprint

This is the working map for the repository. It should stay short and point future AI agents to the canonical architecture doc rather than duplicating the full design.

## Quick Read

1. Read [docs/architecture.md](docs/architecture.md) first.
2. Use the stage docs under [docs/refactor/](docs/refactor/) when planning or performing a modular refactor.
3. Treat `backend/main.go` as the composition root until extraction is complete.
4. Treat `backend/frontend/` as the shipped frontend tree.
5. Treat the top-level `frontend/` directory as legacy until it is intentionally removed.

## What This Repo Is

PresetDock is a small local-only Windows app with three concerns:

1. Backend runtime and HTTP API.
2. Embedded browser frontend.
3. File-backed preset storage.

## Current Structure

1. `backend/main.go` handles startup wiring, embedded assets, server startup, browser launch, and shutdown.
2. `backend/internal/presets/`, `backend/internal/favourites/`, and `backend/internal/decks/` are the intended storage boundaries.
3. `backend/internal/launcher/` is the intended command-launching boundary.
4. `backend/internal/api/` is the intended HTTP boundary.
5. `backend/internal/runtime/` is the intended lifecycle boundary.
6. `backend/frontend/` is the shipped frontend tree.
7. `frontend/` is a legacy duplicate that should not be treated as the primary UI source.

## Extension Rule

When adding a feature, find the owning layer first and keep the change inside the smallest module that can own it.

1. Launcher changes go to `backend/internal/launcher/`.
2. Persistence changes go to the relevant storage package.
3. HTTP changes go to `backend/internal/api/`.
4. UI changes go to `backend/frontend/`.

## Stability Rules

1. Keep the preset JSON schema stable unless a migration is explicitly planned.
2. Keep Windows behavior working.
3. Keep the app local-only.
4. Keep the embedded frontend working without runtime file lookup.

## Refactor Stages

1. Storage extraction: see [docs/refactor/01-storage.md](docs/refactor/01-storage.md).
2. Launcher extraction: see [docs/refactor/02-launcher.md](docs/refactor/02-launcher.md).
3. API and runtime extraction: see [docs/refactor/03-api-and-runtime.md](docs/refactor/03-api-and-runtime.md).
4. Frontend modularization: see [docs/refactor/04-frontend.md](docs/refactor/04-frontend.md).
5. Docs and cleanup: see [docs/refactor/05-cleanup.md](docs/refactor/05-cleanup.md).