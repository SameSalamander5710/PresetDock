package launcher

import (
	"encoding/base64"
	"os/exec"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestPowerShellPrepareEmptyCommand(t *testing.T) {
	l := &PowerShellLauncher{}
	_, _, err := l.Prepare(CommandPayload{Title: "Test", Command: "   "})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestPowerShellPrepareDefaultTitle(t *testing.T) {
	l := &PowerShellLauncher{}
	cmd, cleanup, err := l.Prepare(CommandPayload{Title: "  ", Command: "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleanup == nil {
		t.Fatal("cleanup should not be nil")
	}
	cleanup()

	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	if cmd.Path != "" && !strings.HasSuffix(strings.ToLower(cmd.Path), "powershell.exe") {
		t.Errorf("expected powershell.exe, got %s", cmd.Path)
	}

	encoded := cmd.Args[len(cmd.Args)-1]
	script, err := decodeBase64ToScript(encoded)
	if err != nil {
		t.Fatalf("failed to decode outer script: %v", err)
	}
	if !strings.Contains(script, "Start-Process") {
		t.Error("outer script should contain Start-Process")
	}
}

func TestPowerShellPrepareTitleEscaping(t *testing.T) {
	l := &PowerShellLauncher{}
	title := "user" + string(rune(39)) + "s preset"
	cmd, _, err := l.Prepare(CommandPayload{Title: title, Command: "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	encoded := cmd.Args[len(cmd.Args)-1]
	outerScript, err := decodeBase64ToScript(encoded)
	if err != nil {
		t.Fatalf("failed to decode outer script: %v", err)
	}

	innerEncoded := extractEncodedCommand(outerScript)
	innerScript, err := decodeBase64ToScript(innerEncoded)
	if err != nil {
		t.Fatalf("failed to decode inner script: %v", err)
	}

	escaped := string(rune(39)) + string(rune(39))
	if !strings.Contains(innerScript, "user"+escaped+"s preset") {
		t.Errorf("title single quotes should be escaped, got: %s", innerScript)
	}
}

func TestPowerShellPrepareSysProcAttr(t *testing.T) {
	l := &PowerShellLauncher{}
	cmd, _, err := l.Prepare(CommandPayload{Title: "Test", Command: "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr should not be nil")
	}
	if cmd.SysProcAttr.CreationFlags != createNoWindow {
		t.Errorf("expected CreationFlags=createNoWindow, got %d", cmd.SysProcAttr.CreationFlags)
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow should be true")
	}
}

func TestPowerShellPrepareCommandStructure(t *testing.T) {
	l := &PowerShellLauncher{}
	cmd, _, err := l.Prepare(CommandPayload{Title: "MyPreset", Command: "nvidia-smi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedArgs := []string{"-NoLogo", "-NoProfile", "-WindowStyle", "Hidden", "-EncodedCommand"}
	for i, arg := range expectedArgs {
		if i+1 >= len(cmd.Args) {
			t.Fatalf("missing argument at position %d", i+1)
		}
		if cmd.Args[i+1] != arg {
			t.Errorf("expected arg[%d]=%s, got %s", i+1, arg, cmd.Args[i+1])
		}
	}

	encoded := cmd.Args[len(cmd.Args)-1]
	outerScript, err := decodeBase64ToScript(encoded)
	if err != nil {
		t.Fatalf("failed to decode outer script: %v", err)
	}

	innerEncoded := extractEncodedCommand(outerScript)
	innerScript, err := decodeBase64ToScript(innerEncoded)
	if err != nil {
		t.Fatalf("failed to decode inner script: %v", err)
	}

	checks := []struct {
		name     string
		contains string
	}{
		{"window title", "WindowTitle"},
		{"preset title", "MyPreset"},
		{"user command", "nvidia-smi"},
		{"pause message", "Press any key to close"},
		{"key read", "ReadKey"},
	}

	for _, c := range checks {
		if !strings.Contains(innerScript, c.contains) {
			t.Errorf("inner script should contain %s (%s)", c.name, c.contains)
		}
	}
}

func TestEncodePowerShellCommand(t *testing.T) {
	script := "Write-Host Hello"
	encoded := encodePowerShellCommand(script)

	decoded, err := decodeBase64ToScript(encoded)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if decoded != script {
		t.Errorf("round-trip failed: expected %q, got %q", script, decoded)
	}
}

func TestEncodePowerShellCommandUnicode(t *testing.T) {
	script := "Write-Host Test"
	encoded := encodePowerShellCommand(script)

	decoded, err := decodeBase64ToScript(encoded)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if decoded != script {
		t.Errorf("unicode round-trip failed: expected %q, got %q", script, decoded)
	}
}

func TestEncodePowerShellCommandSpecialChars(t *testing.T) {
	script := "echo test | Select-String test"
	encoded := encodePowerShellCommand(script)

	decoded, err := decodeBase64ToScript(encoded)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if decoded != script {
		t.Errorf("special chars round-trip failed: expected %q, got %q", script, decoded)
	}
}

func decodeBase64ToScript(encoded string) (string, error) {
	buf, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	units := make([]uint16, len(buf)/2)
	for i := 0; i < len(buf); i += 2 {
		units[i/2] = uint16(buf[i]) | uint16(buf[i+1])<<8
	}
	return string(utf16.Decode(units)), nil
}

func extractEncodedCommand(script string) string {
	// Outer script format (as generated by powershell_launcher.go):
	// Start-Process powershell.exe -ArgumentList '-NoLogo','-NoExit','-EncodedCommand','<base64>'
	// After '-EncodedCommand' we have the closing quote ', then comma, then opening quote '<base64>'
	marker := "-EncodedCommand"
	idx := strings.Index(script, marker)
	if idx == -1 {
		return ""
	}
	rest := script[idx+len(marker):]
	// Pattern after marker: ','<base64>'
	// Skip past the closing quote of -EncodedCommand: '
	if len(rest) == 0 || rest[0] != '\'' {
		return ""
	}
	rest = rest[1:] // skip closing quote of -EncodedCommand
	// Skip comma
	if len(rest) == 0 || rest[0] != ',' {
		return ""
	}
	rest = rest[1:]
	// Skip opening quote of base64 value
	if len(rest) == 0 || rest[0] != '\'' {
		return ""
	}
	rest = rest[1:] // skip opening quote
	endIdx := strings.Index(rest, "'")
	if endIdx == -1 {
		return ""
	}
	return rest[:endIdx]
}

var _ Launcher = (*PowerShellLauncher)(nil)
var _ *exec.Cmd = (*exec.Cmd)(nil)
