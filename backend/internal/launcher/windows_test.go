package launcher

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsCmdLauncherPrepare_EmptyCommand(t *testing.T) {
	l := WindowsCmdLauncher{}
	_, err := l.Prepare(CommandPayload{Title: "Test", Command: ""})
	if err == nil {
		t.Fatal("expected error for empty command, got nil")
	}

	_, err = l.Prepare(CommandPayload{Title: "Test", Command: "  "})
	if err == nil {
		t.Fatal("expected error for whitespace-only command, got nil")
	}
}

func TestWindowsCmdLauncherPrepare_TitleSanitization(t *testing.T) {
	l := WindowsCmdLauncher{}

	cmd, err := l.Prepare(CommandPayload{Title: `My "Preset"`, Command: "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the SysProcAttr CmdLine contains sanitized title (quotes replaced with single quotes)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil")
	}

	cmdLine := cmd.SysProcAttr.CmdLine
	if !strings.Contains(cmdLine, `My 'Preset'`) {
		t.Errorf("expected sanitized title in CmdLine, got: %s", cmdLine)
	}
}

func TestWindowsCmdLauncherPrepare_DefaultTitle(t *testing.T) {
	l := WindowsCmdLauncher{}

	cmd, err := l.Prepare(CommandPayload{Title: "", Command: "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmdLine := cmd.SysProcAttr.CmdLine
	if !strings.Contains(cmdLine, "PresetDock") {
		t.Errorf("expected default title 'PresetDock' in CmdLine, got: %s", cmdLine)
	}
}

func TestWindowsCmdLauncherPrepare_CommandStructure(t *testing.T) {
	l := WindowsCmdLauncher{}

	cmd, err := l.Prepare(CommandPayload{Title: "MyApp", Command: "llama-server --help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// exec.Command resolves the full path on Windows, so check the base name
	base := filepath.Base(cmd.Path)
	if base != "cmd.exe" && base != "cmd" {
		t.Errorf("expected cmd.Path base to be 'cmd' or 'cmd.exe', got: %s", base)
	}

	cmdLine := cmd.SysProcAttr.CmdLine
	if !strings.HasPrefix(cmdLine, `cmd /C start "MyApp" cmd /K `) {
		t.Errorf("unexpected CmdLine prefix: %s", cmdLine)
	}

	if !strings.HasSuffix(cmdLine, "llama-server --help") {
		t.Errorf("expected CmdLine to end with the command, got: %s", cmdLine)
	}
}

func TestWindowsCmdLauncherPrepare_CreationFlags(t *testing.T) {
	l := WindowsCmdLauncher{}

	cmd, err := l.Prepare(CommandPayload{Title: "Test", Command: "echo ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.SysProcAttr.CreationFlags != createNoWindow {
		t.Errorf("expected CreationFlags %d, got %d", createNoWindow, cmd.SysProcAttr.CreationFlags)
	}

	if !cmd.SysProcAttr.HideWindow {
		t.Error("expected HideWindow to be true")
	}
}

func TestWindowsCmdLauncherPrepare_ImplementsLauncher(t *testing.T) {
	var l Launcher = &WindowsCmdLauncher{}
	if l == nil {
		t.Fatal("*WindowsCmdLauncher does not implement Launcher")
	}
}

func TestWindowsCmdLauncherPrepare_ReturnsNonNilCmd(t *testing.T) {
	l := WindowsCmdLauncher{}
	cmd, err := l.Prepare(CommandPayload{Title: "Example", Command: "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd == nil {
		t.Fatal("expected non-nil *exec.Cmd")
	}
}
