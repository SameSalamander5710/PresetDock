// --------------------------------------------------------------------------
// Minimal telemetry / server-info helpers (unchanged)
// --------------------------------------------------------------------------
const apiBase = '';

async function loadServerInfo() {
  try {
    const res = await fetch(`${apiBase}/api/info`);
    if (!res.ok) return;
    const data = await res.json();
    const el = document.getElementById('server-info');
    if (el && data) {
      el.textContent = `Server running on port ${data.port} | OS: ${data.os}`;
    }
  } catch {
    // ignore
  }
}

// --------------------------------------------------------------------------
// Heartbeat
// --------------------------------------------------------------------------
let heartbeatInterval = null;

function startHeartbeat() {
  if (heartbeatInterval) return;
  heartbeatInterval = setInterval(async () => {
    try {
      const res = await fetch(`${apiBase}/api/heartbeat`, { method: 'POST' });
      if (res.ok) {
        const data = await res.json();
        const statusEl = document.getElementById('status');
        if (statusEl) {
          statusEl.textContent = data.message || '';
        }
      }
    } catch {
      // ignore heartbeat failures
    }
  }, 30000);
}

// --------------------------------------------------------------------------
// Shared caches
// --------------------------------------------------------------------------
let presetsCache = [];
let decksCache = [];
let favouritesCache = [];

// --------------------------------------------------------------------------
// View mode state
// --------------------------------------------------------------------------
let viewMode = 'single'; // 'single' | 'dual'

// --------------------------------------------------------------------------
// Pane state factory
// --------------------------------------------------------------------------
function createPaneState(paneElement) {
  const searchInput = paneElement.querySelector('.pane-search-input');
  const searchClear = paneElement.querySelector('.pane-search-clear');
  const suggestionsContainer = paneElement.querySelector('.pane-search-suggestions');
  const filterChipsContainer = paneElement.querySelector('.pane-filter-chips');
  const deckSelector = paneElement.querySelector('.pane-deck-selector');
  const presetsContainer = paneElement.querySelector('.pane-presets');
  const filterCountSpan = paneElement.querySelector('.pane-filter-count');

  return {
    el: paneElement,
    searchInput,
    searchClear,
    suggestionsContainer,
    filterChipsContainer,
    deckSelector,
    presetsContainer,
    filterCountSpan,
    searchText: '',
    activeFilters: [], // [{type: 'tag'|'engine', value: string}]
    activeDeckFilter: '',
    suggestionHighlightIndex: -1,
    currentSuggestions: [],
  };
}

// Pane instances
let leftPane = null;
let rightPane = null;

