package runtime

import (
	"log"
	"net/http"
	"time"
)

// LaunchIfAlreadyRunning checks whether another PresetDock instance is already
// serving on the local port. When one is found it re-opens the browser and
// returns true so the caller can exit gracefully.
func LaunchIfAlreadyRunning() bool {
	client := &http.Client{Timeout: 750 * time.Millisecond}
	response, err := client.Get(LocalPresetsURL())
	if err != nil {
		return false
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return false
	}

	if err := OpenBrowser(LocalURL()); err != nil {
		log.Printf("browser launch failed: %v", err)
	}
	log.Println("PresetDock is already running; reopened the browser.")
	return true
}
