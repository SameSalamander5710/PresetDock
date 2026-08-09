package launcher

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const createNoWindow = 0x08000000

type WindowsCmdLauncher struct{}

func (l *WindowsCmdLauncher) Prepare(payload CommandPayload) (*exec.Cmd, error) {
	trimmed := strings.TrimSpace(payload.Command)
	if trimmed == "" {
		return nil, errors.New("preset command is empty")
	}

	title := strings.ReplaceAll(strings.TrimSpace(payload.Title), `"`, "'")
	if title == "" {
		title = "PresetDock"
	}

	// Write the command to a temp .bat file. This is the key fix: it avoids
	// passing operators like && | ^ through TWO layers of cmd.exe parsing
	// (the outer "cmd /C start ..." wrapper, then the inner "cmd /K ...").
	// Previously the outer layer split on the command's own "&&", running
	// half the line in the new visible window and the OTHER half silently
	// in the hidden outer process — which is why output/Ctrl+C never worked.
	batFile, err := os.CreateTemp("", "presetdock-*.bat")
	if err != nil {
		return nil, fmt.Errorf("create temp script: %w", err)
	}
	batPath := batFile.Name() + ".bat" // os.CreateTemp doesn't add an extension itself
	batFile.Close()
	os.Rename(batFile.Name(), batPath)

	script := fmt.Sprintf("@echo off\r\ntitle %s\r\n%s\r\n", title, trimmed)
	if err := os.WriteFile(batPath, []byte(script), 0644); err != nil {
		return nil, fmt.Errorf("write temp script: %w", err)
	}

	// start opens the real, independent, visible window; cmd /K runs the
	// script inside it and keeps it open. Quote the path in case it (or
	// payload data) contains spaces.
	rawCmdLine := fmt.Sprintf(`cmd /C start "%s" cmd /K "%s"`, title, batPath)

	command := exec.Command("cmd")
	command.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       rawCmdLine,
		CreationFlags: createNoWindow, // only hides the outer, throwaway cmd
	}
	return command, nil
}