// --------------------------------------------------------------------------
// Preset card creation (shared across panes)
// --------------------------------------------------------------------------
function createPresetCard(preset, isFavourite) {
  const card = document.createElement('div');
  card.className = 'card';
  card.dataset.id = preset.id;

  const header = document.createElement('div');
  header.className = 'card-header';

  const titleCol = document.createElement('div');
  titleCol.className = 'card-title-col';

  const titleRow = document.createElement('div');
  titleRow.className = 'card-title-row';

  const star = document.createElement('button');
  star.className = 'favourite-star' + (isFavourite ? ' active' : '');
  star.textContent = isFavourite ? '★' : '☆';
  star.title = isFavourite ? 'Remove from favourites' : 'Add to favourites';
  star.addEventListener('click', async (e) => {
    e.stopPropagation();
    try {
      const resp = await fetch(`${apiBase}/api/favourites/${encodeURIComponent(preset.id)}`, {
        method: isFavourite ? 'DELETE' : 'POST',
      });
      if (!resp.ok && resp.status !== 204) {
        const err = await resp.json().catch(() => ({}));
        throw new Error(err.error || 'Favourite toggle failed');
      }
      if (isFavourite) {
        star.classList.remove('active');
        star.textContent = '☆';
        star.title = 'Add to favourites';
      } else {
        star.classList.add('active');
        star.textContent = '★';
        star.title = 'Remove from favourites';
      }
    } catch (error) {
      setStatus(error.message, true);
    }
  });

  const h2 = document.createElement('h2');
  h2.textContent = preset.name || preset.id;

  titleRow.append(star, h2);

  const model = document.createElement('p');
  model.className = 'model';
  model.textContent = preset.model || '';

  titleCol.append(titleRow, model);

  const actions = document.createElement('div');
  actions.className = 'action-buttons';

  // Run button
  const runBtn = document.createElement('button');
  runBtn.type = 'button';
  runBtn.textContent = 'Run';
  runBtn.title = 'Execute command in terminal';
  runBtn.addEventListener('click', () => {
    window.open(`${apiBase}/api/run/${encodeURIComponent(preset.id)}`, '_blank');
  });

  // Edit button
  const editBtn = document.createElement('button');
  editBtn.type = 'button';
  editBtn.textContent = 'Edit';
  editBtn.className = 'secondary-button';
  editBtn.addEventListener('click', () => {
    openEditorForPreset(preset);
  });

  // Duplicate button
  const dupBtn = document.createElement('button');
  dupBtn.type = 'button';
  dupBtn.textContent = 'Duplicate';
  dupBtn.className = 'secondary-button';
  dupBtn.addEventListener('click', async () => {
    try {
      setStatus('Duplicating preset...');
      const resp = await fetch(`${apiBase}/api/duplicate/${encodeURIComponent(preset.id)}`, {
        method: 'POST',
      });
      if (!resp.ok) {
        const err = await resp.json().catch(() => ({}));
        throw new Error(err.error || 'Duplicate failed');
      }
      const newPreset = await resp.json();
      setStatus(`Preset duplicated as "${newPreset.name || newPreset.id}".`);
      await loadPresets();
      renderAllPanes();
    } catch (error) {
      setStatus(error.message, true);
    }
  });

  // Delete button
  const delBtn = document.createElement('button');
  delBtn.type = 'button';
  delBtn.textContent = 'Delete';
  delBtn.className = 'danger-button';
  delBtn.addEventListener('click', async () => {
    if (!window.confirm(`Delete preset "${preset.name || preset.id}"?`)) return;
    try {
      setStatus('Deleting preset...');
      const resp = await fetch(`${apiBase}/api/preset/${encodeURIComponent(preset.id)}`, {
        method: 'DELETE',
      });
      if (!resp.ok && resp.status !== 204) {
        const err = await resp.json().catch(() => ({}));
        throw new Error(err.error || 'Delete failed');
      }
      setStatus('Preset deleted.');
      await loadPresets();
      renderAllPanes();
    } catch (error) {
      setStatus(error.message, true);
    }
  });

  actions.append(runBtn, editBtn, dupBtn, delBtn);

  header.append(titleCol, actions);
  card.appendChild(header);

  // Tags
  if (preset.tags && preset.tags.length) {
    const tags = document.createElement('div');
    tags.className = 'tags';
    preset.tags.forEach((tag) => {
      const span = document.createElement('span');
      span.className = 'tag';
      span.textContent = tag;
      tags.appendChild(span);
    });
    card.appendChild(tags);
  }

  // Engine badge
  if (preset.engine) {
    const metaRow = document.createElement('div');
    metaRow.className = 'meta-row';
    const badge = document.createElement('span');
    badge.className = `engine-badge engine-${preset.engine.toLowerCase().replace(/[^a-z0-9\-]/g, '')}`;
    badge.textContent = preset.engine;
    metaRow.appendChild(badge);
    card.appendChild(metaRow);
  }

  // Description
  if (preset.description) {
    const desc = document.createElement('p');
    desc.className = 'description';
    desc.textContent = preset.description;
    card.appendChild(desc);
  }

  // Command
  if (preset.command) {
    const wrap = document.createElement('div');
    wrap.className = 'command-wrap';

    const pre = document.createElement('pre');
    pre.className = 'command';
    pre.textContent = preset.command;

    const copyBtn = document.createElement('button');
    copyBtn.type = 'button';
    copyBtn.className = 'copy-button';
    copyBtn.title = 'Copy command';
    copyBtn.setAttribute('aria-label', 'Copy command to clipboard');
    copyBtn.innerHTML =
      '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>';

    copyBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      try {
        await navigator.clipboard.writeText(preset.command);
        copyBtn.title = 'Copied!';
        setTimeout(() => {
          copyBtn.title = 'Copy command';
        }, 1500);
      } catch {
        copyBtn.title = 'Copy failed';
      }
    });

    wrap.append(pre, copyBtn);
    card.appendChild(wrap);
  }

  return card;
}

