const presetsEl = document.getElementById('presets');
const statusEl = document.getElementById('status');
const createButton = document.getElementById('create-button');
const shutdownButton = document.getElementById('shutdown-button');
const dialog = document.getElementById('preset-dialog');
const editorForm = document.getElementById('preset-editor');
const dialogTitle = document.getElementById('preset-dialog-title');
const dialogSubtitle = document.getElementById('preset-dialog-subtitle');
const editorFeedback = document.getElementById('preset-editor-feedback');
const editorCloseButton = document.getElementById('editor-close');
const editorCancelButton = document.getElementById('editor-cancel');
const editorSaveButton = document.getElementById('editor-save');
const editorRunButton = document.getElementById('editor-run');
const nameInput = document.getElementById('preset-name');
const engineInput = document.getElementById('preset-engine');
const modelInput = document.getElementById('preset-model');
const tagsInput = document.getElementById('preset-tags');
const descriptionInput = document.getElementById('preset-description');
const commandInput = document.getElementById('preset-command');
const copyCommandBtn = document.getElementById('copy-command-btn');

copyCommandBtn.addEventListener('click', async () => {
  const text = commandInput.value.trim();
  if (!text) return;
  await copyToClipboard(text);
  const originalHTML = copyCommandBtn.innerHTML;
  copyCommandBtn.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>';
  setTimeout(() => {
    copyCommandBtn.innerHTML = originalHTML;
  }, 1500);
});

// Search / filter elements
const searchInput = document.getElementById('search-input');
const searchClear = document.getElementById('search-clear');
const searchSuggestions = document.getElementById('search-suggestions');
const filterChipsEl = document.getElementById('filter-chips');
const filterCountEl = document.getElementById('filter-count');

let activePresetId = '';
let presetsCache = [];
let selectedEngine = '';

// Active filters: array of { type: 'tag' | 'engine', value: string }
let activeFilters = [];
let suggestionHighlightIndex = -1;
let currentSuggestions = [];

// Favourites
let favouritesSet = new Set();

// Decks
let decksCache = [];
let activeDeckFilter = ''; // deck name currently filtering
let editingDeckName = '';  // original name when editing a deck

// --- Search / Filter Logic ---

function gatherAllTagsEngines(presets) {
  const tags = new Set();
  const engines = new Set();
  (presets || []).forEach((p) => {
    (p.tags || []).forEach((t) => tags.add(t.toLowerCase()));
    if (p.engine) engines.add(p.engine.toLowerCase());
  });
  return { tags: [...tags].sort(), engines: [...engines].sort() };
}

function presetsMatchFilters(presets) {
  if (activeFilters.length === 0) return presets;
  return presets.filter((preset) => {
    const pTags = (preset.tags || []).map((t) => t.toLowerCase());
    const pEngine = (preset.engine || '').toLowerCase();
    return activeFilters.every((f) => {
      if (f.type === 'tag') return pTags.includes(f.value.toLowerCase());
      if (f.type === 'engine') return pEngine === f.value.toLowerCase();
      return false;
    });
  });
}

function presetsMatchText(presets, text) {
  if (!text) return presets;
  const q = text.toLowerCase().trim();
  if (!q) return presets;
  return presets.filter((p) => {
    const name = (p.name || '').toLowerCase();
    const engine = (p.engine || '').toLowerCase();
    const tags = (p.tags || []).map((t) => t.toLowerCase());
    const model = (p.model || '').toLowerCase();
    const desc = (p.description || '').toLowerCase();
    if (name.includes(q) || engine.includes(q) || model.includes(q) || desc.includes(q)) return true;
    if (tags.some((t) => t.includes(q))) return true;
    return false;
  });
}

