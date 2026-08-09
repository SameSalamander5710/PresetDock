package launcher

import "os/exec"

// Dispatcher implements Launcher by routing Prepare() calls to the correct
// shell-specific launcher based on payload.Shell.
//
// If an unknown shell type is requested, it falls back to CmdLauncher to
// maintain backward compatibility with presets that lack a shell field.
type Dispatcher struct {
	cmdLauncher        Launcher
	powershellLauncher Launcher
}

// NewDispatcher creates a Dispatcher wired with the default shell launchers.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		cmdLauncher:        &CmdLauncher{},
		powershellLauncher: &PowerShellLauncher{},
	}
}

// Prepare routes to the appropriate launcher based on the shell type.
func (d *Dispatcher) Prepare(payload CommandPayload) (*exec.Cmd, func(), error) {
	switch payload.Shell {
	case ShellPowerShell:
		return d.powershellLauncher.Prepare(payload)
	default:
		// ShellCmd or empty/unknown → default to cmd
		return d.cmdLauncher.Prepare(payload)
	}
}
