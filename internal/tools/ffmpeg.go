package tools

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// allowedCodecs is the set of codecs allowed for transcoding.
var allowedCodecs = map[string]bool{
	"libx264":  true,
	"libx265":  true,
	"libvpx":   true,
	"libaom":   true,
	"copy":     true,
	"aac":      true,
	"libopus":  true,
	"mjpeg":    true,
	"png":      true,
	"rawvideo": true,
}

const (
	minFPS      = 1
	maxFPS      = 60
	maxDuration = 24 * time.Hour
	minDuration = time.Millisecond
)

// resolutionPattern validates WxH format (e.g., "1920x1080").
var resolutionPattern = regexp.MustCompile(`^\d{1,5}x\d{1,5}$`)

// TranscodeOpts configures video transcoding. All values are clamped.
type TranscodeOpts struct {
	Codec      string        // validated against allowlist
	Resolution string        // validated WxH format
	FPS        int           // clamped to 1-60
	Duration   time.Duration // clamped to max 24h
}

// FFmpegBuilder constructs validated ffmpeg commands.
// All paths must be SafePath. All numeric params are typed and clamped.
// Never uses sh -c. Always exec.CommandContext with separate args (S2).
type FFmpegBuilder struct {
	binary     string   // validated at startup
	allowedIn  []string // allowed input base dirs
	allowedOut []string // allowed output base dirs
}

// NewFFmpegBuilder validates the ffmpeg binary exists and returns a builder.
func NewFFmpegBuilder(binaryPath string, allowedIn, allowedOut []safepath.SafePath) (*FFmpegBuilder, error) {
	resolved, err := exec.LookPath(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: binary not found at %q: %w", binaryPath, err)
	}

	inBases := make([]string, len(allowedIn))
	for i, p := range allowedIn {
		inBases[i] = p.String()
	}

	outBases := make([]string, len(allowedOut))
	for i, p := range allowedOut {
		outBases[i] = p.String()
	}

	return &FFmpegBuilder{
		binary:     resolved,
		allowedIn:  inBases,
		allowedOut: outBases,
	}, nil
}

// CaptureFrame builds an ffmpeg command to capture a single frame.
// Both device and output must be SafePath (compile-time enforcement).
func (b *FFmpegBuilder) CaptureFrame(ctx context.Context, device, output safepath.SafePath) *exec.Cmd {
	args := []string{
		"-y",
		"-f", "v4l2",
		"-i", device.String(),
		"-frames:v", "1",
		output.String(),
	}
	return exec.CommandContext(ctx, b.binary, args...) //nolint:gosec // G204: binary validated at startup, args are SafePath-typed
}

// Transcode builds an ffmpeg command to process a video file.
// Both input and output must be SafePath (compile-time enforcement).
// Returns error if codec is not in the allowlist or resolution format is invalid.
func (b *FFmpegBuilder) Transcode(ctx context.Context, input, output safepath.SafePath, opts TranscodeOpts) (*exec.Cmd, error) {
	// Validate codec against allowlist
	if !allowedCodecs[opts.Codec] {
		return nil, fmt.Errorf("ffmpeg: codec %q not in allowlist", opts.Codec)
	}

	// Validate resolution format if provided
	if opts.Resolution != "" && !resolutionPattern.MatchString(opts.Resolution) {
		return nil, fmt.Errorf("ffmpeg: invalid resolution format %q, expected WxH", opts.Resolution)
	}

	// Clamp FPS
	fps := opts.FPS
	if fps < minFPS {
		fps = minFPS
	}
	if fps > maxFPS {
		fps = maxFPS
	}

	// Clamp duration
	dur := opts.Duration
	if dur < minDuration {
		dur = minDuration
	}
	if dur > maxDuration {
		dur = maxDuration
	}

	args := []string{
		"-y",
		"-i", input.String(),
		"-c:v", opts.Codec,
		"-r", fmt.Sprintf("%d", fps),
		"-t", fmt.Sprintf("%.3f", dur.Seconds()),
	}

	if opts.Resolution != "" {
		args = append(args, "-s", opts.Resolution)
	}

	args = append(args, output.String())

	return exec.CommandContext(ctx, b.binary, args...), nil //nolint:gosec // G204: binary validated at startup, args are SafePath-typed and clamped
}

// ValidateCodec checks if a codec is in the allowlist.
func ValidateCodec(codec string) bool {
	return allowedCodecs[codec]
}

// ClampFPS clamps an FPS value to the valid range [1, 60].
func ClampFPS(fps int) int {
	if fps < minFPS {
		return minFPS
	}
	if fps > maxFPS {
		return maxFPS
	}
	return fps
}

// ClampDuration clamps a duration to the valid range [1ms, 24h].
func ClampDuration(d time.Duration) time.Duration {
	if d < minDuration {
		return minDuration
	}
	if d > maxDuration {
		return maxDuration
	}
	return d
}
