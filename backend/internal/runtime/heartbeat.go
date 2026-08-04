package runtime

import (
	"sync"
	"time"
)

// Heartbeat tracks the last received heartbeat timestamp.
type Heartbeat struct {
	mu       sync.Mutex
	lastBeat time.Time
}

// NewHeartbeat returns a Heartbeat initialised to the current time.
func NewHeartbeat() *Heartbeat {
	return &Heartbeat{lastBeat: time.Now()}
}

// Touch updates the last heartbeat timestamp to now.
func (h *Heartbeat) Touch() {
	h.mu.Lock()
	h.lastBeat = time.Now()
	h.mu.Unlock()
}

// Stale reports whether the heartbeat has not been touched within the given timeout.
func (h *Heartbeat) Stale(timeout time.Duration) bool {
	h.mu.Lock()
	lastBeat := h.lastBeat
	h.mu.Unlock()
	return time.Since(lastBeat) > timeout
}