// --------------------------------------------------------------------------
// Status helper
// --------------------------------------------------------------------------
function setStatus(text, isError = false) {
  const el = document.getElementById('status');
  if (!el) return;
  el.textContent = text;
  el.classList.toggle('error', isError);
}

// --------------------------------------------------------------------------
// Data loading
// --------------------------------------------------------------------------
async function loadPresets() {
  try {
    const res = await fetch(`${apiBase}/api/presets`);
    if (!res.ok) throw new Error('Failed to load presets');
    presetsCache = await res.json();
  } catch (error) {
    setStatus(error.message, true);
  }
}

async function loadDecks() {
  try {
    const res = await fetch(`${apiBase}/api/decks`);
    if (!res.ok) throw new Error('Failed to load decks');
    decksCache = await res.json();
  } catch (error) {
    setStatus(error.message, true);
  }
}

async function loadFavourites() {
  try {
    const res = await fetch(`${apiBase}/api/favourites`);
    if (!res.ok) throw new Error('Failed to load favourites');
    const data = await res.json();
    favouritesCache = data.favourites || data;
    if (Array.isArray(data) && !Array.isArray(data.favourites)) {
      favouritesCache = data;
    }
  } catch (error) {
    setStatus(error.message, true);
  }
}

// --------------------------------------------------------------------------
// Pane-scoped filtering
// --------------------------------------------------------------------------
function filterPresetsForPane(pane) {
  let results = presetsCache.slice();

  // Text search
  const query = pane.searchText.toLowerCase().trim();
  if (query) {
    results = results.filter((p) => {
      const name = (p.name || '').toLowerCase();
      const id = (p.id || '').toLowerCase();
      const model = (p.model || '').toLowerCase();
      const desc = (p.description || '').toLowerCase();
      const engine = (p.engine || '').toLowerCase();
      const tags = (p.tags || []).map((t) => t.toLowerCase());
      const command = (p.command || '').toLowerCase();

      return (
        name.includes(query) ||
        id.includes(query) ||
        model.includes(query) ||
        desc.includes(query) ||
        engine.includes(query) ||
        tags.some((t) => t.includes(query)) ||
        command.includes(query)
      );
    });
  }

  // Active tag/engine filters
  for (const filter of pane.activeFilters) {
    if (filter.type === 'tag') {
      const val = filter.value.toLowerCase();
      results = results.filter((p) => (p.tags || []).map((t) => t.toLowerCase()).includes(val));
    } else if (filter.type === 'engine') {
      const val = filter.value.toLowerCase();
      results = results.filter((p) => (p.engine || '').toLowerCase() === val);
    }
  }

  // Deck filter
  if (pane.activeDeckFilter) {
    const deck = decksCache.find((d) => d.name === pane.activeDeckFilter);
    if (deck) {
      results = results.filter((p) => deck.preset_ids.includes(p.id));
    } else {
      results = [];
    }
  }

  return results;
}

// --------------------------------------------------------------------------
// Pane-scoped rendering
// --------------------------------------------------------------------------
function renderPane(pane) {
  const filtered = filterPresetsForPane(pane);

  // Clear and render preset cards
  pane.presetsContainer.innerHTML = '';

  if (filtered.length === 0) {
    const empty = document.createElement('div');
    empty.className = 'empty';
    if (presetsCache.length === 0) {
      empty.textContent = 'No presets found. Create one to get started.';
    } else {
      empty.textContent = 'No presets match the current filters.';
    }
    pane.presetsContainer.appendChild(empty);
  } else {
    filtered.forEach((preset) => {
      const isFav = favouritesCache.includes(preset.id);
      const card = createPresetCard(preset, isFav);
      pane.presetsContainer.appendChild(card);
    });
  }

  // Render filter chips
  renderFilterChips(pane);

  // Update filter count
  updateFilterCount(pane);

  // Update search clear button
  pane.searchClear.hidden = !pane.searchInput.value;
}