function applyFiltersAndRender() {
  let filtered = presetsMatchFilters(presetsCache);
  const searchText = searchInput.value.trim();
  filtered = presetsMatchText(filtered, searchText);

  // Apply deck filter
  if (activeDeckFilter) {
    const deck = decksCache.find((d) => d.name === activeDeckFilter);
    if (deck) {
      const deckIds = new Set(deck.preset_ids);
      filtered = filtered.filter((p) => deckIds.has(p.id));
    } else {
      filtered = [];
    }
  }

  renderPresets(filtered);

  // Update clear button visibility
  searchClear.hidden = !searchInput.value;

  // Update filter count
  const total = presetsCache.length;
  const shown = filtered.length;
  const hasFilters = activeFilters.length > 0 || searchText || activeDeckFilter;
  if (hasFilters) {
    filterCountEl.textContent = `Showing ${shown} of ${total} preset${total === 1 ? '' : 's'}`;
  } else {
    filterCountEl.textContent = '';
  }

  // Update status
  if (hasFilters) {
    setStatus(`Showing ${shown} of ${total}.`);
  } else {
    setStatus(`Loaded ${total} preset${total === 1 ? '' : 's'}.`);
  }

  // Update deck filter chip
  renderDeckFilterChip();
}

function renderDeckFilterChip() {
  // Remove existing deck chip if any
  const existing = filterChipsEl.querySelector('.deck-chip');
  if (existing) existing.remove();

  if (!activeDeckFilter) return;

  const chip = document.createElement('span');
  chip.className = 'filter-chip deck-chip';
  chip.textContent = `🗂 ${activeDeckFilter}`;

  const removeBtn = document.createElement('button');
  removeBtn.type = 'button';
  removeBtn.className = 'filter-chip-remove';
  removeBtn.setAttribute('aria-label', 'Remove deck filter');
  removeBtn.textContent = '\u00d7';
  removeBtn.addEventListener('click', () => {
    activeDeckFilter = '';
    deckSelector.value = '';
    applyFiltersAndRender();
  });

  chip.appendChild(removeBtn);
  filterChipsEl.appendChild(chip);
}

function renderFilterChips() {
  // Preserve deck chip
  const deckChip = filterChipsEl.querySelector('.deck-chip');
  filterChipsEl.innerHTML = '';
  if (deckChip) filterChipsEl.appendChild(deckChip);

  activeFilters.forEach((f, idx) => {
    const chip = document.createElement('span');
    chip.className = `filter-chip ${f.type}-chip`;
    chip.textContent = f.type === 'tag' ? `#${f.value}` : f.value;

    const removeBtn = document.createElement('button');
    removeBtn.type = 'button';
    removeBtn.className = 'filter-chip-remove';
    removeBtn.setAttribute('aria-label', `Remove ${f.value} filter`);
    removeBtn.textContent = '\u00d7';
    removeBtn.addEventListener('click', () => {
      activeFilters.splice(idx, 1);
      applyFiltersAndRender();
      renderFilterChips();
      updateSuggestions();
    });

    chip.appendChild(removeBtn);
    filterChipsEl.appendChild(chip);
  });
}

function addFilter(type, value) {
  const normalized = value.toLowerCase().trim();
  if (!normalized) return;
  // Check if already active
  if (activeFilters.some((f) => f.type === type && f.value.toLowerCase() === normalized)) return;
  activeFilters.push({ type, value: normalized });
  applyFiltersAndRender();
  renderFilterChips();
  // Clear search input and suggestions when a filter is added
  searchInput.value = '';
  searchClear.hidden = true;
  hideSuggestions();
}

function getSuggestions(query) {
  const q = query.toLowerCase().trim();
  if (!q) return [];
  const { tags, engines } = gatherAllTagsEngines(presetsCache);
  const results = [];

  // Only suggest tags/engines that aren't already active filters
  const activeTagValues = new Set(activeFilters.filter((f) => f.type === 'tag').map((f) => f.value.toLowerCase()));
  const activeEngineValues = new Set(activeFilters.filter((f) => f.type === 'engine').map((f) => f.value.toLowerCase()));

  tags.forEach((t) => {
    if (!activeTagValues.has(t) && t.includes(q)) {
      results.push({ type: 'tag', value: t });
    }
  });
  engines.forEach((e) => {
    if (!activeEngineValues.has(e) && e.includes(q)) {
      results.push({ type: 'engine', value: e });
    }
  });

  // Limit to 10 suggestions
  return results.slice(0, 10);
}

