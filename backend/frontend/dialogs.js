// --------------------------------------------------------------------------
// Dialog logic — preset editor dialog + decks dialog
// --------------------------------------------------------------------------

// --------------------------------------------------------------------------
// DOM references (resolved lazily)
// --------------------------------------------------------------------------
let _dialogRefs = null;
function dialogRefs() {
  if (!_dialogRefs) {
    _dialogRefs = {
      presetDialog: assertElement('preset-dialog', 'dialogs'),
      presetEditor: assertElement('preset-editor', 'dialogs'),
      presetDialogTitle: assertElement('preset-dialog-title', 'dialogs'),
      presetDialogSubtitle: assertElement('preset-dialog-subtitle', 'dialogs'),
      presetName: assertElement('preset-name', 'dialogs'),
      presetEngine: assertElement('preset-engine', 'dialogs'),
      presetModel: assertElement('preset-model', 'dialogs'),
      presetTags: assertElement('preset-tags', 'dialogs'),
      presetDescription: assertElement('preset-description', 'dialogs'),
      presetCommand: assertElement('preset-command', 'dialogs'),
      presetEditorFeedback: assertElement('preset-editor-feedback', 'dialogs'),
      editorSave: assertElement('editor-save', 'dialogs'),
      editorCancel: assertElement('editor-cancel', 'dialogs'),
      editorClose: assertElement('editor-close', 'dialogs'),
      editorRun: assertElement('editor-run', 'dialogs'),
      copyCommandBtn: assertElement('copy-command-btn', 'dialogs'),
      decksDialog: assertElement('decks-dialog', 'dialogs'),
      decksListEl: assertElement('decks-list', 'dialogs'),
      deckEditorEmpty: assertElement('deck-editor-empty', 'dialogs'),
      deckEditorForm: assertElement('deck-editor-form', 'dialogs'),
      deckNameInput: assertElement('deck-name-input', 'dialogs'),
      deckPresetsSearch: assertElement('deck-presets-search', 'dialogs'),
      deckPresetsList: assertElement('deck-presets-list', 'dialogs'),
      deckNewButton: assertElement('deck-new-button', 'dialogs'),
      deckSaveButton: assertElement('deck-save-button', 'dialogs'),
      deckDeleteButton: assertElement('deck-delete-button', 'dialogs'),
      deckFeedback: assertElement('deck-feedback', 'dialogs'),
    };
  }
  return _dialogRefs;
}

// --------------------------------------------------------------------------
// Preset editor dialog state
// --------------------------------------------------------------------------
let editingPresetId = null;

function openEditorForPreset(preset) {
  const r = dialogRefs();
  editingPresetId = preset.id;
  r.presetDialogTitle.textContent = 'Edit preset';
  r.presetDialogSubtitle.textContent = `Editing: ${preset.name || preset.id}`;
  r.presetName.value = preset.name || '';
  r.presetEngine.value = preset.engine || '';
  r.presetModel.value = preset.model || '';
  r.presetTags.value = (preset.tags || []).join(', ');
  r.presetDescription.value = preset.description || '';
  r.presetCommand.value = preset.command || '';
  r.presetEditorFeedback.textContent = '';
  r.presetEditorFeedback.classList.remove('error');
  r.editorSave.textContent = 'Save changes';
  r.presetDialog.showModal();
}

function openCreateEditor() {
  const r = dialogRefs();
  editingPresetId = null;
  r.presetDialogTitle.textContent = 'Create preset';
  r.presetDialogSubtitle.textContent = 'Fill out the fields and save the preset JSON.';
  r.presetName.value = '';
  r.presetEngine.value = '';
  r.presetModel.value = '';
  r.presetTags.value = '';
  r.presetDescription.value = '';
  r.presetCommand.value = '';
  r.presetEditorFeedback.textContent = '';
  r.presetEditorFeedback.classList.remove('error');
  r.editorSave.textContent = 'Save preset';
  r.presetDialog.showModal();
}

function closeEditor() {
  const r = dialogRefs();
  r.presetDialog.close();
  editingPresetId = null;
}

// --------------------------------------------------------------------------
// Deck dialog state
// --------------------------------------------------------------------------
let editingDeckName = null;

