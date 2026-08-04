// --------------------------------------------------------------------------
// Actions — user actions, button wiring, and orchestration
// --------------------------------------------------------------------------

// --------------------------------------------------------------------------
// Refresh helper: reload data + re-render
// --------------------------------------------------------------------------
async function refreshAll(cardActions) {
  await loadPresets();
  await loadDecks();
  await loadFavourites();
  syncDeckSelectors();
  renderAllPanes(cardActions);
}

// --------------------------------------------------------------------------
// Card action callbacks (passed to pane rendering)
// --------------------------------------------------------------------------
function createCardActions(onRefresh) {
  return {
    onToggleFavourite: async (presetId, isFavourite) => {
      const newIsFav = await toggleFavourite(presetId, isFavourite);
      await loadFavourites();
      return newIsFav;
    },

    onRun: async (presetId, presetName) => {
      await runPreset(presetId, presetName);
    },

    onEdit: (preset) => {
      openEditorForPreset(preset);
    },

    onDuplicate: async (preset) => {
      await duplicatePreset(preset);
      await onRefresh();
    },

    onDelete: async (presetId, presetName) => {
      await deletePreset(presetId, presetName);
      await onRefresh();
    },
  };
}

// --------------------------------------------------------------------------
// Editor callbacks
// --------------------------------------------------------------------------
function createEditorCallbacks(onRefresh) {
  return {
    onEditorSave: async (presetId, payload) => {
      if (presetId) {
        await savePreset(presetId, payload);
      } else {
        await createPreset(payload);
      }
      await onRefresh();
    },

    onEditorRun: async (name, command) => {
      await runCommand(name, command);
    },
  };
}

// --------------------------------------------------------------------------
// Deck callbacks
// --------------------------------------------------------------------------
function createDeckCallbacks(onRefresh) {
  return {
    onDeckSave: async (deckName, name, presetIds) => {
      if (deckName) {
        await saveDeck(deckName, name, presetIds);
      } else {
        await createDeckApi(name, presetIds);
      }
      await onRefresh();
    },

    onDeckDelete: async (deckName) => {
      await deleteDeckApi(deckName);
      // Clear deck filter on panes that had this deck selected
      [leftPane, rightPane].forEach((pane) => {
        if (pane && pane.activeDeckFilter === deckName) {
          pane.activeDeckFilter = '';
          pane.deckSelector.value = '';
        }
      });
      await onRefresh();
    },
  };
}

// --------------------------------------------------------------------------
// Wire global button handlers
// --------------------------------------------------------------------------
function wireGlobalActions() {
  assertElement('create-button', 'global actions').addEventListener('click', openCreateEditor);
  assertElement('decks-button', 'global actions').addEventListener('click', showDecksDialog);
  assertElement('decks-close', 'global actions').addEventListener('click', closeDecksDialog);

  assertElement('shutdown-button', 'global actions').addEventListener('click', async () => {
    if (!window.confirm('Shut down the PresetDock server?')) return;
    try {
      await shutdownServer();
      setStatus('Server shutting down...');
    } catch (error) {
      setStatus(error.message, true);
    }
  });

  assertElement('view-toggle', 'global actions').addEventListener('click', () => {
    setViewModeUI(viewMode === 'single' ? 'dual' : 'single');
  });
}

// --------------------------------------------------------------------------
// Close suggestions when clicking outside
// --------------------------------------------------------------------------
function wireOutsideClick() {
  document.addEventListener('click', (e) => {
    [leftPane, rightPane].forEach((pane) => {
      if (!pane) return;
      if (!pane.el.contains(e.target)) {
        hideSuggestions(pane);
      }
    });
  });
}