// --------------------------------------------------------------------------
// app.js — Bootstrap entry point
// --------------------------------------------------------------------------
// This file wires together the modular frontend layers:
//   state.js    — shared state and mutation helpers
//   dom.js      — DOM lookup and assertion helpers
//   api         — fetch wrappers and endpoint helpers
//   heartbeat.js — client-side liveness loop
//   panes.js    — pane rendering, filtering, suggestions, view mode
//   dialogs.js  — preset editor + decks dialog logic
//   actions.js  — user actions, button wiring, orchestration
// --------------------------------------------------------------------------

// --------------------------------------------------------------------------
// Bootstrap
// --------------------------------------------------------------------------
async function init() {
  // Validate required DOM nodes before wiring anything
  const requiredIds = [
    'pane-left',
    'pane-right',
    'split-container',
    'status',
    'create-button',
    'decks-button',
    'decks-close',
    'shutdown-button',
    'view-toggle',
    'preset-dialog',
    'preset-editor',
    'decks-dialog',
  ];
  for (const id of requiredIds) {
    assertElement(id, 'bootstrap');
  }

  // Create pane states
  leftPane = createPaneState(document.getElementById('pane-left'));
  rightPane = createPaneState(document.getElementById('pane-right'));

  // Wire pane events
  wirePaneEvents(leftPane);
  wirePaneEvents(rightPane);

  // Wire global actions
  wireGlobalActions();

  // Wire outside-click handler
  wireOutsideClick();

  // Build refresh callback
  async function onRefresh() {
    await loadPresets();
    await loadDecks();
    await loadFavourites();
    syncDeckSelectors();
    renderAllPanes(cardActions);
  }

  // Build card action callbacks
  const cardActions = createCardActions(onRefresh);
  setCardActionsRef(cardActions);

  // Wire dialogs with callbacks
  const editorCallbacks = createEditorCallbacks(onRefresh);
  const deckCallbacks = createDeckCallbacks(onRefresh);
  wireDialogs(editorCallbacks.onEditorSave, editorCallbacks.onEditorRun, deckCallbacks.onDeckSave, deckCallbacks.onDeckDelete);

  // Load data
  await loadPresets();
  await loadDecks();
  await loadFavourites();

  // Sync deck selectors
  syncDeckSelectors();

  // Initial render
  renderAllPanes(cardActions);

  // Start heartbeat
  startHeartbeat();

  // Load server info if element exists
  loadServerInfo();

  // Mark bootstrap as successful
  setStatus('Ready');
}

// Wrap bootstrap in try/catch to catch startup failures
(async function bootstrap() {
  try {
    setStatus('Loading presets...');
    await init();
  } catch (error) {
    setStatus(`Failed to start: ${error.message}`, true);
    console.error('Bootstrap failed:', error);
  }
})();