function renderFilterChips(pane) {
  pane.filterChipsContainer.innerHTML = '';

  pane.activeFilters.forEach((filter) => {
    const chip = document.createElement('span');
    chip.className = `filter-chip ${filter.type === 'tag' ? 'tag-chip' : 'engine-chip'}`;

    const label = document.createElement('span');
    label.textContent = filter.value;

    const removeBtn = document.createElement('button');
    removeBtn.className = 'filter-chip-remove';
    removeBtn.textContent = '\u00d7';
    removeBtn.addEventListener('click', () => {
      pane.activeFilters = pane.activeFilters.filter((f) => f !== filter);
      renderPane(pane);
    });

    chip.append(label, removeBtn);
    pane.filterChipsContainer.appendChild(chip);
  });

  // Deck filter chip
  if (pane.activeDeckFilter) {
    const chip = document.createElement('span');
    chip.className = 'filter-chip deck-chip';

    const label = document.createElement('span');
    label.textContent = pane.activeDeckFilter;

    const removeBtn = document.createElement('button');
    removeBtn.className = 'filter-chip-remove';
    removeBtn.textContent = '\u00d7';
    removeBtn.addEventListener('click', () => {
      pane.activeDeckFilter = '';
      pane.deckSelector.value = '';
      renderPane(pane);
    });

    chip.append(label, removeBtn);
    pane.filterChipsContainer.appendChild(chip);
  }
}

function updateFilterCount(pane) {
  const filtered = filterPresetsForPane(pane);
  if (pane.searchText || pane.activeFilters.length > 0 || pane.activeDeckFilter) {
    pane.filterCountSpan.textContent = `${filtered.length} of ${presetsCache.length} presets`;
  } else {
    pane.filterCountSpan.textContent = `${presetsCache.length} preset${presetsCache.length === 1 ? '' : 's'}`;
  }
}

// --------------------------------------------------------------------------
// Render all visible panes
// --------------------------------------------------------------------------
function renderAllPanes() {
  if (leftPane) renderPane(leftPane);
  if (rightPane && !rightPane.el.hidden) renderPane(rightPane);
}

// --------------------------------------------------------------------------
// Sync deck selectors
// --------------------------------------------------------------------------
function syncDeckSelectors() {
  [leftPane, rightPane].forEach((pane) => {
    if (!pane) return;
    const selector = pane.deckSelector;
    const currentVal = selector.value;
    selector.innerHTML = '<option value="">All presets</option>';
    decksCache.forEach((deck) => {
      const opt = document.createElement('option');
      opt.value = deck.name;
      opt.textContent = deck.name;
      selector.appendChild(opt);
    });
    // Restore selection if still valid
    const stillValid = [...selector.options].some((o) => o.value === currentVal);
    selector.value = stillValid ? currentVal : '';
  });
}

// --------------------------------------------------------------------------
// Suggestions
// --------------------------------------------------------------------------
function generateSuggestions(pane) {
  const query = pane.searchInput.value.toLowerCase().trim();
  pane.currentSuggestions = [];

  if (!query) {
    hideSuggestions(pane);
    return;
  }

  const tagSet = new Set();
  const engineSet = new Set();

  presetsCache.forEach((p) => {
    (p.tags || []).forEach((t) => {
      if (t.toLowerCase().includes(query)) tagSet.add(t);
    });
    if (p.engine && p.engine.toLowerCase().includes(query)) {
      engineSet.add(p.engine);
    }
  });

  tagSet.forEach((t) => {
    pane.currentSuggestions.push({ type: 'tag', value: t });
  });
  engineSet.forEach((e) => {
    pane.currentSuggestions.push({ type: 'engine', value: e });
  });

  pane.suggestionHighlightIndex = -1;
  renderSuggestions(pane);
}

function renderSuggestions(pane) {
  pane.suggestionsContainer.innerHTML = '';

  if (pane.currentSuggestions.length === 0) {
    hideSuggestions(pane);
    return;
  }

  pane.currentSuggestions.forEach((sug, idx) => {
    const item = document.createElement('div');
    item.className = 'suggestion-item';
    if (idx === pane.suggestionHighlightIndex) {
      item.classList.add('highlighted');
    }

    const typeBadge = document.createElement('span');
    typeBadge.className = `suggestion-type ${sug.type}`;
    typeBadge.textContent = sug.type;

    const label = document.createElement('span');
    label.className = 'suggestion-label';
    label.textContent = sug.value;

    item.append(typeBadge, label);

    item.addEventListener('click', () => {
      applySuggestionFilter(pane, sug);
    });

    pane.suggestionsContainer.appendChild(item);
  });

  pane.suggestionsContainer.classList.add('visible');
}

