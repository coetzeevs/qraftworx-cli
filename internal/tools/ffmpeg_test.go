package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// testSafePath creates a SafePath for testing. Creates the file/dir if needed.
func testSafePath(t *testing.T, base, name string) safepath.SafePath {
	t.Helper()
	full := filepath.Join(base, name)
	// Ensure parent dir exists
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create the file if it doesn't exist
	if _, err := os.Stat(full); os.IsNotExist(err) {
		if err := os.WriteFile(full, []byte("test"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	sp, err := safepath.New(full, []string{base})
	if err != nil {
		t.Fatalf("safepath.New(%q, [%q]): %v", full, base, err)
	}
	return sp
}

// testSafeDir creates a SafePath for a directory.
func testSafeDir(t *testing.T, dir string) safepath.SafePath {
	t.Helper()
	sp, err := safepath.New(dir, []string{dir})
	if err != nil {
		t.Fatalf("safepath.New(%q): %v", dir, err)
	}
	return sp
}

// Task 6.1: NewFFmpegBuilder validates binary exists.

func TestNewFFmpegBuilder_ValidBinary(t *testing.T) {
	// Use a known binary that exists on all systems
	inDir := t.TempDir()
	outDir := t.TempDir()
	inSafe := testSafeDir(t, inDir)
	outSafe := testSafeDir(t, outDir)

	// "true" exists on all POSIX systems
	builder, err := NewFFmpegBuilder("true", []safepath.SafePath{inSafe}, []safepath.SafePath{outSafe})
	if err != nil {
		t.Fatalf("expected valid binary to succeed, got: %v", err)
	}
	if builder == nil {
		t.Fatal("expected non-nil builder")
	}
}

func TestNewFFmpegBuilder_MissingBinary(t *testing.T) {
	inDir := t.TempDir()
	outDir := t.TempDir()
	inSafe := testSafeDir(t, inDir)
	outSafe := testSafeDir(t, outDir)

	_, err := NewFFmpegBuilder("/nonexistent/binary/ffmpeg_not_here", []safepath.SafePath{inSafe}, []safepath.SafePath{outSafe})
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

// Task 6.2: CaptureFrame correct arg slice.

func TestFFmpegBuilder_CaptureFrame_ArgSlice(t *testing.T) {
	inDir := t.TempDir()
	outDir := t.TempDir()
	inSafe := testSafeDir(t, inDir)
	outSafe := testSafeDir(t, outDir)

	builder, err := NewFFmpegBuilder("true", []safepath.SafePath{inSafe}, []safepath.SafePath{outSafe})
	if err != nil {
		t.Fatal(err)
	}

	device := testSafePath(t, inDir, "video0")
	output := testSafePath(t, outDir, "frame.jpg")

	cmd := builder.CaptureFrame(context.Background(), device, output)

	// Verify args are separate elements (not concatenated shell string)
	args := cmd.Args
	if len(args) < 8 {
		t.Fatalf("expected at least 8 args, got %d: %v", len(args), args)
	}

	// Verify specific arg positions
	// Args[0] is the binary, then -y, -f, v4l2, -i, device, -frames:v, 1, output
	foundY := false
	foundF := false
	foundI := false
	foundFrames := false

	for i, arg := range args {
		switch arg {
		case "-y":
			foundY = true
		case "-f":
			if i+1 < len(args) && args[i+1] == "v4l2" {
				foundF = true
			}
		case "-i":
			if i+1 < len(args) && args[i+1] == device.String() {
				foundI = true
			}
		case "-frames:v":
			if i+1 < len(args) && args[i+1] == "1" {
				foundFrames = true
			}
		}
	}

	if !foundY {
		t.Error("missing -y flag")
	}
	if !foundF {
		t.Error("missing -f v4l2")
	}
	if !foundI {
		t.Errorf("missing -i with device path %s", device.String())
	}
	if !foundFrames {
		t.Error("missing -frames:v 1")
	}

	// Last arg should be output path
	if args[len(args)-1] != output.String() {
		t.Errorf("last arg=%q, want output path %q", args[len(args)-1], output.String())
	}

	// Verify no shell interpolation -- no arg should contain spaces
	// (paths are separate elements, not "sh -c ffmpeg ...")
	for _, arg := range args {
		if strings.Contains(arg, "sh -c") {
			t.Errorf("found shell invocation in arg: %q", arg)
		}
	}
}

// Task 6.3: CaptureFrame only SafePath accepted (compile-time).
// This test verifies that the method signature requires SafePath.
// If someone tried to pass a raw string, it would fail to compile.

func TestFFmpegBuilder_CaptureFrame_RequiresSafePath(t *testing.T) {
	// This is a compile-time enforcement test.
	// The method signature func (b *FFmpegBuilder) CaptureFrame(ctx context.Context, device safepath.SafePath, output safepath.SafePath)
	// enforces that only SafePath values can be passed.
	// If this test compiles, SafePath enforcement is working.

	inDir := t.TempDir()
	outDir := t.TempDir()
	inSafe := testSafeDir(t, inDir)
	outSafe := testSafeDir(t, outDir)

	builder, err := NewFFmpegBuilder("true", []safepath.SafePath{inSafe}, []safepath.SafePath{outSafe})
	if err != nil {
		t.Fatal(err)
	}

	device := testSafePath(t, inDir, "device")
	output := testSafePath(t, outDir, "out.jpg")

	// The key assertion: this compiles only because device and output are SafePath.
	// A raw string would cause a compile error.
	cmd := builder.CaptureFrame(context.Background(), device, output)
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
}

// Task 6.4: Transcode clamps FPS.

func TestFFmpegBuilder_Transcode_ClampsFPS(t *testing.T) {
	inDir := t.TempDir()
	outDir := t.TempDir()
	inSafe := testSafeDir(t, inDir)
	outSafe := testSafeDir(t, outDir)

	builder, err := NewFFmpegBuilder("true", []safepath.SafePath{inSafe}, []safepath.SafePath{outSafe})
	if err != nil {
		t.Fatal(err)
	}

	input := testSafePath(t, inDir, "input.mp4")
	output := testSafePath(t, outDir, "output.mp4")

	tests := []struct {
		name     string
		fps      int
		expected string // expected -r value in args
	}{
		{"too high", 120, "60"},
		{"too low", 0, "1"},
		{"negative", -5, "1"},
		{"normal", 30, "30"},
		{"max boundary", 60, "60"},
		{"min boundary", 1, "1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := TranscodeOpts{
				Codec:    "libx264",
				FPS:      tc.fps,
				Duration: 10 * time.Second,
			}
			cmd, cmdErr := builder.Transcode(context.Background(), input, output, opts)
			if cmdErr != nil {
				t.Fatal(cmdErr)
			}

			// Find -r flag and its value
			args := cmd.Args
			found := false
			for i, arg := range args {
				if arg == "-r" && i+1 < len(args) {
					if args[i+1] != tc.expected {
						t.Errorf("FPS %d: -r value=%q, want %q", tc.fps, args[i+1], tc.expected)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("FPS %d: -r flag not found in args: %v", tc.fps, args)
			}
		})
	}
}

// Task 6.5: Transcode validates codec allowlist.

func TestFFmpegBuilder_Transcode_RejectsUnknownCodec(t *testing.T) {
	inDir := t.TempDir()
	outDir := t.TempDir()
	inSafe := testSafeDir(t, inDir)
	outSafe := testSafeDir(t, outDir)

	builder, err := NewFFmpegBuilder("true", []safepath.SafePath{inSafe}, []safepath.SafePath{outSafe})
	if err != nil {
		t.Fatal(err)
	}

	input := testSafePath(t, inDir, "input.mp4")
	output := testSafePath(t, outDir, "output.mp4")

	badCodecs := []string{"evil_codec", "\"; rm -rf /", "libx264; cat /etc/passwd", "unknown", ""}
	for _, codec := range badCodecs {
		t.Run(codec, func(t *testing.T) {
			opts := TranscodeOpts{
				Codec:    codec,
				FPS:      30,
				Duration: 10 * time.Second,
			}
			_, cmdErr := builder.Transcode(context.Background(), input, output, opts)
			if cmdErr == nil {
				t.Errorf("expected error for codec %q", codec)
			}
		})
	}

	// Verify allowed codecs work
	for codec := range allowedCodecs {
		t.Run("allowed_"+codec, func(t *testing.T) {
			opts := TranscodeOpts{
				Codec:    codec,
				FPS:      30,
				Duration: 10 * time.Second,
			}
			cmd, cmdErr := builder.Transcode(context.Background(), input, output, opts)
			if cmdErr != nil {
				t.Errorf("expected codec %q to be allowed, got: %v", codec, cmdErr)
			}
			if cmd == nil {
				t.Errorf("expected non-nil cmd for allowed codec %q", codec)
			}
		})
	}
}

// Task 6.6: Transcode clamps duration.

func TestFFmpegBuilder_Transcode_ClampsDuration(t *testing.T) {
	inDir := t.TempDir()
	outDir := t.TempDir()
	inSafe := testSafeDir(t, inDir)
	outSafe := testSafeDir(t, outDir)

	builder, err := NewFFmpegBuilder("true", []safepath.SafePath{inSafe}, []safepath.SafePath{outSafe})
	if err != nil {
		t.Fatal(err)
	}

	input := testSafePath(t, inDir, "input.mp4")
	output := testSafePath(t, outDir, "output.mp4")

	tests := []struct {
		name     string
		duration time.Duration
		expected string // expected -t value
	}{
		{"over 24h", 25 * time.Hour, "86400.000"},
		{"exactly 24h", 24 * time.Hour, "86400.000"},
		{"normal", 30 * time.Second, "30.000"},
		{"zero", 0, "0.001"}, // clamped to minDuration (1ms)
		{"negative", -5 * time.Second, "0.001"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := TranscodeOpts{
				Codec:    "libx264",
				FPS:      30,
				Duration: tc.duration,
			}
			cmd, cmdErr := builder.Transcode(context.Background(), input, output, opts)
			if cmdErr != nil {
				t.Fatal(cmdErr)
			}

			args := cmd.Args
			found := false
			for i, arg := range args {
				if arg == "-t" && i+1 < len(args) {
					if args[i+1] != tc.expected {
						t.Errorf("duration %v: -t value=%q, want %q", tc.duration, args[i+1], tc.expected)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("duration %v: -t flag not found in args: %v", tc.duration, args)
			}
		})
	}
}

// Task 6.7: No sh -c anywhere.

func TestFFmpegBuilder_NoShellInvocation(t *testing.T) {
	inDir := t.TempDir()
	outDir := t.TempDir()
	inSafe := testSafeDir(t, inDir)
	outSafe := testSafeDir(t, outDir)

	builder, err := NewFFmpegBuilder("true", []safepath.SafePath{inSafe}, []safepath.SafePath{outSafe})
	if err != nil {
		t.Fatal(err)
	}

	device := testSafePath(t, inDir, "video0")
	input := testSafePath(t, inDir, "input.mp4")
	output := testSafePath(t, outDir, "output.mp4")

	// Check CaptureFrame
	captureCmd := builder.CaptureFrame(context.Background(), device, output)
	for _, arg := range captureCmd.Args {
		if arg == "sh" || arg == "-c" || arg == "/bin/sh" || arg == "bash" || arg == "/bin/bash" {
			t.Errorf("CaptureFrame: found shell invocation arg %q", arg)
		}
	}
	// Verify the binary is not a shell
	if strings.Contains(captureCmd.Path, "/sh") || strings.Contains(captureCmd.Path, "/bash") {
		t.Errorf("CaptureFrame: binary path is a shell: %q", captureCmd.Path)
	}

	// Check Transcode
	opts := TranscodeOpts{
		Codec:    "libx264",
		FPS:      30,
		Duration: 10 * time.Second,
	}
	transcodeCmd, cmdErr := builder.Transcode(context.Background(), input, output, opts)
	if cmdErr != nil {
		t.Fatal(cmdErr)
	}
	for _, arg := range transcodeCmd.Args {
		if arg == "sh" || arg == "-c" || arg == "/bin/sh" || arg == "bash" || arg == "/bin/bash" {
			t.Errorf("Transcode: found shell invocation arg %q", arg)
		}
	}
	if strings.Contains(transcodeCmd.Path, "/sh") || strings.Contains(transcodeCmd.Path, "/bash") {
		t.Errorf("Transcode: binary path is a shell: %q", transcodeCmd.Path)
	}
}

// Additional helper tests.

func TestClampFPS(t *testing.T) {
	if ClampFPS(0) != 1 {
		t.Errorf("ClampFPS(0)=%d, want 1", ClampFPS(0))
	}
	if ClampFPS(100) != 60 {
		t.Errorf("ClampFPS(100)=%d, want 60", ClampFPS(100))
	}
	if ClampFPS(30) != 30 {
		t.Errorf("ClampFPS(30)=%d, want 30", ClampFPS(30))
	}
}

func TestClampDuration(t *testing.T) {
	if ClampDuration(0) != time.Millisecond {
		t.Errorf("ClampDuration(0)=%v, want 1ms", ClampDuration(0))
	}
	if ClampDuration(25*time.Hour) != 24*time.Hour {
		t.Errorf("ClampDuration(25h)=%v, want 24h", ClampDuration(25*time.Hour))
	}
	if ClampDuration(10*time.Second) != 10*time.Second {
		t.Errorf("ClampDuration(10s)=%v, want 10s", ClampDuration(10*time.Second))
	}
}

func TestValidateCodec(t *testing.T) {
	if !ValidateCodec("libx264") {
		t.Error("expected libx264 to be valid")
	}
	if ValidateCodec("evil_codec") {
		t.Error("expected evil_codec to be invalid")
	}
	if ValidateCodec("") {
		t.Error("expected empty string to be invalid")
	}
}

func TestFFmpegBuilder_Transcode_InvalidResolution(t *testing.T) {
	inDir := t.TempDir()
	outDir := t.TempDir()
	inSafe := testSafeDir(t, inDir)
	outSafe := testSafeDir(t, outDir)

	builder, err := NewFFmpegBuilder("true", []safepath.SafePath{inSafe}, []safepath.SafePath{outSafe})
	if err != nil {
		t.Fatal(err)
	}

	input := testSafePath(t, inDir, "input.mp4")
	output := testSafePath(t, outDir, "output.mp4")

	badResolutions := []string{"abc", "1920", "x1080", "1920x", "1920X1080", "not a resolution"}
	for _, res := range badResolutions {
		t.Run(res, func(t *testing.T) {
			opts := TranscodeOpts{
				Codec:      "libx264",
				Resolution: res,
				FPS:        30,
				Duration:   10 * time.Second,
			}
			_, cmdErr := builder.Transcode(context.Background(), input, output, opts)
			if cmdErr == nil {
				t.Errorf("expected error for resolution %q", res)
			}
		})
	}

	// Valid resolutions
	goodResolutions := []string{"1920x1080", "640x480", "1x1"}
	for _, res := range goodResolutions {
		t.Run("valid_"+res, func(t *testing.T) {
			opts := TranscodeOpts{
				Codec:      "libx264",
				Resolution: res,
				FPS:        30,
				Duration:   10 * time.Second,
			}
			cmd, cmdErr := builder.Transcode(context.Background(), input, output, opts)
			if cmdErr != nil {
				t.Errorf("expected resolution %q to be valid, got: %v", res, cmdErr)
			}
			// Verify -s flag is present
			found := false
			for i, arg := range cmd.Args {
				if arg == "-s" && i+1 < len(cmd.Args) && cmd.Args[i+1] == res {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected -s %s in args: %v", res, cmd.Args)
			}
		})
	}
}
