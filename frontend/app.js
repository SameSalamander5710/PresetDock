const presetsEl = document.getElementById('presets');
const statusEl = document.getElementById('status');

function setStatus(message, isError = false) {
  statusEl.textContent = message;
  statusEl.style.color = isError ? 'var(--danger)' : 'var(--muted)';
}

function createTag(text) {
  const tag = document.createElement('span');
  tag.className = 'tag';
  tag.textContent = text;
  return tag;
}

function renderEmpty(message) {
  presetsEl.innerHTML = '';
  const empty = document.createElement('div');
  empty.className = 'empty';
  empty.textContent = message;
  presetsEl.appendChild(empty);
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
  const model = document.createElement('p');
  model.className = 'model';
  model.textContent = preset.model || 'No model specified';
  titleWrap.append(title, model);

  header.appendChild(titleWrap);

  const description = document.createElement('p');
  description.className = 'description';
  description.textContent = preset.description || 'No description provided.';

  const tags = document.createElement('div');
  tags.className = 'tags';
  (preset.tags || []).forEach((tag) => {
    tags.appendChild(createTag(tag));
  });

  const command = document.createElement('pre');
  command.className = 'command';
  command.textContent = preset.command || '';

  const actions = document.createElement('div');
  actions.className = 'actions';

  const button = document.createElement('button');
  button.type = 'button';
  button.textContent = 'Run';

  const feedback = document.createElement('span');
  feedback.className = 'feedback';

  button.addEventListener('click', async () => {
    const originalLabel = button.textContent;
    button.disabled = true;
    button.textContent = 'Launching...';
    feedback.classList.remove('error');
    feedback.textContent = '';

    try {
      const response = await fetch(`/api/run/${encodeURIComponent(preset.id)}`, {
        method: 'POST',
      });

      if (!response.ok) {
        const payload = await response.json().catch(() => ({}));
        throw new Error(payload.error || `Request failed with status ${response.status}`);
      }

      feedback.textContent = 'Launched in a new terminal.';
      button.textContent = 'Launched';
    } catch (error) {
      feedback.classList.add('error');
      feedback.textContent = error instanceof Error ? error.message : 'Launch failed.';
      button.textContent = originalLabel;
    } finally {
      button.disabled = false;
      window.setTimeout(() => {
        if (button.textContent === 'Launched') {
          button.textContent = originalLabel;
        }
      }, 1400);
    }
  });

  actions.append(button, feedback);
  card.append(header, description, tags, command, actions);
  return card;
}

async function loadPresets() {
  try {
    const response = await fetch('/api/presets');
    if (!response.ok) {
      throw new Error(`Request failed with status ${response.status}`);
    }

    const presets = await response.json();
    presetsEl.innerHTML = '';

    if (!Array.isArray(presets) || presets.length === 0) {
      renderEmpty('No presets found. Add JSON files to the presets folder next to the app and refresh.');
      setStatus('No presets loaded.', true);
      return;
    }

    presets.forEach((preset, index) => {
      presetsEl.appendChild(createPresetCard(preset, index));
    });

    setStatus(`Loaded ${presets.length} preset${presets.length === 1 ? '' : 's'}.`);
  } catch (error) {
    renderEmpty('Unable to load presets. Check the backend or refresh after adding valid JSON files.');
    setStatus(error instanceof Error ? error.message : 'Failed to load presets.', true);
  }
}

loadPresets();
