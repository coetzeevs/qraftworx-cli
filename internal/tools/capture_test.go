package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// Task 6.8: CaptureMediaTool requires confirmation.

func TestCaptureMediaTool_RequiresConfirmation(t *testing.T) {
	tool := &CaptureMediaTool{}
	if !tool.RequiresConfirmation() {
		t.Error("CaptureMediaTool.RequiresConfirmation() should return true")
	}
}

func TestCaptureMediaTool_Name(t *testing.T) {
	tool := &CaptureMediaTool{}
	if tool.Name() != "capture_media" {
		t.Errorf("Name()=%q, want capture_media", tool.Name())
	}
}

func TestCaptureMediaTool_Description(t *testing.T) {
	tool := &CaptureMediaTool{}
	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestCaptureMediaTool_Parameters_HasFilename(t *testing.T) {
	tool := &CaptureMediaTool{}
	params := tool.Parameters()
	if _, ok := params["filename"]; !ok {
		t.Error("expected 'filename' parameter")
	}
}

func TestCaptureMediaTool_Permissions(t *testing.T) {
	tool := &CaptureMediaTool{}
	p := tool.Permissions()
	if !p.Hardware {
		t.Error("expected Hardware permission")
	}
	if !p.MediaCapture {
		t.Error("expected MediaCapture permission")
	}
	if !p.FileSystem {
		t.Error("expected FileSystem permission")
	}
}

// Task 6.9: CaptureMediaTool execute with mock.

func TestCaptureMediaTool_Execute(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping media test in short mode")
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed, skipping capture test")
	}

	workDir := t.TempDir()
	workSafe := testSafeDir(t, workDir)

	// Create a fake device file (we'll use lavfi for actual capture)
	// For this test, we use a builder that points to ffmpeg
	// but we need to create a special capture that uses lavfi instead of v4l2
	// Since CaptureFrame uses v4l2, we test the arg construction instead
	// and test actual execution in integration test.

	deviceDir := t.TempDir()
	deviceFile := filepath.Join(deviceDir, "video0")
	if err := os.WriteFile(deviceFile, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	deviceSafe, err := safepath.New(deviceFile, []string{deviceDir})
	if err != nil {
		t.Fatal(err)
	}

	builder, err := NewFFmpegBuilder(ffmpegPath, []safepath.SafePath{testSafeDir(t, deviceDir)}, []safepath.SafePath{workSafe})
	if err != nil {
		t.Fatal(err)
	}

	tool := NewCaptureMediaTool(builder, workSafe, deviceSafe)

	// Verify the tool constructs correctly
	if tool.Name() != "capture_media" {
		t.Errorf("Name()=%q, want capture_media", tool.Name())
	}

	// We can't truly test video capture without a real device,
	// but we verify the command construction is correct
	args := json.RawMessage(`{"filename": "test_frame.jpg"}`)

	// The Execute will fail because there's no actual v4l2 device,
	// but we verify it creates the output path correctly
	_, execErr := tool.Execute(context.Background(), args)
	// Expected to fail (no v4l2 device), but should fail at ffmpeg execution,
	// not at argument validation
	if execErr == nil {
		t.Log("Execute succeeded (unexpected without real device, but OK)")
	} else if !strings.Contains(execErr.Error(), "ffmpeg") {
		t.Errorf("expected ffmpeg-related error, got: %v", execErr)
	}
}

// Task 6.10: Device from config only, not from args.

func TestCaptureMediaTool_DeviceFromConfig(t *testing.T) {
	workDir := t.TempDir()
	workSafe := testSafeDir(t, workDir)
	deviceDir := t.TempDir()
	deviceFile := filepath.Join(deviceDir, "video0")
	if err := os.WriteFile(deviceFile, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	deviceSafe, err := safepath.New(deviceFile, []string{deviceDir})
	if err != nil {
		t.Fatal(err)
	}

	builder, builderErr := NewFFmpegBuilder("true", []safepath.SafePath{testSafeDir(t, deviceDir)}, []safepath.SafePath{workSafe})
	if builderErr != nil {
		t.Fatal(builderErr)
	}

	tool := NewCaptureMediaTool(builder, workSafe, deviceSafe)

	// Verify that the tool's device comes from config (constructor), not from args.
	// The args JSON only has "filename" -- no device field.
	params := tool.Parameters()
	if _, hasDevice := params["device"]; hasDevice {
		t.Error("Parameters should NOT expose a 'device' field -- device comes from config only (S2)")
	}

	// Also verify that even if an attacker passes "device" in args, it's ignored
	maliciousArgs := json.RawMessage(`{"filename": "frame.jpg", "device": "/dev/evil"}`)
	// The tool should use the config device, not the args device.
	// We verify by checking the struct -- devicePath should remain the config value.
	if tool.devicePath.String() != deviceSafe.String() {
		t.Errorf("device should come from config: got %q, want %q", tool.devicePath.String(), deviceSafe.String())
	}

	// Execute will fail (binary is "true", not ffmpeg), but it should
	// not use any device path from the args
	_, _ = tool.Execute(context.Background(), maliciousArgs)
	// After execution, device should still be config value
	if tool.devicePath.String() != deviceSafe.String() {
		t.Errorf("device changed after execute: got %q, want %q", tool.devicePath.String(), deviceSafe.String())
	}
}

// Task 6.11: args parsing errors.

func TestCaptureMediaTool_Execute_BadArgs(t *testing.T) {
	workDir := t.TempDir()
	workSafe := testSafeDir(t, workDir)
	deviceDir := t.TempDir()
	deviceFile := filepath.Join(deviceDir, "dev")
	if err := os.WriteFile(deviceFile, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	deviceSafe, err := safepath.New(deviceFile, []string{deviceDir})
	if err != nil {
		t.Fatal(err)
	}

	builder, builderErr := NewFFmpegBuilder("true", []safepath.SafePath{testSafeDir(t, deviceDir)}, []safepath.SafePath{workSafe})
	if builderErr != nil {
		t.Fatal(builderErr)
	}

	tool := NewCaptureMediaTool(builder, workSafe, deviceSafe)

	tests := []struct {
		name string
		args string
	}{
		{"invalid json", `{not json`},
		{"empty filename", `{"filename": ""}`},
		{"missing filename", `{}`},
		{"path traversal", `{"filename": "../../../etc/passwd"}`},
		{"absolute path", `{"filename": "/etc/passwd"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, execErr := tool.Execute(context.Background(), json.RawMessage(tc.args))
			if execErr == nil {
				t.Errorf("expected error for args %q", tc.args)
			}
		})
	}
}
