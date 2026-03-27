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

// ProcessVideoTool runs ffmpeg transcoding on an existing file.
type ProcessVideoTool struct {
	builder *FFmpegBuilder
	workDir safepath.SafePath
}

// NewProcessVideoTool creates a ProcessVideoTool.
func NewProcessVideoTool(builder *FFmpegBuilder, workDir safepath.SafePath) *ProcessVideoTool {
	return &ProcessVideoTool{
		builder: builder,
		workDir: workDir,
	}
}

func (t *ProcessVideoTool) Name() string        { return "process_video" }
func (t *ProcessVideoTool) Description() string { return "Transcode a video file using ffmpeg" }
func (t *ProcessVideoTool) Parameters() map[string]any {
	return map[string]any{
		"input": map[string]any{
			"type":        "STRING",
			"description": "input video file path (must be within work directory)",
			"required":    true,
		},
		"output": map[string]any{
			"type":        "STRING",
			"description": "output filename (placed in work directory)",
			"required":    true,
		},
		"codec": map[string]any{
			"type":        "STRING",
			"description": "video codec (e.g., libx264, libx265, copy)",
			"required":    true,
		},
		"resolution": map[string]any{
			"type":        "STRING",
			"description": "output resolution WxH (e.g., 1920x1080)",
		},
		"fps": map[string]any{
			"type":        "NUMBER",
			"description": "frames per second (clamped to 1-60)",
		},
		"duration": map[string]any{
			"type":        "STRING",
			"description": "maximum duration (e.g., 30s, 5m, 1h)",
		},
	}
}

// RequiresConfirmation returns false (no hardware, no upload).
func (t *ProcessVideoTool) RequiresConfirmation() bool { return false }

// Permissions declares filesystem access.
func (t *ProcessVideoTool) Permissions() ToolPermission {
	return ToolPermission{FileSystem: true}
}

type processVideoArgs struct {
	Input      string `json:"input"`
	Output     string `json:"output"`
	Codec      string `json:"codec"`
	Resolution string `json:"resolution"`
	FPS        int    `json:"fps"`
	Duration   string `json:"duration"`
}

// Execute transcodes a video file.
func (t *ProcessVideoTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	var a processVideoArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("process_video: invalid args: %w", err)
	}
	if a.Input == "" {
		return nil, fmt.Errorf("process_video: input is required")
	}
	if a.Output == "" {
		return nil, fmt.Errorf("process_video: output is required")
	}
	if a.Codec == "" {
		return nil, fmt.Errorf("process_video: codec is required")
	}

	// Validate input path is within workDir
	inputPath, err := safepath.New(a.Input, []string{t.workDir.String()})
	if err != nil {
		return nil, fmt.Errorf("process_video: input path validation: %w", err)
	}

	// Prevent directory traversal in output filename
	if filepath.Base(a.Output) != a.Output {
		return nil, fmt.Errorf("process_video: output must be a filename, not a path")
	}

	// Build output path within workDir
	outputRaw := filepath.Join(t.workDir.String(), a.Output)
	outputPath, err := safepath.New(outputRaw, []string{t.workDir.String()})
	if err != nil {
		return nil, fmt.Errorf("process_video: output path validation: %w", err)
	}

	// Parse duration if provided
	dur := 10 * time.Second // default
	if a.Duration != "" {
		parsed, parseErr := time.ParseDuration(a.Duration)
		if parseErr != nil {
			return nil, fmt.Errorf("process_video: invalid duration %q: %w", a.Duration, parseErr)
		}
		dur = parsed
	}

	// Default FPS if not provided
	fps := a.FPS
	if fps == 0 {
		fps = 30
	}

	opts := TranscodeOpts{
		Codec:      a.Codec,
		Resolution: a.Resolution,
		FPS:        fps,
		Duration:   dur,
	}

	cmd, err := t.builder.Transcode(ctx, inputPath, outputPath, opts)
	if err != nil {
		return nil, fmt.Errorf("process_video: %w", err)
	}

	if cmdErr := cmd.Run(); cmdErr != nil {
		return nil, fmt.Errorf("process_video: ffmpeg: %w", cmdErr)
	}

	// Verify output
	info, err := os.Stat(outputPath.String())
	if err != nil {
		return nil, fmt.Errorf("process_video: output file not created: %w", err)
	}

	return map[string]any{
		"status":    "processed",
		"input":     inputPath.String(),
		"output":    outputPath.String(),
		"size":      info.Size(),
		"codec":     a.Codec,
		"processed": time.Now().UTC().Format(time.RFC3339),
	}, nil
}