function renderSuggestions() {
  const query = searchInput.value.trim();
  currentSuggestions = query ? getSuggestions(query) : [];
  suggestionHighlightIndex = -1;

  searchSuggestions.innerHTML = '';
  if (currentSuggestions.length === 0) {
    hideSuggestions();
    return;
  }

  currentSuggestions.forEach((s, idx) => {
    const item = document.createElement('div');
    item.className = 'suggestion-item';
    item.setAttribute('role', 'option');

    const typeBadge = document.createElement('span');
    typeBadge.className = `suggestion-type ${s.type}`;
    typeBadge.textContent = s.type;

    const label = document.createElement('span');
    label.className = 'suggestion-label';
    // Highlight matching portion
    const q = query.toLowerCase();
    const val = s.value;
    const idx_pos = val.indexOf(q);
    if (idx_pos >= 0) {
      label.textContent = val;
    } else {
      label.textContent = val;
    }

    item.append(typeBadge, label);
    item.addEventListener('click', () => {
      addFilter(s.type, s.value);
    });
    searchSuggestions.appendChild(item);
  });

  searchSuggestions.classList.add('visible');
}

function hideSuggestions() {
  searchSuggestions.classList.remove('visible');
  currentSuggestions = [];
  suggestionHighlightIndex = -1;
}

function updateSuggestions() {
  renderSuggestions();
}

// Search input events
let searchDebounceTimer;
searchInput.addEventListener('input', () => {
  searchClear.hidden = !searchInput.value;
  clearTimeout(searchDebounceTimer);
  searchDebounceTimer = setTimeout(() => {
    applyFiltersAndRender();
    updateSuggestions();
  }, 120);
});

searchInput.addEventListener('focus', () => {
  updateSuggestions();
});

searchInput.addEventListener('keydown', (e) => {
  const items = searchSuggestions.querySelectorAll('.suggestion-item');
  if (e.key === 'ArrowDown') {
    e.preventDefault();
    suggestionHighlightIndex = Math.min(suggestionHighlightIndex + 1, items.length - 1);
    items.forEach((it, i) => it.classList.toggle('highlighted', i === suggestionHighlightIndex));
    if (items[suggestionHighlightIndex]) items[suggestionHighlightIndex].scrollIntoView({ block: 'nearest' });
  } else if (e.key === 'ArrowUp') {
    e.preventDefault();
    suggestionHighlightIndex = Math.max(suggestionHighlightIndex - 1, 0);
    items.forEach((it, i) => it.classList.toggle('highlighted', i === suggestionHighlightIndex));
    if (items[suggestionHighlightIndex]) items[suggestionHighlightIndex].scrollIntoView({ block: 'nearest' });
  } else if (e.key === 'Enter') {
    if (suggestionHighlightIndex >= 0 && suggestionHighlightIndex < currentSuggestions.length) {
      e.preventDefault();
      const s = currentSuggestions[suggestionHighlightIndex];
      addFilter(s.type, s.value);
    } else {
      // Just apply the text filter
      hideSuggestions();
      applyFiltersAndRender();
    }
  } else if (e.key === 'Escape') {
    hideSuggestions();
    searchInput.blur();
  }
});

searchClear.addEventListener('click', () => {
  searchInput.value = '';
  searchClear.hidden = true;
  hideSuggestions();
  applyFiltersAndRender();
  searchInput.focus();
});

// Close suggestions when clicking outside
document.addEventListener('click', (e) => {
  if (!e.target.closest('.search-bar')) {
    hideSuggestions();
  }
});

function setStatus(message, isError = false) {
  statusEl.textContent = message;
  statusEl.style.color = isError ? 'var(--danger)' : 'var(--muted)';
}