function hideSuggestions(pane) {
  pane.suggestionsContainer.classList.remove('visible');
  pane.suggestionHighlightIndex = -1;
}

function applySuggestionFilter(pane, suggestion) {
  // Check if filter already exists
  const exists = pane.activeFilters.some(
    (f) => f.type === suggestion.type && f.value.toLowerCase() === suggestion.value.toLowerCase()
  );
  if (!exists) {
    pane.activeFilters.push({ type: suggestion.type, value: suggestion.value });
  }
  pane.searchInput.value = '';
  pane.searchClear.hidden = true;
  hideSuggestions(pane);
  renderPane(pane);
}

// --------------------------------------------------------------------------
// Wire pane events
// --------------------------------------------------------------------------
function wirePaneEvents(pane) {
  // Search input
  let searchTimeout;
  pane.searchInput.addEventListener('input', () => {
    clearTimeout(searchTimeout);
    pane.searchText = pane.searchInput.value;
    searchTimeout = setTimeout(() => {
      generateSuggestions(pane);
      renderPane(pane);
    }, 150);
  });

  // Search clear
  pane.searchClear.addEventListener('click', () => {
    pane.searchInput.value = '';
    pane.searchText = '';
    pane.searchClear.hidden = true;
    hideSuggestions(pane);
    renderPane(pane);
    pane.searchInput.focus();
  });

  // Keyboard navigation for suggestions
  pane.searchInput.addEventListener('keydown', (e) => {
    if (!pane.suggestionsContainer.classList.contains('visible')) return;
    const count = pane.currentSuggestions.length;

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      pane.suggestionHighlightIndex = Math.min(pane.suggestionHighlightIndex + 1, count - 1);
      renderSuggestions(pane);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      pane.suggestionHighlightIndex = Math.max(pane.suggestionHighlightIndex - 1, 0);
      renderSuggestions(pane);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (pane.suggestionHighlightIndex >= 0 && pane.currentSuggestions[pane.suggestionHighlightIndex]) {
        applySuggestionFilter(pane, pane.currentSuggestions[pane.suggestionHighlightIndex]);
      }
    } else if (e.key === 'Escape') {
      hideSuggestions(pane);
    }
  });

  // Deck selector
  pane.deckSelector.addEventListener('change', () => {
    pane.activeDeckFilter = pane.deckSelector.value;
    renderPane(pane);
  });
}

// --------------------------------------------------------------------------
// View mode toggle
// --------------------------------------------------------------------------
function setViewMode(mode) {
  viewMode = mode;
  const container = document.getElementById('split-container');
  const rightEl = document.getElementById('pane-right');
  const toggleBtn = document.getElementById('view-toggle');

  if (mode === 'dual') {
    container.classList.remove('single-mode');
    container.classList.add('dual-mode');
    rightEl.hidden = false;
    toggleBtn.querySelector('span').textContent = 'Single View';

    // Clone left state into right when entering dual mode
    if (rightPane) {
      rightPane.searchText = leftPane.searchText;
      rightPane.activeFilters = leftPane.activeFilters.slice();
      rightPane.activeFilters = rightPane.activeFilters.map((f) => ({ ...f }));
      rightPane.activeDeckFilter = leftPane.activeDeckFilter;
      rightPane.searchInput.value = leftPane.searchInput.value;
      rightPane.searchClear.hidden = leftPane.searchClear.hidden;
      rightPane.deckSelector.value = leftPane.deckSelector.value;
      renderPane(rightPane);
    }
  } else {
    // Single mode: keep left pane state, hide right
    container.classList.remove('dual-mode');
    container.classList.add('single-mode');
    rightEl.hidden = true;
    toggleBtn.querySelector('span').textContent = 'Dual View';
  }
}

