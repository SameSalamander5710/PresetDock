// --------------------------------------------------------------------------
// Pane rendering, filtering, suggestions, and view mode
// --------------------------------------------------------------------------

// --------------------------------------------------------------------------
// Preset card creation (shared across panes)
// --------------------------------------------------------------------------
function createPresetCard(preset, isFavourite, actions) {
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
       const currentIsFav = favouritesCache.includes(preset.id);
       const newIsFav = await actions.onToggleFavourite(preset.id, currentIsFav);
      star.classList.toggle('active', newIsFav);
      star.textContent = newIsFav ? '★' : '☆';
      star.title = newIsFav ? 'Remove from favourites' : 'Add to favourites';
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

  const actionButtons = document.createElement('div');
  actionButtons.className = 'action-buttons';

  // Run button
  const runBtn = document.createElement('button');
  runBtn.type = 'button';
  runBtn.textContent = 'Run';
  runBtn.title = 'Execute command in terminal';
  runBtn.addEventListener('click', async () => {
    try {
      await actions.onRun(preset.id, preset.name || preset.id);
    } catch (error) {
      setStatus(error.message, true);
    }
  });

  // Edit button
  const editBtn = document.createElement('button');
  editBtn.type = 'button';
  editBtn.textContent = 'Edit';
  editBtn.className = 'secondary-button';
  editBtn.addEventListener('click', () => {
    actions.onEdit(preset);
  });

  // Duplicate button
  const dupBtn = document.createElement('button');
  dupBtn.type = 'button';
  dupBtn.textContent = 'Duplicate';
  dupBtn.className = 'secondary-button';
  dupBtn.addEventListener('click', async () => {
    try {
      await actions.onDuplicate(preset);
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
      await actions.onDelete(preset.id, preset.name || preset.id);
    } catch (error) {
      setStatus(error.message, true);
    }
  });

  actionButtons.append(runBtn, editBtn, dupBtn, delBtn);

  header.append(titleCol, actionButtons);
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
function renderPane(pane, cardActions) {
  // Fall back to global cardActionsRef when called from event handlers
  // that don't have direct access to the cardActions object
  const actions = cardActions || getCardActionsRef();
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
      const card = createPresetCard(preset, isFav, actions);
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
function renderAllPanes(cardActions) {
  if (leftPane) renderPane(leftPane, cardActions);
  if (rightPane && !rightPane.el.hidden) renderPane(rightPane, cardActions);
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
function setViewModeUI(mode) {
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