const presetsEl = document.getElementById('presets');
const statusEl = document.getElementById('status');
const newPresetEl = document.getElementById('new-preset');

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

function createField(labelText, name, value, options = {}) {
  const field = document.createElement('label');
  field.className = options.fullWidth ? 'field full-width' : 'field';

  const label = document.createElement('span');
  label.className = 'field-label';
  label.textContent = labelText;

  let control;
  if (options.multiline) {
    control = document.createElement('textarea');
    control.rows = options.rows || 3;
  } else {
    control = document.createElement('input');
    control.type = 'text';
  }

  control.name = name;
  control.value = value || '';
  if (options.placeholder) {
    control.placeholder = options.placeholder;
  }

  field.append(label, control);
  return { field, control };
}

function renderEmpty(message) {
  presetsEl.innerHTML = '';
  const empty = document.createElement('div');
  empty.className = 'empty';
  empty.textContent = message;
  presetsEl.appendChild(empty);
}

function readPresetForm(form) {
  const values = new FormData(form);
  return {
    id: String(values.get('id') || '').trim(),
    name: String(values.get('name') || '').trim(),
    model: String(values.get('model') || '').trim(),
    tags: textToTags(String(values.get('tags') || '')),
    description: String(values.get('description') || '').trim(),
    command: String(values.get('command') || '').trim(),
  };
}

function createPresetEditor(preset, options = {}) {
  const isExisting = Boolean(options.originalId);
  const form = document.createElement('form');
  form.className = 'card preset-form';
  form.dataset.originalId = options.originalId || '';

  const header = document.createElement('div');
  header.className = 'card-header';

  const headerText = document.createElement('div');
  const title = document.createElement('h2');
  title.textContent = isExisting ? `Editing ${options.originalId}` : 'New preset';
  const subtitle = document.createElement('p');
  subtitle.className = 'model';
  subtitle.textContent = isExisting ? 'Update the saved file and click Save preset.' : 'Fill out the form and create a new file.';
  headerText.append(title, subtitle);

  const headerActions = document.createElement('div');
  headerActions.className = 'actions';

  const saveButton = document.createElement('button');
  saveButton.type = 'submit';
  saveButton.textContent = isExisting ? 'Save preset' : 'Create preset';

  headerActions.appendChild(saveButton);

  header.append(headerText, headerActions);

  const grid = document.createElement('div');
  grid.className = 'field-grid';

  const idField = createField('Preset ID', 'id', preset.id || '', {
    placeholder: 'example',
  });
  const nameField = createField('Name', 'name', preset.name || '', {
    placeholder: 'Gemma 2 9B Q4',
  });
  const modelField = createField('Model', 'model', preset.model || '', {
    placeholder: 'gemma-2-9b-it-Q4_K_M.gguf',
  });
  const tagsField = createField('Tags', 'tags', tagsToText(preset.tags), {
    multiline: true,
    rows: 3,
    fullWidth: true,
    placeholder: 'gemma, 9b, q4',
  });
  const descriptionField = createField('Description', 'description', preset.description || '', {
    multiline: true,
    rows: 3,
    fullWidth: true,
    placeholder: 'General purpose, balanced speed/quality',
  });
  const commandField = createField('Command', 'command', preset.command || '', {
    multiline: true,
    rows: 7,
    fullWidth: true,
    placeholder: 'llama-cli.exe -m models/...',
  });

  grid.append(
    idField.field,
    nameField.field,
    modelField.field,
    tagsField.field,
    descriptionField.field,
    commandField.field,
  );

  const footer = document.createElement('div');
  footer.className = 'form-footer';

  const feedback = document.createElement('span');
  feedback.className = 'feedback';

  const runButton = document.createElement('button');
  runButton.type = 'button';
  runButton.textContent = 'Run';
  if (!isExisting) {
    runButton.disabled = true;
    runButton.title = 'Save the new preset before running it.';
  }

  footer.append(feedback, runButton);

  let autoSuggestedId = !isExisting;
  if (!isExisting) {
    nameField.control.addEventListener('input', () => {
      if (!autoSuggestedId || idField.control.value.trim()) {
        return;
      }

      idField.control.value = slugify(nameField.control.value);
    });

    idField.control.addEventListener('input', () => {
      autoSuggestedId = false;
    });
  }

  form.addEventListener('submit', async (event) => {
    event.preventDefault();

    saveButton.disabled = true;
    runButton.disabled = true;
    feedback.classList.remove('error');
    feedback.textContent = isExisting ? 'Saving...' : 'Creating...';

    const payload = readPresetForm(form);
    const originalId = form.dataset.originalId || '';
    const endpoint = originalId ? `/api/presets/${encodeURIComponent(originalId)}` : '/api/presets';
    const method = originalId ? 'PUT' : 'POST';

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

      feedback.textContent = originalId ? 'Preset saved.' : 'Preset created.';
      setStatus(originalId ? 'Preset updated.' : 'Preset created.');
      await loadPresets();
    } catch (error) {
      feedback.classList.add('error');
      feedback.textContent = error instanceof Error ? error.message : 'Save failed.';
    } finally {
      saveButton.disabled = false;
      runButton.disabled = !isExisting;
    }
  });

  runButton.addEventListener('click', async () => {
    if (!options.originalId) {
      return;
    }

    const originalLabel = runButton.textContent;
    runButton.disabled = true;
    feedback.classList.remove('error');
    feedback.textContent = 'Launching...';

    try {
      const response = await fetch(`/api/run/${encodeURIComponent(options.originalId)}`, {
        method: 'POST',
      });

      if (!response.ok) {
        const errorPayload = await response.json().catch(() => ({}));
        throw new Error(errorPayload.error || `Request failed with status ${response.status}`);
      }

      feedback.textContent = 'Launched in a new terminal.';
      runButton.textContent = 'Launched';
    } catch (error) {
      feedback.classList.add('error');
      feedback.textContent = error instanceof Error ? error.message : 'Launch failed.';
    } finally {
      window.setTimeout(() => {
        runButton.textContent = originalLabel;
      }, 1400);
      runButton.disabled = !isExisting;
    }
  });

  form.append(header, grid, footer);
  return form;
}

function renderPresets(presets) {
  newPresetEl.innerHTML = '';
  presetsEl.innerHTML = '';

  newPresetEl.appendChild(createPresetEditor(
    {
      id: '',
      name: '',
      model: '',
      tags: [],
      description: '',
      command: '',
    },
  ));

  if (!Array.isArray(presets) || presets.length === 0) {
    renderEmpty('No presets found. Create one above and save it to the presets folder.');
    return;
  }

  presets.forEach((preset, index) => {
    const editor = createPresetEditor(preset, { originalId: preset.id });
    editor.style.animationDelay = `${Math.min(index * 70, 350)}ms`;
    presetsEl.appendChild(editor);
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
    newPresetEl.innerHTML = '';
    newPresetEl.appendChild(createPresetEditor({ id: '', name: '', model: '', tags: [], description: '', command: '' }));
    renderEmpty('Unable to load presets. Check the backend or refresh after adding valid JSON files.');
    setStatus(error instanceof Error ? error.message : 'Failed to load presets.', true);
  }
}

startHeartbeat();
loadPresets();
