package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdLauncherPrepare_EmptyCommand(t *testing.T) {
	l := CmdLauncher{}
	_, _, err := l.Prepare(CommandPayload{Title: "Test", Command: ""})
	if err == nil {
		t.Fatal("expected error for empty command, got nil")
	}

	_, _, err = l.Prepare(CommandPayload{Title: "Test", Command: "  "})
	if err == nil {
		t.Fatal("expected error for whitespace-only command, got nil")
	}
}

func TestCmdLauncherPrepare_TitleSanitization(t *testing.T) {
	l := CmdLauncher{}

	cmd, cleanup, err := l.Prepare(CommandPayload{Title: `My "Preset"`, Command: "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	// Verify the SysProcAttr CmdLine contains sanitized title (quotes replaced with single quotes)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil")
	}

	cmdLine := cmd.SysProcAttr.CmdLine
	if !strings.Contains(cmdLine, `My 'Preset'`) {
		t.Errorf("expected sanitized title in CmdLine, got: %s", cmdLine)
	}
}

func TestCmdLauncherPrepare_DefaultTitle(t *testing.T) {
	l := CmdLauncher{}

	cmd, cleanup, err := l.Prepare(CommandPayload{Title: "", Command: "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	cmdLine := cmd.SysProcAttr.CmdLine
	if !strings.Contains(cmdLine, "PresetDock") {
		t.Errorf("expected default title 'PresetDock' in CmdLine, got: %s", cmdLine)
	}
}

func TestCmdLauncherPrepare_CommandStructure(t *testing.T) {
	l := CmdLauncher{}

	cmd, cleanup, err := l.Prepare(CommandPayload{Title: "MyApp", Command: "llama-server --help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	// exec.Command resolves the full path on Windows, so check the base name
	base := filepath.Base(cmd.Path)
	if base != "cmd.exe" && base != "cmd" {
		t.Errorf("expected cmd.Path base to be 'cmd' or 'cmd.exe', got: %s", base)
	}

	cmdLine := cmd.SysProcAttr.CmdLine
	if !strings.HasPrefix(cmdLine, `cmd /C start /WAIT "MyApp" cmd /K `) {
		t.Errorf("unexpected CmdLine prefix: %s", cmdLine)
	}

	// The command is written inside the batch file, so the CmdLine ends with
	// the quoted .bat path — verify it references a presetdock-*.bat file.
	if !strings.Contains(cmdLine, "presetdock-") || !strings.HasSuffix(cmdLine, `.bat"`) {
		t.Errorf("expected CmdLine to reference a presetdock-*.bat file, got: %s", cmdLine)
	}
}

func TestCmdLauncherPrepare_CreationFlags(t *testing.T) {
	l := CmdLauncher{}

	cmd, cleanup, err := l.Prepare(CommandPayload{Title: "Test", Command: "echo ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if cmd.SysProcAttr.CreationFlags != createNoWindow {
		t.Errorf("expected CreationFlags %d, got %d", createNoWindow, cmd.SysProcAttr.CreationFlags)
	}
}

func TestCmdLauncherPrepare_BatchScriptCreated(t *testing.T) {
	l := CmdLauncher{}

	cmd, cleanup, err := l.Prepare(CommandPayload{Title: "Test", Command: "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the batch file exists before cleanup
	cmdLine := cmd.SysProcAttr.CmdLine
	// The bat path is quoted at the end of the CmdLine, e.g. ... cmd /K "C:\Users\...\presetdock-abc123.bat"
	batPath := strings.TrimSuffix(strings.TrimPrefix(strings.Fields(strings.TrimPrefix(cmdLine, `cmd /C start /WAIT "Test" cmd /K `))[0], `"`), `"`)
	if _, err := os.Stat(batPath); err != nil {
		t.Fatalf("batch file should exist before cleanup: %v", err)
	}

	// Verify cleanup removes the batch file
	cleanup()
	if _, err := os.Stat(batPath); !os.IsNotExist(err) {
		t.Errorf("batch file should be removed after cleanup, still exists at %s", batPath)
	}
}

func TestCmdLauncherPrepare_BatchScriptContent(t *testing.T) {
	l := CmdLauncher{}

	cmd, cleanup, err := l.Prepare(CommandPayload{Title: "MyTitle", Command: "set FOO=bar && echo done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	// Extract bat path from CmdLine
	cmdLine := cmd.SysProcAttr.CmdLine
	batPath := strings.TrimSuffix(strings.TrimPrefix(strings.Fields(strings.TrimPrefix(cmdLine, `cmd /C start /WAIT "MyTitle" cmd /K `))[0], `"`), `"`)

	content, err := os.ReadFile(batPath)
	if err != nil {
		t.Fatalf("failed to read batch file: %v", err)
	}

	script := string(content)
	if !strings.Contains(script, "set FOO=bar && echo done") {
		t.Errorf("batch script should contain the full command, got: %s", script)
	}
}

func TestCmdLauncherPrepare_ImplementsLauncher(t *testing.T) {
	var l Launcher = &CmdLauncher{}
	if l == nil {
		t.Fatal("*CmdLauncher does not implement Launcher")
	}
}

func TestCmdLauncherPrepare_ReturnsNonNilCmdAndCleanup(t *testing.T) {
	l := CmdLauncher{}
	cmd, cleanup, err := l.Prepare(CommandPayload{Title: "Example", Command: "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd == nil {
		t.Fatal("expected non-nil *exec.Cmd")
	}
	if cleanup == nil {
		t.Fatal("expected non-nil cleanup function")
	}
}