// --------------------------------------------------------------------------
// Editor dialog
// --------------------------------------------------------------------------
const presetDialog = document.getElementById('preset-dialog');
const presetEditor = document.getElementById('preset-editor');
const presetDialogTitle = document.getElementById('preset-dialog-title');
const presetDialogSubtitle = document.getElementById('preset-dialog-subtitle');
const presetName = document.getElementById('preset-name');
const presetEngine = document.getElementById('preset-engine');
const presetModel = document.getElementById('preset-model');
const presetTags = document.getElementById('preset-tags');
const presetDescription = document.getElementById('preset-description');
const presetCommand = document.getElementById('preset-command');
const presetEditorFeedback = document.getElementById('preset-editor-feedback');
const editorSave = document.getElementById('editor-save');
const editorCancel = document.getElementById('editor-cancel');
const editorClose = document.getElementById('editor-close');
const editorRun = document.getElementById('editor-run');
const copyCommandBtn = document.getElementById('copy-command-btn');

let editingPresetId = null;

function openEditorForPreset(preset) {
  editingPresetId = preset.id;
  presetDialogTitle.textContent = 'Edit preset';
  presetDialogSubtitle.textContent = `Editing: ${preset.name || preset.id}`;
  presetName.value = preset.name || '';
  presetEngine.value = preset.engine || '';
  presetModel.value = preset.model || '';
  presetTags.value = (preset.tags || []).join(', ');
  presetDescription.value = preset.description || '';
  presetCommand.value = preset.command || '';
  presetEditorFeedback.textContent = '';
  presetEditorFeedback.classList.remove('error');
  editorSave.textContent = 'Save changes';
  presetDialog.showModal();
}

function openCreateEditor() {
  editingPresetId = null;
  presetDialogTitle.textContent = 'Create preset';
  presetDialogSubtitle.textContent = 'Fill out the fields and save the preset JSON.';
  presetName.value = '';
  presetEngine.value = '';
  presetModel.value = '';
  presetTags.value = '';
  presetDescription.value = '';
  presetCommand.value = '';
  presetEditorFeedback.textContent = '';
  presetEditorFeedback.classList.remove('error');
  editorSave.textContent = 'Save preset';
  presetDialog.showModal();
}

function closeEditor() {
  presetDialog.close();
  editingPresetId = null;
}

