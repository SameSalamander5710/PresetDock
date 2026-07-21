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
const nameInput = document.getElementById('preset-name');
const engineInput = document.getElementById('preset-engine');
const modelInput = document.getElementById('preset-model');
const tagsInput = document.getElementById('preset-tags');
const descriptionInput = document.getElementById('preset-description');
const commandInput = document.getElementById('preset-command');

let activePresetId = '';
let presetsCache = [];
let selectedEngine = '';

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

function createPresetCard(preset, index) {
  const card = document.createElement('article');
  card.className = 'card';
  card.style.animationDelay = `${Math.min(index * 70, 350)}ms`;

  const header = document.createElement('div');
  header.className = 'card-header';

  const titleWrap = document.createElement('div');
  const title = document.createElement('h2');
  title.textContent = preset.name || preset.id;
  const metaRow = document.createElement('div');
  metaRow.className = 'meta-row';

  const model = document.createElement('p');
  model.className = 'model';
  model.textContent = preset.model || 'No model specified';

  metaRow.append(model, createEngineBadge(preset.engine));
  titleWrap.append(title, metaRow);

  const actionButtons = document.createElement('div');
  actionButtons.className = 'action-buttons';

  const runButton = document.createElement('button');
  runButton.type = 'button';
  runButton.textContent = 'Run';

  const editButton = document.createElement('button');
  editButton.type = 'button';
  editButton.textContent = 'Edit';
  editButton.className = 'secondary-button';

  const deleteButton = document.createElement('button');
  deleteButton.type = 'button';
  deleteButton.textContent = 'Delete';
  deleteButton.className = 'danger-button';

  actionButtons.append(runButton, editButton, deleteButton);
  header.append(titleWrap, actionButtons);

  const description = document.createElement('p');
  description.className = 'description';
  description.textContent = preset.description || 'No description provided.';

  const tags = document.createElement('div');
  tags.className = 'tags';
  (preset.tags || []).forEach((tagText) => {
    tags.appendChild(createTag(tagText));
  });

  const command = document.createElement('pre');
  command.className = 'command';
  command.textContent = preset.command || '';

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

  card.append(header, description, tags, command, feedback);
  return card;
}

function renderPresets(presets) {
  presetsEl.innerHTML = '';

  if (!Array.isArray(presets) || presets.length === 0) {
    renderEmpty('No presets found. Use Create to add one.');
    return;
  }

  presetsCache = presets;
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
    renderPresets(presets);
    const presetCount = Array.isArray(presets) ? presets.length : 0;
    setStatus(`Loaded ${presetCount} preset${presetCount === 1 ? '' : 's'}.`);
  } catch (error) {
    renderEmpty('Unable to load presets. Check the backend or refresh after adding valid JSON files.');
    setStatus(error instanceof Error ? error.message : 'Failed to load presets.', true);
  }
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

startHeartbeat();
loadPresets();