function startHeartbeat() {
  const sendHeartbeat = () => {
    fetch('/api/heartbeat', {
      method: 'POST',
      keepalive: true,
    }).catch(() => {});
  };

  sendHeartbeat();
  window.setInterval(sendHeartbeat, 30000);
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) {
      sendHeartbeat();
    }
  });
  window.addEventListener('focus', sendHeartbeat);
  window.addEventListener('pagehide', sendHeartbeat);
}

function slugify(value) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .replace(/-+/g, '-');
}

function tagsToText(tags) {
  return (tags || []).join(', ');
}

function textToTags(text) {
  return text
    .split(/[\n,]+/)
    .map((value) => value.trim())
    .filter(Boolean);
}

function copyToClipboard(text) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    return navigator.clipboard.writeText(text).catch(() => {});
  }
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  document.body.removeChild(textarea);
  return Promise.resolve();
}

function createCopyButton(commandText) {
  const btn = document.createElement('button');
  btn.type = 'button';
  btn.className = 'copy-button';
  btn.setAttribute('aria-label', 'Copy command to clipboard');
  btn.setAttribute('title', 'Copy command');
  btn.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>';

  btn.addEventListener('click', async () => {
    if (!commandText) return;
    await copyToClipboard(commandText);
    const originalHTML = btn.innerHTML;
    btn.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>';
    setTimeout(() => {
      btn.innerHTML = originalHTML;
    }, 1500);
  });

  return btn;
}

function createTag(text) {
  const tag = document.createElement('span');
  tag.className = 'tag';
  tag.textContent = text;
  return tag;
}

function createEngineBadge(engine) {
  const badge = document.createElement('span');
  const normalizedEngine = (engine || 'unknown').toLowerCase();
  badge.className = `engine-badge engine-${slugify(normalizedEngine) || 'unknown'}`;
  badge.textContent = normalizedEngine;
  return badge;
}

function renderEmpty(message) {
  presetsEl.innerHTML = '';
  const empty = document.createElement('div');
  empty.className = 'empty';
  empty.textContent = message;
  presetsEl.appendChild(empty);
}

function readEditorValues() {
  return {
    name: nameInput.value.trim(),
    engine: engineInput.value.trim(),
    model: modelInput.value.trim(),
    tags: textToTags(tagsInput.value),
    description: descriptionInput.value.trim(),
    command: commandInput.value.trim(),
  };
}

function fillEditor(preset) {
  nameInput.value = preset.name || '';
  engineInput.value = preset.engine || '';
  modelInput.value = preset.model || '';
  tagsInput.value = tagsToText(preset.tags);
  descriptionInput.value = preset.description || '';
  commandInput.value = preset.command || '';
}

function openEditor(preset) {
  const isExisting = Boolean(preset && preset.id);
  activePresetId = isExisting ? preset.id : '';
  selectedEngine = preset && preset.engine ? preset.engine : '';

  dialogTitle.textContent = isExisting ? `Edit ${preset.name || preset.id}` : 'Create preset';
  dialogSubtitle.textContent = isExisting
    ? 'Update the preset fields and save the JSON file.'
    : 'Fill out the fields and create a new preset JSON file.';
  editorSaveButton.textContent = isExisting ? 'Save preset' : 'Create preset';

  fillEditor(preset || { name: '', engine: '', model: '', tags: [], description: '', command: '' });
  editorFeedback.classList.remove('error');
  editorFeedback.textContent = '';

  if (typeof dialog.showModal === 'function') {
    dialog.showModal();
  } else {
    dialog.setAttribute('open', '');
  }

  nameInput.focus();
}

function closeEditor() {
  if (dialog.open) {
    dialog.close();
  }
  editorFeedback.classList.remove('error');
  editorFeedback.textContent = '';
}

