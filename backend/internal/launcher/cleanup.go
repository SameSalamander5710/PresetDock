package launcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StartStaleScriptSweeper periodically removes leftover presetdock-*.bat
// temp files older than maxAge. Safety net for cases the normal
// cmd.Wait()-triggered cleanup can't reach — e.g. the app itself crashing
// or being force-killed before its cleanup goroutine runs.
func StartStaleScriptSweeper(interval, maxAge time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sweepStaleScripts(maxAge)
			case <-stop:
				return
			}
		}
	}()
}

func sweepStaleScripts(maxAge time.Duration) {
	dir := os.TempDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "presetdock-") || !strings.HasSuffix(name, ".bat") {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil {
			log.Printf("stale script sweep: failed to remove %s: %v", path, err)
		}
	}
}
