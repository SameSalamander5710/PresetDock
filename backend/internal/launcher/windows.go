package launcher

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

const createNoWindow = 0x08000000

// WindowsCmdLauncher is the default launcher for Windows that uses cmd.exe
// to open a titled console window and run the preset command.
type WindowsCmdLauncher struct{}

// Prepare builds a cmd.exe process that starts a new console window with the
// given title and runs the command via "cmd /K".
func (l *WindowsCmdLauncher) Prepare(payload CommandPayload) (*exec.Cmd, error) {
	trimmed := strings.TrimSpace(payload.Command)
	if trimmed == "" {
		return nil, errors.New("preset command is empty")
	}

	title := strings.ReplaceAll(strings.TrimSpace(payload.Title), `"`, "'")
	if title == "" {
		title = "PresetDock"
	}

	rawCmdLine := fmt.Sprintf(`cmd /C start "%s" cmd /K %s`, title, trimmed)

	command := exec.Command("cmd")
	command.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       rawCmdLine,
		CreationFlags: createNoWindow,
		HideWindow:    true,
	}
	return command, nil
}
