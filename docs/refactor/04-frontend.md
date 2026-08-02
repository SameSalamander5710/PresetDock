# Stage 04 - Frontend Modularization

This stage splits the shipped frontend into focused modules while preserving the current UI behavior.

## Goal

Keep `backend/frontend/app.js` as the bootstrap file and move state, API, rendering, dialog, action, heartbeat, and DOM concerns into separate modules.

## Steps

1. Extract shared state and mutation helpers into `backend/frontend/state.js`.
2. Extract fetch wrappers and endpoint helpers into `backend/frontend/api.js`.
3. Extract pane state and rendering into `backend/frontend/panes.js`.
4. Extract dialog logic into `backend/frontend/dialogs.js`.
5. Extract user actions and button wiring into `backend/frontend/actions.js`.
6. Extract heartbeat logic into `backend/frontend/heartbeat.js`.
7. Extract DOM lookup and assertion helpers into `backend/frontend/dom.js`.
8. Keep the UI loading and interacting exactly as it does now.

## Keep Working After Completion

1. Keep the embedded frontend path unchanged.
2. Keep the current page layout and workflows working.
3. Avoid changing the browser-visible behavior unless the change is intentionally documented.

## Validation

1. Start the app and confirm the page renders.
2. Load presets, edit a preset, duplicate a preset, and run a preset.
3. Verify favourites, decks, and heartbeat still work.
