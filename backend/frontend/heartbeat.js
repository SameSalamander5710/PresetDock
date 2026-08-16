// --------------------------------------------------------------------------
// Heartbeat — client-side liveness loop
// --------------------------------------------------------------------------

let heartbeatInterval = null;

function startHeartbeat() {
  if (heartbeatInterval) return;
  heartbeatInterval = setInterval(async () => {
    try {
      // Backend returns 204 No Content — no body to parse, any outcome is fine.
      await fetch(`${apiBase}/api/heartbeat`, { method: 'POST' });
    } catch {
      // ignore heartbeat failures
    }
  }, 30000);
}

function stopHeartbeat() {
  if (heartbeatInterval) {
    clearInterval(heartbeatInterval);
    heartbeatInterval = null;
  }
}