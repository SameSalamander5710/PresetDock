package launcher

import "os/exec"

// CommandPayload describes the inputs required to prepare a launcher command.
type CommandPayload struct {
	Title   string // Console window title (derived from preset name)
	Command string // The raw command to execute
}

// Launcher prepares a command payload into an exec.Cmd ready to be started.
type Launcher interface {
	Prepare(payload CommandPayload) (*exec.Cmd, error)
}
