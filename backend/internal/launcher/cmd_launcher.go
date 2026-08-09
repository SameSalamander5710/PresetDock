package launcher

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// CmdLauncher opens a new, titled, visible console window via cmd.exe
// and runs the preset command inside it.
type CmdLauncher struct{}

// Prepare builds a cmd.exe process that opens a new visible console window
// (via "start /WAIT") and runs the preset command inside it via a generated
// batch script. The command is written to a temp .bat rather than inlined
// into the outer CmdLine, because operators like &&, |, ^ would otherwise
// be parsed TWICE — once by the outer "cmd /C start ..." wrapper and once
// by the inner "cmd /K" — silently splitting the command across two
// different processes (one visible with no output, one hidden actually
// running it).
//
// The returned cleanup func removes the temp batch file and must be called
// by the caller after the returned *exec.Cmd exits. Because "start /WAIT"
// blocks the outer process until the spawned window's process terminates,
// cmd.Wait() returning is a reliable signal covering normal exit, the user
// closing the window, AND a force-kill via Task Manager — not just the
// graceful case.
func (l *CmdLauncher) Prepare(payload CommandPayload) (*exec.Cmd, func(), error) {
	trimmed := strings.TrimSpace(payload.Command)
	if trimmed == "" {
		return nil, nil, errors.New("preset command is empty")
	}

	title := strings.ReplaceAll(strings.TrimSpace(payload.Title), `"`, "'")
	if title == "" {
		title = "PresetDock"
	}

	batPath, err := writeBatchScript(title, trimmed)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { os.Remove(batPath) }

	rawCmdLine := fmt.Sprintf(`cmd /C start /WAIT "%s" cmd /K "%s"`, title, batPath)

	command := exec.Command("cmd")
	command.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       rawCmdLine,
		CreationFlags: createNoWindow, // only hides the outer, throwaway cmd
	}
	return command, cleanup, nil
}

// writeBatchScript writes the preset command to a temp .bat file so that
// operators like && | ^ are parsed exactly once, by the visible window's
// own cmd interpreter, instead of being re-split by an outer wrapper.
func writeBatchScript(title, command string) (string, error) {
	f, err := os.CreateTemp("", "presetdock-*.bat")
	if err != nil {
		return "", fmt.Errorf("create temp script: %w", err)
	}
	path := f.Name()
	f.Close()

	script := fmt.Sprintf("@echo off\r\ntitle %s\r\n%s\r\n", title, command)
	if err := os.WriteFile(path, []byte(script), 0644); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("write temp script: %w", err)
	}
	return path, nil
}
