package launcher

import "os/exec"

// Shell types supported by PresetDock.
const (
	ShellCmd        = "cmd"
	ShellPowerShell = "powershell"
)

// CommandPayload describes the inputs required to prepare a launcher command.
type CommandPayload struct {
	Title   string // Console window title (derived from preset name)
	Command string // The raw command to execute
	Shell   string // "cmd" (default) or "powershell"
}

// Launcher prepares a command for execution on the current platform.
// The returned cleanup func should be called once the returned *exec.Cmd
// has exited (e.g. after cmd.Wait() returns); it may be nil-safe/no-op on
// platforms that don't need temp-file cleanup.
type Launcher interface {
	Prepare(payload CommandPayload) (*exec.Cmd, func(), error)
}
