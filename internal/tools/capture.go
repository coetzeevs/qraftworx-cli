package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// CaptureMediaTool captures a frame or short video using ffmpeg.
// Device path comes from config only, NEVER from LLM args (S2).
type CaptureMediaTool struct {
	builder    *FFmpegBuilder
	workDir    safepath.SafePath // validated output directory
	devicePath safepath.SafePath // device from config, not from args
}

// NewCaptureMediaTool creates a CaptureMediaTool.
func NewCaptureMediaTool(builder *FFmpegBuilder, workDir, devicePath safepath.SafePath) *CaptureMediaTool {
	return &CaptureMediaTool{
		builder:    builder,
		workDir:    workDir,
		devicePath: devicePath,
	}
}

func (t *CaptureMediaTool) Name() string { return "capture_media" }
func (t *CaptureMediaTool) Description() string {
	return "Capture a frame or short video from a media device"
}
func (t *CaptureMediaTool) Parameters() map[string]any {
	return map[string]any{
		"filename": map[string]any{
			"type":        "STRING",
			"description": "output filename (placed in work directory)",
			"required":    true,
		},
	}
}

// RequiresConfirmation returns true (hardware + media capture).
func (t *CaptureMediaTool) RequiresConfirmation() bool { return true }

// Permissions declares hardware and media capture capabilities.
func (t *CaptureMediaTool) Permissions() ToolPermission {
	return ToolPermission{Hardware: true, MediaCapture: true, FileSystem: true}
}

type captureArgs struct {
	Filename string `json:"filename"`
}

// Execute captures a single frame to the work directory.
func (t *CaptureMediaTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	var a captureArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("capture_media: invalid args: %w", err)
	}
	if a.Filename == "" {
		return nil, fmt.Errorf("capture_media: filename is required")
	}

	// Prevent directory traversal in filename
	if filepath.Base(a.Filename) != a.Filename {
		return nil, fmt.Errorf("capture_media: filename must not contain path separators")
	}

	// Build output path within workDir
	outputRaw := filepath.Join(t.workDir.String(), a.Filename)
	output, err := safepath.NewOutput(outputRaw, []string{t.workDir.String()})
	if err != nil {
		return nil, fmt.Errorf("capture_media: output path validation: %w", err)
	}

	// Build and run the ffmpeg capture command
	cmd := t.builder.CaptureFrame(ctx, t.devicePath, output)
	if cmdErr := cmd.Run(); cmdErr != nil {
		return nil, fmt.Errorf("capture_media: ffmpeg: %w", cmdErr)
	}

	// Verify output file exists
	info, err := os.Stat(output.String())
	if err != nil {
		return nil, fmt.Errorf("capture_media: output file not created: %w", err)
	}

	return map[string]any{
		"status":   "captured",
		"path":     output.String(),
		"size":     info.Size(),
		"captured": time.Now().UTC().Format(time.RFC3339),
	}, nil
}
