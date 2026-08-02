# Stage 05 - Documentation and Legacy Cleanup

This stage removes the last sources of confusion once the modular split is in place.

## Goal

Make the architecture doc the main reference for future AI agents and retire the duplicate top-level frontend copy when it is no longer needed.

## Steps

1. Update `instruction.md` and `blueprints.md` so they point at `docs/architecture.md` as the canonical repo map.
2. Check that the stage docs still match the real folder structure after the code refactor lands.
3. Decide whether the top-level `frontend/` directory should be deleted or kept as an explicit legacy mirror.
4. Remove any stale references that still describe the old monolithic layout.

## Keep Working After Completion

1. Keep the repo map easy for local AI agents to scan quickly.
2. Keep the shipped frontend path explicit.
3. Keep the docs aligned with the actual code layout.

## Validation

1. Verify the docs point to the canonical architecture file.
2. Confirm there is no ambiguity about which frontend tree is shipped.
3. Confirm the app still runs after the cleanup decisions.
