package runtime

import (
	"sync"
	"testing"
	"time"
)

func TestNewHeartbeatNotStale(t *testing.T) {
	hb := NewHeartbeat()
	if hb.Stale(time.Minute) {
		t.Errorf("fresh heartbeat should not be stale")
	}
}

func TestStaleAfterBackdating(t *testing.T) {
	hb := NewHeartbeat()
	// White-box: backdate the last beat past the timeout.
	hb.lastBeat = time.Now().Add(-3 * time.Minute)
	if !hb.Stale(2 * time.Minute) {
		t.Errorf("heartbeat backdated 3m should be stale at a 2m timeout")
	}
}

func TestTouchClearsStale(t *testing.T) {
	hb := NewHeartbeat()
	hb.lastBeat = time.Now().Add(-3 * time.Minute)
	if !hb.Stale(2 * time.Minute) {
		t.Fatalf("setup: heartbeat should be stale before Touch")
	}
	hb.Touch()
	if hb.Stale(2 * time.Minute) {
		t.Errorf("heartbeat should be fresh after Touch")
	}
}

func TestHeartbeatConcurrentTouchAndStale(t *testing.T) {
	hb := NewHeartbeat()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			hb.Touch()
		}()
		go func() {
			defer wg.Done()
			_ = hb.Stale(time.Minute)
		}()
	}
	wg.Wait()
	if hb.Stale(time.Minute) {
		t.Errorf("heartbeat should be fresh after concurrent Touches")
	}
}
