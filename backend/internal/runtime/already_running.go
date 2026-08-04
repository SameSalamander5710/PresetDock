package runtime

import (
	"log"
	"net/http"
	"time"
)

const localPresetsURL = "http://127.0.0.1:8765/api/presets"
const localUIURL = "http://127.0.0.1:8765"

// LaunchIfAlreadyRunning checks whether another PresetDock instance is already
// serving on the local port. When one is found it re-opens the browser and
// returns true so the caller can exit gracefully.
func LaunchIfAlreadyRunning() bool {
	client := &http.Client{Timeout: 750 * time.Millisecond}
	response, err := client.Get(localPresetsURL)
	if err != nil {
		return false
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return false
	}

	if err := OpenBrowser(localUIURL); err != nil {
		log.Printf("browser launch failed: %v", err)
	}
	log.Println("PresetDock is already running; reopened the browser.")
	return true
}
