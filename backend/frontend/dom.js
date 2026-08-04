// --------------------------------------------------------------------------
// DOM lookup and assertion helpers
// --------------------------------------------------------------------------

/**
 * assertElement - fail fast if a required element is missing.
 * @param {string} id - Element ID to look up.
 * @param {string} [context=''] - Optional context for error message.
 * @returns {HTMLElement}
 */
function assertElement(id, context = '') {
  const el = document.getElementById(id);
  if (!el) {
    throw new Error(`Missing required element #${id}${context ? ' (' + context + ')' : ''}`);
  }
  return el;
}

/**
 * setStatus - Update the status bar text.
 * @param {string} text - Status message.
 * @param {boolean} [isError=false] - Whether this is an error message.
 */
function setStatus(text, isError = false) {
  const el = document.getElementById('status');
  if (!el) return;
  el.textContent = text;
  el.classList.toggle('error', isError);
}