package runtime

import "os/exec"

// OpenBrowser launches the default browser at the given URL.
func OpenBrowser(url string) error {
	return exec.Command("cmd", "/c", "start", "", url).Start()
}
