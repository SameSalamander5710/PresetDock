// --------------------------------------------------------------------------
// Heartbeat — client-side liveness loop
// --------------------------------------------------------------------------

let heartbeatInterval = null;

function startHeartbeat() {
  if (heartbeatInterval) return;
  heartbeatInterval = setInterval(async () => {
    try {
      const res = await fetch(`${apiBase}/api/heartbeat`, { method: 'POST' });
      // Backend returns 204 No Content — treat any 2xx as success, do NOT parse JSON
      if (res.ok && res.status !== 204) {
        // If a body is ever returned in the future, handle it here
      }
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