function openDuplicateEditor(preset) {
  // Clear activePresetId so it creates a NEW preset, not edits the existing one
  activePresetId = '';
  selectedEngine = preset.engine || '';
  // Store the source preset ID so the backend can propagate deck/favourite membership
  window._duplicateSourcePresetId = preset.id;

  dialogTitle.textContent = `Duplicate ${preset.name || preset.id}`;
  dialogSubtitle.textContent = 'Modify the fields and save as a new preset.';
  editorSaveButton.textContent = 'Create preset';

  // Pre-fill with copied values, appending " (copy)" to the name
  nameInput.value = (preset.name || '') + ' (copy)';
  engineInput.value = preset.engine || '';
  modelInput.value = preset.model || '';
  tagsInput.value = tagsToText(preset.tags);
  descriptionInput.value = preset.description || '';
  commandInput.value = preset.command || '';

  editorFeedback.classList.remove('error');
  editorFeedback.textContent = '';

  if (typeof dialog.showModal === 'function') {
    dialog.showModal();
  } else {
    dialog.setAttribute('open', '');
  }

  nameInput.focus();
  nameInput.select();
}

function createPresetCard(preset, index) {
  const card = document.createElement('article');
  card.className = 'card';
  card.style.animationDelay = `${Math.min(index * 70, 350)}ms`;

  const header = document.createElement('div');
  header.className = 'card-header';

  const titleWrap = document.createElement('div');

  // Favourite star button
  const starBtn = document.createElement('button');
  starBtn.type = 'button';
  starBtn.className = 'favourite-star';
  starBtn.setAttribute('title', preset.is_favourite ? 'Remove from favourites' : 'Add to favourites');
  starBtn.setAttribute('aria-label', starBtn.getAttribute('title'));
  starBtn.textContent = preset.is_favourite ? '★' : '☆';
  if (preset.is_favourite) starBtn.classList.add('active');

  starBtn.addEventListener('click', async (e) => {
    e.stopPropagation();
    const isNewFav = !starBtn.classList.contains('active');
    if (isNewFav) {
      await fetch('/api/favourites', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ preset_id: preset.id }),
      });
    } else {
      await fetch(`/api/favourites/${encodeURIComponent(preset.id)}`, { method: 'DELETE' });
    }
    // Reload presets to get updated ordering
    await loadPresets();
  });

   const title = document.createElement('h2');
   title.textContent = preset.name || preset.id;

   // Wrap star and title in the same inline row
   const titleRow = document.createElement('div');
   titleRow.className = 'card-title-row';
   titleRow.append(starBtn, title);

   const metaRow = document.createElement('div');
   metaRow.className = 'meta-row';

   const model = document.createElement('p');
   model.className = 'model';
   model.textContent = preset.model || 'No model specified';

   metaRow.append(model, createEngineBadge(preset.engine));
   titleWrap.append(titleRow, metaRow);

  const actionButtons = document.createElement('div');
  actionButtons.className = 'action-buttons';

  const runButton = document.createElement('button');
  runButton.type = 'button';
  runButton.textContent = 'Run';

  const editButton = document.createElement('button');
  editButton.type = 'button';
  editButton.textContent = 'Edit';
  editButton.className = 'secondary-button';

  const duplicateButton = document.createElement('button');
  duplicateButton.type = 'button';
  duplicateButton.className = 'subtle-button';
  duplicateButton.setAttribute('title', 'Duplicate preset');
  duplicateButton.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>';

  const deleteButton = document.createElement('button');
  deleteButton.type = 'button';
  deleteButton.textContent = 'Delete';
  deleteButton.className = 'danger-button';

  duplicateButton.addEventListener('click', () => {
    openDuplicateEditor(preset);
  });

  actionButtons.append(runButton, editButton, duplicateButton, deleteButton);
  header.append(titleWrap, actionButtons);

  const description = document.createElement('p');
  description.className = 'description';
  description.textContent = preset.description || 'No description provided.';

  const tags = document.createElement('div');
  tags.className = 'tags';
  (preset.tags || []).forEach((tagText) => {
    tags.appendChild(createTag(tagText));
  });

  const commandWrap = document.createElement('div');
  commandWrap.className = 'command-wrap';
  const command = document.createElement('pre');
  command.className = 'command';
  command.textContent = preset.command || '';
  const copyBtn = createCopyButton(preset.command || '');
  commandWrap.append(command, copyBtn);

  const feedback = document.createElement('span');
  feedback.className = 'feedback';

  runButton.addEventListener('click', async () => {
    const originalLabel = runButton.textContent;
    runButton.disabled = true;
    feedback.classList.remove('error');
    feedback.textContent = 'Launching...';

    try {
      const response = await fetch(`/api/run/${encodeURIComponent(preset.id)}`, {
        method: 'POST',
      });

      if (!response.ok) {
        const payload = await response.json().catch(() => ({}));
        throw new Error(payload.error || `Request failed with status ${response.status}`);
      }

      feedback.textContent = 'Launched in a new terminal.';
      runButton.textContent = 'Launched';
    } catch (error) {
      feedback.classList.add('error');
      feedback.textContent = error instanceof Error ? error.message : 'Launch failed.';
      runButton.textContent = originalLabel;
    } finally {
      runButton.disabled = false;
      window.setTimeout(() => {
        if (runButton.textContent === 'Launched') {
          runButton.textContent = originalLabel;
        }
      }, 1400);
    }
  });

  editButton.addEventListener('click', () => {
    openEditor(preset);
  });

  deleteButton.addEventListener('click', async () => {
    if (!window.confirm(`Delete preset ${preset.name || preset.id}?`)) {
      return;
    }

    const originalLabel = deleteButton.textContent;
    deleteButton.disabled = true;
    deleteButton.textContent = 'Deleting...';
    feedback.classList.remove('error');
    feedback.textContent = '';

    try {
      const response = await fetch(`/api/presets/${encodeURIComponent(preset.id)}`, {
        method: 'DELETE',
      });

      if (!response.ok && response.status !== 204) {
        const payload = await response.json().catch(() => ({}));
        throw new Error(payload.error || `Request failed with status ${response.status}`);
      }

      setStatus('Preset deleted.');
      await loadPresets();
    } catch (error) {
      feedback.classList.add('error');
      feedback.textContent = error instanceof Error ? error.message : 'Delete failed.';
    } finally {
      deleteButton.disabled = false;
      deleteButton.textContent = originalLabel;
    }
  });

  card.append(header, description, tags, commandWrap, feedback);
  return card;
}