function showDecksDialog() {
  const r = dialogRefs();
  editingDeckName = null;
  r.deckEditorEmpty.hidden = true;
  r.deckEditorForm.hidden = true;
  renderDecksList();
  r.decksDialog.showModal();
}

function closeDecksDialog() {
  const r = dialogRefs();
  r.decksDialog.close();
  editingDeckName = null;
}

function showNewDeckUI() {
  const r = dialogRefs();
  editingDeckName = null;
  r.deckNameInput.value = '';
  r.deckPresetsSearch.value = '';
  r.deckEditorEmpty.hidden = true;
  r.deckEditorForm.hidden = false;
  r.deckFeedback.textContent = '';
  r.deckFeedback.classList.remove('error');
  renderDeckPresetsList([]);
  renderDecksList();
}

function renderDecksList() {
  const r = dialogRefs();
  r.decksListEl.innerHTML = '';

  if (decksCache.length === 0) {
    const empty = document.createElement('div');
    empty.className = 'empty';
    empty.textContent = 'No decks created yet.';
    r.decksListEl.appendChild(empty);
    return;
  }

  decksCache.forEach((deck) => {
    const item = document.createElement('div');
    item.className = 'deck-list-item' + (editingDeckName === deck.name ? ' selected' : '');

    const label = document.createElement('span');
    label.className = 'deck-list-label';
    label.textContent = deck.name;

    const count = document.createElement('span');
    count.className = 'deck-list-count';
    count.textContent = `${deck.preset_ids.length} preset${deck.preset_ids.length === 1 ? '' : 's'}`;

    item.append(label, count);
    item.addEventListener('click', () => selectDeckForEditing(deck.name));
    r.decksListEl.appendChild(item);
  });
}

function selectDeckForEditing(deckName) {
  const r = dialogRefs();
  editingDeckName = deckName;
  const deck = decksCache.find((d) => d.name === deckName);
  if (!deck) return;

  r.deckNameInput.value = deck.name;
  r.deckEditorEmpty.hidden = true;
  r.deckEditorForm.hidden = false;
  r.deckFeedback.textContent = '';
  r.deckFeedback.classList.remove('error');

  renderDeckPresetsList(deck.preset_ids);
  renderDecksList();
}

function renderDeckPresetsList(selectedIds) {
  const r = dialogRefs();
  const searchQuery = r.deckPresetsSearch.value.toLowerCase().trim();
  r.deckPresetsList.innerHTML = '';

  let available = presetsCache.slice();
  if (searchQuery) {
    available = available.filter((p) =>
      (p.name || '').toLowerCase().includes(searchQuery) || (p.id || '').toLowerCase().includes(searchQuery)
    );
  }

  if (available.length === 0) {
    const empty = document.createElement('div');
    empty.className = 'empty';
    empty.textContent = 'No presets found.';
    r.deckPresetsList.appendChild(empty);
    return;
  }

  available.forEach((preset) => {
    const item = document.createElement('label');
    item.className = 'deck-preset-item';

    const cb = document.createElement('input');
    cb.type = 'checkbox';
    cb.checked = selectedIds.includes(preset.id);

    const label = document.createElement('span');
    label.textContent = preset.name || preset.id;

    item.append(cb, label);
    r.deckPresetsList.appendChild(item);
  });
}

