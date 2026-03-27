package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// Task 6.12: ProcessVideoTool does not require confirmation.

func TestProcessVideoTool_RequiresConfirmation(t *testing.T) {
	tool := &ProcessVideoTool{}
	if tool.RequiresConfirmation() {
		t.Error("ProcessVideoTool.RequiresConfirmation() should return false")
	}
}

func TestProcessVideoTool_Name(t *testing.T) {
	tool := &ProcessVideoTool{}
	if tool.Name() != "process_video" {
		t.Errorf("Name()=%q, want process_video", tool.Name())
	}
}

func TestProcessVideoTool_Description(t *testing.T) {
	tool := &ProcessVideoTool{}
	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestProcessVideoTool_Parameters_HasRequiredFields(t *testing.T) {
	tool := &ProcessVideoTool{}
	params := tool.Parameters()
	for _, field := range []string{"input", "output", "codec"} {
		if _, ok := params[field]; !ok {
			t.Errorf("expected %q parameter", field)
		}
	}
}

func TestProcessVideoTool_Permissions(t *testing.T) {
	tool := &ProcessVideoTool{}
	p := tool.Permissions()
	if !p.FileSystem {
		t.Error("expected FileSystem permission")
	}
	if p.Hardware {
		t.Error("should not have Hardware permission")
	}
	if p.MediaCapture {
		t.Error("should not have MediaCapture permission")
	}
}

// Task 6.13: ProcessVideoTool execute.

func TestProcessVideoTool_Execute(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping media test in short mode")
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed, skipping process_video test")
	}

	workDir := t.TempDir()
	workSafe := testSafeDir(t, workDir)

	// Create a synthetic test video using ffmpeg lavfi
	inputFile := filepath.Join(workDir, "test_input.mp4")
	genCmd := exec.CommandContext(context.Background(), ffmpegPath, //nolint:gosec // G204: test with validated binary path
		"-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=10",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-y",
		inputFile,
	)
	if out, genErr := genCmd.CombinedOutput(); genErr != nil {
		t.Skipf("failed to generate test video: %v\n%s", genErr, out)
	}

	builder, builderErr := NewFFmpegBuilder(ffmpegPath, []safepath.SafePath{workSafe}, []safepath.SafePath{workSafe})
	if builderErr != nil {
		t.Fatal(builderErr)
	}

	tool := NewProcessVideoTool(builder, workSafe)

	args := json.RawMessage(fmt.Sprintf(`{
		"input": %q,
		"output": "test_output.mp4",
		"codec": "libx264",
		"fps": 10,
		"duration": "1s"
	}`, inputFile))

	result, execErr := tool.Execute(context.Background(), args)
	if execErr != nil {
		t.Fatalf("Execute: %v", execErr)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["status"] != "processed" {
		t.Errorf("status=%v, want processed", m["status"])
	}

	// Verify output file exists
	outputPath := filepath.Join(workDir, "test_output.mp4")
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}
}

// Task 6.14: Input must be within workDir.

func TestProcessVideoTool_Execute_RejectsPathOutsideWorkDir(t *testing.T) {
	workDir := t.TempDir()
	workSafe := testSafeDir(t, workDir)
	outsideDir := t.TempDir()

	// Create a file outside the workDir
	outsideFile := filepath.Join(outsideDir, "secret.mp4")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	builder, err := NewFFmpegBuilder("true", []safepath.SafePath{workSafe}, []safepath.SafePath{workSafe})
	if err != nil {
		t.Fatal(err)
	}

	tool := NewProcessVideoTool(builder, workSafe)

	args := json.RawMessage(fmt.Sprintf(`{
		"input": %q,
		"output": "out.mp4",
		"codec": "libx264"
	}`, outsideFile))

	_, execErr := tool.Execute(context.Background(), args)
	if execErr == nil {
		t.Fatal("expected error for input path outside workDir")
	}
	if !contains(execErr.Error(), "path") {
		t.Errorf("expected path-related error, got: %v", execErr)
	}
}

func TestProcessVideoTool_Execute_BadArgs(t *testing.T) {
	workDir := t.TempDir()
	workSafe := testSafeDir(t, workDir)

	builder, err := NewFFmpegBuilder("true", []safepath.SafePath{workSafe}, []safepath.SafePath{workSafe})
	if err != nil {
		t.Fatal(err)
	}

	tool := NewProcessVideoTool(builder, workSafe)

	tests := []struct {
		name string
		args string
	}{
		{"invalid json", `{not json`},
		{"empty input", `{"input": "", "output": "out.mp4", "codec": "libx264"}`},
		{"empty output", `{"input": "/some/path", "output": "", "codec": "libx264"}`},
		{"empty codec", `{"input": "/some/path", "output": "out.mp4", "codec": ""}`},
		{"missing fields", `{}`},
		{"output with path traversal", `{"input": "/some/path", "output": "../../../etc/evil.mp4", "codec": "libx264"}`},
		{"bad duration", `{"input": "/some/path", "output": "out.mp4", "codec": "libx264", "duration": "notaduration"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, execErr := tool.Execute(context.Background(), json.RawMessage(tc.args))
			if execErr == nil {
				t.Errorf("expected error for args: %s", tc.args)
			}
		})
	}
}

func TestProcessVideoTool_Execute_RejectsUnknownCodec(t *testing.T) {
	workDir := t.TempDir()
	workSafe := testSafeDir(t, workDir)

	// Create an input file in workDir
	inputFile := filepath.Join(workDir, "input.mp4")
	if err := os.WriteFile(inputFile, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	builder, err := NewFFmpegBuilder("true", []safepath.SafePath{workSafe}, []safepath.SafePath{workSafe})
	if err != nil {
		t.Fatal(err)
	}

	tool := NewProcessVideoTool(builder, workSafe)

	args := json.RawMessage(fmt.Sprintf(`{
		"input": %q,
		"output": "out.mp4",
		"codec": "evil_codec"
	}`, inputFile))

	_, execErr := tool.Execute(context.Background(), args)
	if execErr == nil {
		t.Fatal("expected error for unknown codec")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := range s {
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
