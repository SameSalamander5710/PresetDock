package launcher

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// createNewConsole gives PowerShell its own window directly.
const createNewConsole = 0x00000010

// PowerShellLauncher opens a new, titled, visible PowerShell console window
// and runs the preset command inside it.
//
// Unlike cmd.exe, PowerShell's .exe parses its own argv the standard Windows
// way, so arguments can be passed normally through exec.Command without
// needing raw CmdLine or temp batch files.
type PowerShellLauncher struct{}

// noOp is a cleanup callback that does nothing (PowerShell needs no temp files).
var noOp = func() {}

// Prepare builds a powershell.exe process that opens a new visible console
// window (via CREATE_NEW_CONSOLE) and runs the preset command inside it.
//
// -NoExit keeps the window open after the command finishes so output stays
// visible. The window title is set first so presets are easy to tell apart
// when several are running at once.
//
// The returned cleanup func is a no-op since PowerShell does not require
// temporary artifacts.
func (l *PowerShellLauncher) Prepare(payload CommandPayload) (*exec.Cmd, func(), error) {
	trimmed := strings.TrimSpace(payload.Command)
	if trimmed == "" {
		return nil, nil, errors.New("preset command is empty")
	}

	title := strings.ReplaceAll(strings.TrimSpace(payload.Title), `'`, `''`)
	if title == "" {
		title = "PresetDock"
	}

	// Set the window title first, then execute the user command.
	script := fmt.Sprintf(`$Host.UI.RawUI.WindowTitle = '%s'; %s`, title, trimmed)

	command := exec.Command("powershell.exe", "-NoLogo", "-NoExit", "-Command", script)
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole}
	return command, noOp, nil
}