presetEditor.addEventListener('submit', async (e) => {
  e.preventDefault();

  const tagsText = presetTags.value;
  const tags = tagsText
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean);

  const payload = {
    name: presetName.value.trim(),
    engine: presetEngine.value.trim(),
    model: presetModel.value.trim(),
    tags,
    description: presetDescription.value.trim(),
    command: presetCommand.value.trim(),
  };

  if (!payload.name && !payload.command) {
    presetEditorFeedback.classList.add('error');
    presetEditorFeedback.textContent = 'At least a name or command is required.';
    return;
  }

  editorSave.disabled = true;
  presetEditorFeedback.classList.remove('error');
  presetEditorFeedback.textContent = 'Saving...';

  try {
    let resp;
    if (editingPresetId) {
      resp = await fetch(`${apiBase}/api/preset/${encodeURIComponent(editingPresetId)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
    } else {
      resp = await fetch(`${apiBase}/api/preset`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
    }

    if (!resp.ok) {
      const err = await resp.json().catch(() => ({}));
      throw new Error(err.error || 'Save failed');
    }

    presetEditorFeedback.textContent = editingPresetId ? 'Preset updated.' : 'Preset created.';
    await loadPresets();
    await loadFavourites();
    renderAllPanes();
    setTimeout(closeEditor, 600);
  } catch (error) {
    presetEditorFeedback.classList.add('error');
    presetEditorFeedback.textContent = error.message || 'Save failed.';
  } finally {
    editorSave.disabled = false;
  }
});

editorRun.addEventListener('click', async () => {
  const command = presetCommand.value.trim();
  if (!command) {
    presetEditorFeedback.classList.add('error');
    presetEditorFeedback.textContent = 'Command is required to run.';
    return;
  }

  editorRun.disabled = true;
  presetEditorFeedback.classList.remove('error');
  presetEditorFeedback.textContent = 'Launching...';

  try {
    const resp = await fetch(`${apiBase}/api/run-command`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ command }),
    });

    if (!resp.ok) {
      const err = await resp.json().catch(() => ({}));
      throw new Error(err.error || 'Run failed');
    }

    presetEditorFeedback.textContent = 'Command launched.';
  } catch (error) {
    presetEditorFeedback.classList.add('error');
    presetEditorFeedback.textContent = error.message || 'Run failed.';
  } finally {
    editorRun.disabled = false;
  }
});

copyCommandBtn.addEventListener('click', async () => {
  const command = presetCommand.value;
  if (!command) return;
  try {
    await navigator.clipboard.writeText(command);
    copyCommandBtn.title = 'Copied!';
    setTimeout(() => {
      copyCommandBtn.title = 'Copy command';
    }, 1500);
  } catch {
    copyCommandBtn.title = 'Copy failed';
  }
});

editorCancel.addEventListener('click', closeEditor);
editorClose.addEventListener('click', closeEditor);

// --------------------------------------------------------------------------
// Deck dialog
// --------------------------------------------------------------------------
const decksDialog = document.getElementById('decks-dialog');
const decksListEl = document.getElementById('decks-list');
const deckEditorEmpty = document.getElementById('deck-editor-empty');
const deckEditorForm = document.getElementById('deck-editor-form');
const deckNameInput = document.getElementById('deck-name-input');
const deckPresetsSearch = document.getElementById('deck-presets-search');
const deckPresetsList = document.getElementById('deck-presets-list');
const deckNewButton = document.getElementById('deck-new-button');
const deckSaveButton = document.getElementById('deck-save-button');
const deckDeleteButton = document.getElementById('deck-delete-button');
const deckFeedback = document.getElementById('deck-feedback');

let editingDeckName = null;

function showDecksDialog() {
  editingDeckName = null;
  deckEditorEmpty.hidden = false;
  deckEditorForm.hidden = true;
  renderDecksList();
  decksDialog.showModal();
}

function closeDecksDialog() {
  decksDialog.close();
  editingDeckName = null;
}

function showNewDeckUI() {
  editingDeckName = null;
  deckNameInput.value = '';
  deckPresetsSearch.value = '';
  deckEditorEmpty.hidden = true;
  deckEditorForm.hidden = false;
  deckFeedback.textContent = '';
  deckFeedback.classList.remove('error');

  // Show all presets unchecked
  renderDeckPresetsList([]);
  renderDecksList();
}

function renderDecksList() {
  decksListEl.innerHTML = '';

  if (decksCache.length === 0) {
    const empty = document.createElement('div');
    empty.className = 'empty';
    empty.textContent = 'No decks created yet.';
    decksListEl.appendChild(empty);
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
    decksListEl.appendChild(item);
  });
}

function selectDeckForEditing(deckName) {
  editingDeckName = deckName;
  const deck = decksCache.find((d) => d.name === deckName);
  if (!deck) return;

  deckNameInput.value = deck.name;
  deckEditorEmpty.hidden = true;
  deckEditorForm.hidden = false;
  deckFeedback.textContent = '';
  deckFeedback.classList.remove('error');

  renderDeckPresetsList(deck.preset_ids);
  renderDecksList(); // update selection highlight
}

function renderDeckPresetsList(selectedIds) {
  const searchQuery = deckPresetsSearch.value.toLowerCase().trim();
  deckPresetsList.innerHTML = '';

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
    deckPresetsList.appendChild(empty);
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
    deckPresetsList.appendChild(item);
  });
}

deckPresetsSearch.addEventListener('input', () => {
  if (editingDeckName) {
    const deck = decksCache.find((d) => d.name === editingDeckName);
    if (deck) renderDeckPresetsList(deck.preset_ids);
  }
});

deckNewButton.addEventListener('click', () => {
  showNewDeckUI();
});

deckSaveButton.addEventListener('click', async () => {
  const name = deckNameInput.value.trim();
  if (!name) {
    deckFeedback.classList.add('error');
    deckFeedback.textContent = 'Deck name is required.';
    return;
  }

  // Gather selected preset IDs
  const checkboxes = deckPresetsList.querySelectorAll('.deck-preset-item input[type="checkbox"]');
  const presetIds = [];
  checkboxes.forEach((cb) => {
    if (cb.checked) {
      // Find the preset id from the label text
      const labelText = cb.parentElement.querySelector('span').textContent;
      const preset = presetsCache.find((p) => (p.name || p.id) === labelText);
      if (preset) presetIds.push(preset.id);
    }
  });

  deckSaveButton.disabled = true;
  deckFeedback.classList.remove('error');
  deckFeedback.textContent = 'Saving...';

  try {
    if (editingDeckName) {
      // Update existing deck
      const response = await fetch(`${apiBase}/api/decks/${encodeURIComponent(editingDeckName)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, preset_ids: presetIds }),
      });
      if (!response.ok) {
        const err = await response.json().catch(() => ({}));
        throw new Error(err.error || 'Failed to update deck');
      }
    } else {
      // Create new deck
      const response = await fetch(`${apiBase}/api/decks`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, preset_ids: presetIds }),
      });
      if (!response.ok) {
        const err = await response.json().catch(() => ({}));
        throw new Error(err.error || 'Failed to create deck');
      }
      editingDeckName = name;
    }

    deckFeedback.textContent = 'Deck saved.';
    await loadDecks();
    syncDeckSelectors();
    renderDecksList();
    selectDeckForEditing(name);
    renderAllPanes();
  } catch (error) {
    deckFeedback.classList.add('error');
    deckFeedback.textContent = error.message || 'Save failed.';
  } finally {
    deckSaveButton.disabled = false;
  }
});