function renderPresets(presets) {
  presetsEl.innerHTML = '';

  if (!Array.isArray(presets) || presets.length === 0) {
    renderEmpty('No presets found. Use Create to add one.');
    return;
  }

  presets.forEach((preset, index) => {
    presetsEl.appendChild(createPresetCard(preset, index));
  });
}

async function loadPresets() {
  try {
    const response = await fetch('/api/presets');
    if (!response.ok) {
      throw new Error(`Request failed with status ${response.status}`);
    }

    const presets = await response.json();
    presetsCache = presets;

    // Build favourites set from presets data
    favouritesSet = new Set();
    presets.forEach((p) => {
      if (p.is_favourite) favouritesSet.add(p.id);
    });

    applyFiltersAndRender();
    const presetCount = Array.isArray(presets) ? presets.length : 0;
    setStatus(`Loaded ${presetCount} preset${presetCount === 1 ? '' : 's'}.`);
  } catch (error) {
    renderEmpty('Unable to load presets. Check the backend or refresh after adding valid JSON files.');
    setStatus(error instanceof Error ? error.message : 'Failed to load presets.', true);
  }
}

async function loadDecks() {
  try {
    const response = await fetch('/api/decks');
    if (!response.ok) {
      throw new Error(`Request failed with status ${response.status}`);
    }
    decksCache = await response.json();
    populateDeckSelector();
  } catch (error) {
    console.error('Failed to load decks:', error);
  }
}

function populateDeckSelector() {
  deckSelector.innerHTML = '<option value="">All presets</option>';
  decksCache.forEach((deck) => {
    const opt = document.createElement('option');
    opt.value = deck.name;
    opt.textContent = deck.name;
    if (deck.name === activeDeckFilter) opt.selected = true;
    deckSelector.appendChild(opt);
  });
}

