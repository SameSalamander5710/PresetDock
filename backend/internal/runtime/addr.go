package runtime

import "fmt"

// Port is the TCP port the local server listens on.
const Port = 8765

// ListenAddr returns the loopback address the server binds to.
func ListenAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", Port)
}

// LocalURL returns the base URL of the local instance.
func LocalURL() string {
	return "http://" + ListenAddr()
}

// LocalPresetsURL returns the /api/presets endpoint URL, used to probe for
// an already-running instance.
func LocalPresetsURL() string {
	return LocalURL() + "/api/presets"
}