// --------------------------------------------------------------------------
// Wire dialog event listeners (called once from bootstrap)
// --------------------------------------------------------------------------
function wireDialogs(onEditorSave, onEditorRun, onDeckSave, onDeckDelete) {
  const r = dialogRefs();

  // Preset editor form submit
  r.presetEditor.addEventListener('submit', async (e) => {
    e.preventDefault();

    const tagsText = r.presetTags.value;
    const tags = tagsText
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);

    const payload = {
      name: r.presetName.value.trim(),
      engine: r.presetEngine.value.trim(),
      model: r.presetModel.value.trim(),
      tags,
      description: r.presetDescription.value.trim(),
      command: r.presetCommand.value.trim(),
    };

    if (!payload.name && !payload.command) {
      r.presetEditorFeedback.classList.add('error');
      r.presetEditorFeedback.textContent = 'At least a name or command is required.';
      return;
    }

    r.editorSave.disabled = true;
    r.presetEditorFeedback.classList.remove('error');
    r.presetEditorFeedback.textContent = 'Saving...';

    try {
      await onEditorSave(editingPresetId, payload);
      r.presetEditorFeedback.textContent = editingPresetId ? 'Preset updated.' : 'Preset created.';
      setTimeout(closeEditor, 600);
    } catch (error) {
      r.presetEditorFeedback.classList.add('error');
      r.presetEditorFeedback.textContent = error.message || 'Save failed.';
    } finally {
      r.editorSave.disabled = false;
    }
  });

  // Editor run button
  r.editorRun.addEventListener('click', async () => {
    const command = r.presetCommand.value.trim();
    if (!command) {
      r.presetEditorFeedback.classList.add('error');
      r.presetEditorFeedback.textContent = 'Command is required to run.';
      return;
    }

    r.editorRun.disabled = true;
    r.presetEditorFeedback.classList.remove('error');
    r.presetEditorFeedback.textContent = 'Launching...';

    try {
      await onEditorRun(r.presetName.value.trim() || 'PresetDock', command);
      r.presetEditorFeedback.textContent = 'Command launched.';
    } catch (error) {
      r.presetEditorFeedback.classList.add('error');
      r.presetEditorFeedback.textContent = error.message || 'Run failed.';
    } finally {
      r.editorRun.disabled = false;
    }
  });

  // Copy command button
  r.copyCommandBtn.addEventListener('click', async () => {
    const command = r.presetCommand.value;
    if (!command) return;
    try {
      await navigator.clipboard.writeText(command);
      r.copyCommandBtn.title = 'Copied!';
      setTimeout(() => {
        r.copyCommandBtn.title = 'Copy command';
      }, 1500);
    } catch {
      r.copyCommandBtn.title = 'Copy failed';
    }
  });

  // Cancel / close
  r.editorCancel.addEventListener('click', closeEditor);
  r.editorClose.addEventListener('click', closeEditor);

  // Deck presets search
  r.deckPresetsSearch.addEventListener('input', () => {
    if (editingDeckName) {
      const deck = decksCache.find((d) => d.name === editingDeckName);
      if (deck) renderDeckPresetsList(deck.preset_ids);
    }
  });

  // Deck new button
  r.deckNewButton.addEventListener('click', showNewDeckUI);

  // Deck save button
  r.deckSaveButton.addEventListener('click', async () => {
    const name = r.deckNameInput.value.trim();
    if (!name) {
      r.deckFeedback.classList.add('error');
      r.deckFeedback.textContent = 'Deck name is required.';
      return;
    }

    const checkboxes = r.deckPresetsList.querySelectorAll('.deck-preset-item input[type="checkbox"]');
    const presetIds = [];
    checkboxes.forEach((cb) => {
      if (cb.checked) {
        const labelText = cb.parentElement.querySelector('span').textContent;
        const preset = presetsCache.find((p) => (p.name || p.id) === labelText);
        if (preset) presetIds.push(preset.id);
      }
    });

    r.deckSaveButton.disabled = true;
    r.deckFeedback.classList.remove('error');
    r.deckFeedback.textContent = 'Saving...';

    try {
      await onDeckSave(editingDeckName, name, presetIds);
      r.deckFeedback.textContent = 'Deck saved.';
      if (!editingDeckName) editingDeckName = name;
      renderDecksList();
      selectDeckForEditing(name);
    } catch (error) {
      r.deckFeedback.classList.add('error');
      r.deckFeedback.textContent = error.message || 'Save failed.';
    } finally {
      r.deckSaveButton.disabled = false;
    }
  });

  // Deck delete button
  r.deckDeleteButton.addEventListener('click', async () => {
    if (!editingDeckName) return;
    if (!window.confirm(`Delete deck "${editingDeckName}"?`)) return;

    r.deckDeleteButton.disabled = true;
    r.deckFeedback.classList.remove('error');
    r.deckFeedback.textContent = 'Deleting...';

    try {
      await onDeckDelete(editingDeckName);
      r.deckFeedback.textContent = 'Deck deleted.';
      r.deckEditorEmpty.hidden = true;
      r.deckEditorForm.hidden = true;
      editingDeckName = '';
      renderDecksList();
    } catch (error) {
      r.deckFeedback.classList.add('error');
      r.deckFeedback.textContent = error.message || 'Delete failed.';
    } finally {
      r.deckDeleteButton.disabled = false;
    }
  });
}