createButton.addEventListener('click', () => {
  openEditor({ name: '', engine: '', model: '', tags: [], description: '', command: '' });
});

shutdownButton.addEventListener('click', async () => {
  const originalLabel = shutdownButton.textContent;
  shutdownButton.disabled = true;
  shutdownButton.textContent = 'Shutting down...';
  setStatus('Stopping server...');

  try {
    const response = await fetch('/api/shutdown', { method: 'POST' });
    if (!response.ok && response.status !== 204) {
      const payload = await response.json().catch(() => ({}));
      throw new Error(payload.error || `Request failed with status ${response.status}`);
    }

    setStatus('Server stopped. Restart PresetDock to continue.');
  } catch (error) {
    setStatus(error instanceof Error ? error.message : 'Failed to stop the server.', true);
  } finally {
    shutdownButton.disabled = false;
    shutdownButton.textContent = originalLabel;
  }
});

editorCloseButton.addEventListener('click', closeEditor);
editorCancelButton.addEventListener('click', closeEditor);

editorRunButton.addEventListener('click', async () => {
  const payload = readEditorValues();

  if (!payload.command || !payload.command.trim()) {
    editorFeedback.classList.add('error');
    editorFeedback.textContent = 'Command is required to run a preset.';
    return;
  }

  editorRunButton.disabled = true;
  editorSaveButton.disabled = true;
  editorFeedback.classList.remove('error');
  editorFeedback.textContent = 'Running...';

  try {
    const response = await fetch('/api/run', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      const body = await response.json().catch(() => ({}));
      throw new Error(body.error || `Request failed with status ${response.status}`);
    }

    editorFeedback.classList.remove('error');
    editorFeedback.textContent = 'Preset launched!';

    // Refresh state
    await Promise.all([loadPresets(), loadDecks()]);
  } catch (error) {
    editorFeedback.classList.add('error');
    editorFeedback.textContent = error instanceof Error ? error.message : 'Failed to run preset.';
  } finally {
    editorRunButton.disabled = false;
    editorSaveButton.disabled = false;
  }
});

dialog.addEventListener('cancel', () => {
  editorFeedback.classList.remove('error');
  editorFeedback.textContent = '';
});

engineInput.addEventListener('input', () => {
  selectedEngine = engineInput.value.trim();
});

