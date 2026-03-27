package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// Task 6.15: ffmpeg integration test using synthetic source.
// Guarded by testing.Short() -- skipped in short mode.

func TestFFmpeg_Integration_CaptureWithTestSrc(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ffmpeg integration test in short mode")
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed, skipping integration test")
	}

	workDir := t.TempDir()
	workSafe := testSafeDir(t, workDir)

	builder, err := NewFFmpegBuilder(ffmpegPath, []safepath.SafePath{workSafe}, []safepath.SafePath{workSafe})
	if err != nil {
		t.Fatalf("NewFFmpegBuilder: %v", err)
	}

	t.Run("capture_frame_with_lavfi", func(t *testing.T) {
		// Generate a single frame using lavfi (no hardware needed)
		outputFile := filepath.Join(workDir, "integration_frame.png")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Use ffmpeg directly with lavfi to capture a frame
		cmd := exec.CommandContext(ctx, ffmpegPath, //nolint:gosec // G204: test with validated binary path
			"-f", "lavfi",
			"-i", "testsrc=duration=1:size=320x240:rate=1",
			"-frames:v", "1",
			"-y",
			outputFile,
		)
		out, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			t.Fatalf("ffmpeg capture frame: %v\n%s", cmdErr, out)
		}

		info, statErr := os.Stat(outputFile)
		if statErr != nil {
			t.Fatalf("output not created: %v", statErr)
		}
		if info.Size() == 0 {
			t.Error("output file is empty")
		}
	})

	t.Run("transcode_with_lavfi", func(t *testing.T) {
		// Generate a test video
		inputFile := filepath.Join(workDir, "integration_input.mp4")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		genCmd := exec.CommandContext(ctx, ffmpegPath, //nolint:gosec // G204: test with validated binary path
			"-f", "lavfi",
			"-i", "testsrc=duration=1:size=320x240:rate=10",
			"-c:v", "libx264",
			"-pix_fmt", "yuv420p",
			"-y",
			inputFile,
		)
		out, genErr := genCmd.CombinedOutput()
		if genErr != nil {
			t.Fatalf("generate test video: %v\n%s", genErr, out)
		}

		// Now transcode using the builder
		inputSafe, safeErr := safepath.New(inputFile, []string{workDir})
		if safeErr != nil {
			t.Fatalf("safepath input: %v", safeErr)
		}

		outputFile := filepath.Join(workDir, "integration_output.mp4")
		if err := os.WriteFile(outputFile, []byte{}, 0o600); err != nil {
			t.Fatal(err)
		}
		outputSafe, safeErr := safepath.New(outputFile, []string{workDir})
		if safeErr != nil {
			t.Fatalf("safepath output: %v", safeErr)
		}

		opts := TranscodeOpts{
			Codec:      "libx264",
			Resolution: "160x120",
			FPS:        10,
			Duration:   1 * time.Second,
		}

		transcodeCtx, transcodeCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer transcodeCancel()

		transcodeCmd, cmdErr := builder.Transcode(transcodeCtx, inputSafe, outputSafe, opts)
		if cmdErr != nil {
			t.Fatalf("Transcode: %v", cmdErr)
		}

		transcodeOut, runErr := transcodeCmd.CombinedOutput()
		if runErr != nil {
			t.Fatalf("transcode run: %v\n%s", runErr, transcodeOut)
		}

		info, statErr := os.Stat(outputFile)
		if statErr != nil {
			t.Fatalf("output not created: %v", statErr)
		}
		if info.Size() == 0 {
			t.Error("output file is empty")
		}
		t.Logf("transcoded: %d bytes", info.Size())
	})

	t.Run("arg_slice_integrity", func(t *testing.T) {
		// Verify that the builder produces proper arg slices even in integration
		inputFile := filepath.Join(workDir, "integration_input.mp4")
		inputSafe, safeErr := safepath.New(inputFile, []string{workDir})
		if safeErr != nil {
			t.Fatalf("safepath: %v", safeErr)
		}

		outputFile := filepath.Join(workDir, "integration_verify.mp4")
		if err := os.WriteFile(outputFile, []byte{}, 0o600); err != nil {
			t.Fatal(err)
		}
		outputSafe, safeErr := safepath.New(outputFile, []string{workDir})
		if safeErr != nil {
			t.Fatalf("safepath: %v", safeErr)
		}

		opts := TranscodeOpts{
			Codec:    "copy",
			FPS:      30,
			Duration: 1 * time.Second,
		}

		cmd, cmdErr := builder.Transcode(context.Background(), inputSafe, outputSafe, opts)
		if cmdErr != nil {
			t.Fatal(cmdErr)
		}

		// Every arg should be its own element
		for _, arg := range cmd.Args {
			if len(arg) > 200 {
				t.Errorf("suspiciously long arg (possible concatenated command): %.50s...", arg)
			}
		}

		// Binary should be the ffmpeg path
		if cmd.Path != ffmpegPath {
			t.Errorf("cmd.Path=%q, want %q", cmd.Path, ffmpegPath)
		}
	})
}