deckDeleteButton.addEventListener('click', async () => {
  if (!editingDeckName) return;
  if (!window.confirm(`Delete deck "${editingDeckName}"?`)) return;

  deckDeleteButton.disabled = true;
  deckFeedback.classList.remove('error');
  deckFeedback.textContent = 'Deleting...';

  try {
    const response = await fetch(`${apiBase}/api/decks/${encodeURIComponent(editingDeckName)}`, { method: 'DELETE' });
    if (!response.ok && response.status !== 204) {
      const err = await response.json().catch(() => ({}));
      throw new Error(err.error || 'Failed to delete deck');
    }

    deckFeedback.textContent = 'Deck deleted.';

    // Clear deck filter on all panes that had this deck selected
    [leftPane, rightPane].forEach((pane) => {
      if (pane && pane.activeDeckFilter === editingDeckName) {
        pane.activeDeckFilter = '';
        pane.deckSelector.value = '';
      }
    });

    await loadDecks();
    syncDeckSelectors();
    deckEditorEmpty.hidden = true;
    deckEditorForm.hidden = true;
    editingDeckName = '';
    renderDecksList();
    renderAllPanes();
  } catch (error) {
    deckFeedback.classList.add('error');
    deckFeedback.textContent = error.message || 'Delete failed.';
  } finally {
    deckDeleteButton.disabled = false;
  }
});

// --------------------------------------------------------------------------
// Global button handlers
// --------------------------------------------------------------------------
document.getElementById('create-button').addEventListener('click', openCreateEditor);
document.getElementById('decks-button').addEventListener('click', showDecksDialog);
document.getElementById('decks-close').addEventListener('click', closeDecksDialog);

document.getElementById('shutdown-button').addEventListener('click', async () => {
  if (!window.confirm('Shut down the PresetDock server?')) return;
  try {
    const resp = await fetch(`${apiBase}/api/shutdown`, { method: 'POST' });
    if (!resp.ok) {
      const err = await resp.json().catch(() => ({}));
      throw new Error(err.error || 'Shutdown failed');
    }
    setStatus('Server shutting down...');
  } catch (error) {
    setStatus(error.message, true);
  }
});

// View toggle button
document.getElementById('view-toggle').addEventListener('click', () => {
  setViewMode(viewMode === 'single' ? 'dual' : 'single');
});

// --------------------------------------------------------------------------
// Close suggestions when clicking outside
// --------------------------------------------------------------------------
document.addEventListener('click', (e) => {
  [leftPane, rightPane].forEach((pane) => {
    if (!pane) return;
    if (!pane.el.contains(e.target)) {
      hideSuggestions(pane);
    }
  });
});

// --------------------------------------------------------------------------
// Initialization
// --------------------------------------------------------------------------
async function init() {
  // Create pane states
  leftPane = createPaneState(document.getElementById('pane-left'));
  rightPane = createPaneState(document.getElementById('pane-right'));

  // Wire events for both panes
  wirePaneEvents(leftPane);
  wirePaneEvents(rightPane);

  // Load data
  await loadPresets();
  await loadDecks();
  await loadFavourites();

  // Sync deck selectors
  syncDeckSelectors();

  // Initial render
  renderAllPanes();

  // Start heartbeat
  startHeartbeat();

  // Load server info if element exists
  loadServerInfo();
}

init();