editorForm.addEventListener('submit', async (event) => {
  event.preventDefault();

  const payload = readEditorValues();
  if (!payload.name) {
    editorFeedback.classList.add('error');
    editorFeedback.textContent = 'Preset name is required.';
    return;
  }

  if (!payload.engine) {
    payload.engine = selectedEngine || 'unknown';
  }

  // Include source_preset_id when duplicating
  if (window._duplicateSourcePresetId) {
    payload.source_preset_id = window._duplicateSourcePresetId;
    window._duplicateSourcePresetId = null;
  }

  const isEditing = Boolean(activePresetId);
  const endpoint = isEditing ? `/api/presets/${encodeURIComponent(activePresetId)}` : '/api/presets';
  const method = isEditing ? 'PUT' : 'POST';

  editorSaveButton.disabled = true;
  editorFeedback.classList.remove('error');
  editorFeedback.textContent = isEditing ? 'Saving...' : 'Creating...';

  try {
    const response = await fetch(endpoint, {
      method,
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      const errorPayload = await response.json().catch(() => ({}));
      throw new Error(errorPayload.error || `Request failed with status ${response.status}`);
    }

    editorFeedback.textContent = isEditing ? 'Preset saved.' : 'Preset created.';
    setStatus(isEditing ? 'Preset updated.' : 'Preset created.');
    closeEditor();
    await loadPresets();
  } catch (error) {
    editorFeedback.classList.add('error');
    editorFeedback.textContent = error instanceof Error ? error.message : 'Save failed.';
  } finally {
    editorSaveButton.disabled = false;
  }
});

// Deck selector element
const deckSelector = document.getElementById('deck-selector');
const decksButton = document.getElementById('decks-button');

deckSelector.addEventListener('change', () => {
  activeDeckFilter = deckSelector.value || '';
  applyFiltersAndRender();
});

// --- Decks Dialog ---
const decksDialog = document.getElementById('decks-dialog');
const decksCloseButton = document.getElementById('decks-close');
const deckNewButton = document.getElementById('deck-new-button');
const decksListEl = document.getElementById('decks-list');
const deckEditorEmpty = document.getElementById('deck-editor-empty');
const deckEditorForm = document.getElementById('deck-editor-form');
const deckNameInput = document.getElementById('deck-name-input');
const deckPresetsSearch = document.getElementById('deck-presets-search');
const deckPresetsList = document.getElementById('deck-presets-list');
const deckSaveButton = document.getElementById('deck-save-button');
const deckDeleteButton = document.getElementById('deck-delete-button');
const deckFeedback = document.getElementById('deck-feedback');

decksButton.addEventListener('click', () => {
  openDecksDialog();
});

decksCloseButton.addEventListener('click', closeDecksDialog);

function openDecksDialog() {
  renderDecksList();
  deckEditorEmpty.hidden = false;
  deckEditorForm.hidden = true;
  editingDeckName = '';

  if (typeof decksDialog.showModal === 'function') {
    decksDialog.showModal();
  } else {
    decksDialog.setAttribute('open', '');
  }
}

function closeDecksDialog() {
  if (decksDialog.open) {
    decksDialog.close();
  }
}

function renderDecksList() {
  decksListEl.innerHTML = '';
  if (decksCache.length === 0) {
    const empty = document.createElement('div');
    empty.className = 'empty';
    empty.textContent = 'No decks yet. Create one to get started.';
    decksListEl.appendChild(empty);
    return;
  }

  decksCache.forEach((deck) => {
    const item = document.createElement('div');
    item.className = 'deck-list-item';
    if (deck.name === editingDeckName) item.classList.add('selected');

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
  editingDeckName = '';
  deckNameInput.value = '';
  deckEditorEmpty.hidden = true;
  deckEditorForm.hidden = false;
  deckFeedback.textContent = '';
  deckFeedback.classList.remove('error');
  deckPresetsSearch.value = '';
  deckPresetsList.innerHTML = '';

  // Show all presets unselected
  presetsCache.forEach((preset) => {
    const item = document.createElement('label');
    item.className = 'deck-preset-item';

    const cb = document.createElement('input');
    cb.type = 'checkbox';
    cb.checked = false;

    const label = document.createElement('span');
    label.textContent = preset.name || preset.id;

    item.append(cb, label);
    deckPresetsList.appendChild(item);
  });

  renderDecksList();
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
      const response = await fetch(`/api/decks/${encodeURIComponent(editingDeckName)}`, {
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
      const response = await fetch('/api/decks', {
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
    renderDecksList();
    selectDeckForEditing(name);
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
    const response = await fetch(`/api/decks/${encodeURIComponent(editingDeckName)}`, { method: 'DELETE' });
    if (!response.ok && response.status !== 204) {
      const err = await response.json().catch(() => ({}));
      throw new Error(err.error || 'Failed to delete deck');
    }

    deckFeedback.textContent = 'Deck deleted.';

    // If the deleted deck was the active filter, clear it
    if (activeDeckFilter === editingDeckName) {
      activeDeckFilter = '';
      deckSelector.value = '';
      applyFiltersAndRender();
    }

    await loadDecks();
    deckEditorEmpty.hidden = true;
    deckEditorForm.hidden = true;
    editingDeckName = '';
    renderDecksList();
  } catch (error) {
    deckFeedback.classList.add('error');
    deckFeedback.textContent = error.message || 'Delete failed.';
  } finally {
    deckDeleteButton.disabled = false;
  }
});

startHeartbeat();
loadPresets();
loadDecks();
