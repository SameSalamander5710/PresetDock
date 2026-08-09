package launcher

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
)

// PowerShellLauncher opens a new, titled, visible PowerShell console window
// and runs the preset command inside it.
//
// Go's os/exec silently redirects Stdin/Stdout/Stderr to NUL when those
// fields are left unset. That means a direct powershell.exe -NoExit -Command
// call would lose all output (sent to NUL) and exit instantly (-NoExit reads
// NUL stdin as EOF).
//
// This launcher sidesteps the problem by using a hidden outer PowerShell
// process that calls Start-Process to spawn an independent inner PowerShell
// window. The inner process is a grandchild that Go never directly manages,
// so it gets real console handles. The outer process exits immediately after
// spawning, so its own NUL'd stdio doesn't matter.
//
// -EncodedCommand is used for both the outer and inner scripts. The payload
// is UTF-16LE encoded then base64-encoded, which eliminates all quoting and
// escaping issues — pipes, backticks, $vars, and nested quotes in the preset
// command behave exactly as if typed by hand.
type PowerShellLauncher struct{}

// noOp is a cleanup callback that does nothing. PowerShell no longer needs
// temp files, so there are no disk artifacts to remove.
var noOp = func() {}

// Prepare builds a hidden, disposable PowerShell launcher that spawns the
// real, visible PowerShell window via Start-Process.
//
// The inner script (run in the visible window):
//  1. Sets the console window title.
//  2. Executes the user's preset command.
//  3. Prints "Press any key to close..." and waits for a keypress.
//
// The outer script (hidden, exits after spawning):
//   - Calls Start-Process to launch powershell.exe with the encoded inner
//     script, using -NoLogo -NoExit so the window stays open.
//
// The returned cleanup func is a no-op since no temporary artifacts are
// created.
func (l *PowerShellLauncher) Prepare(payload CommandPayload) (*exec.Cmd, func(), error) {
	trimmed := strings.TrimSpace(payload.Command)
	if trimmed == "" {
		return nil, nil, errors.New("preset command is empty")
	}

	title := strings.ReplaceAll(strings.TrimSpace(payload.Title), `'`, `''`)
	if title == "" {
		title = "PresetDock"
	}

	// This runs verbatim in the real, visible console — no escaping layer
	// between it and PowerShell's own parser, so quotes, pipes, $vars in
	// the preset command behave exactly as if typed by hand.
	innerScript := fmt.Sprintf(
		"$Host.UI.RawUI.WindowTitle = '%s'\n"+
			"%s\n"+
			"Write-Host \"`nPress any key to close...\" -ForegroundColor DarkGray\n"+
			"$null = $Host.UI.RawUI.ReadKey('NoEcho,IncludeKeyDown')\n",
		title, trimmed,
	)
	encodedInner := encodePowerShellCommand(innerScript)

	// Spawned as an independent process via Start-Process, so it gets its
	// own fresh console handles — never touches Go's NUL-redirected stdio.
	outerScript := fmt.Sprintf(
		`Start-Process powershell.exe -ArgumentList '-NoLogo','-NoExit','-EncodedCommand','%s' -WindowStyle Normal`,
		encodedInner,
	)
	encodedOuter := encodePowerShellCommand(outerScript)

	// This outer process is just a disposable launcher — hidden, and its
	// own NUL'd stdio doesn't matter because it exits right after spawning.
	command := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-WindowStyle", "Hidden", "-EncodedCommand", encodedOuter)
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow, HideWindow: true}
	return command, noOp, nil
}

// encodePowerShellCommand encodes a PowerShell script for the -EncodedCommand
// parameter. The script is converted to UTF-16LE (little-endian UTF-16) and
// then standard base64-encoded, which is the format PowerShell expects.
func encodePowerShellCommand(script string) string {
	units := utf16.Encode([]rune(script))
	buf := make([]byte, len(units)*2)
	for i, u := range units {
		buf[i*2] = byte(u)
		buf[i*2+1] = byte(u >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
