// Package nvs defines the interface contracts for the Nexus Vision System.
// This file is a design artifact -- it defines the types and interfaces that
// the Implementation Engineer must follow. It is NOT production code.
//
// See nvs-architecture.md for full context and rationale.
// See ADR-NVS-001 and ADR-NVS-002 for key design decisions.
package nvs

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"time"
)

// ---------------------------------------------------------------------------
// Frame and Ring Buffer
// ---------------------------------------------------------------------------

// Frame is a single captured and encoded video frame.
type Frame struct {
	Data      []byte    // JPEG-encoded image data
	Timestamp time.Time // capture timestamp (monotonic clock preferred)
	SeqNum    uint64    // monotonically increasing sequence number
}

// RingBuffer is a fixed-capacity circular buffer of Frames.
// Contract: single-writer (capture goroutine), multiple-reader (socket server).
type RingBuffer interface {
	// Write adds a frame to the buffer, overwriting the oldest if full.
	Write(f Frame)

	// Latest returns the most recently written frame.
	// Returns error if the buffer is empty (no frames captured yet).
	Latest() (Frame, error)

	// Since returns all frames captured after the given timestamp, up to limit.
	// Used by --clip: startTime = time.Now().Add(-clipDuration).
	Since(t time.Time, limit int) []Frame

	// Depth returns the number of frames currently in the buffer.
	Depth() int

	// Capacity returns the maximum number of frames the buffer can hold.
	Capacity() int
}

// ---------------------------------------------------------------------------
// Camera Capture
// ---------------------------------------------------------------------------

// Resolution represents a video resolution.
type Resolution struct {
	Width  int
	Height int
}

// CaptureConfig configures the ffmpeg capture subprocess.
// All fields must come from validated config -- never from LLM or user input.
type CaptureConfig struct {
	Device     string     // "/dev/video0" -- from config only
	Resolution Resolution // from config
	FPS        int        // from config
	Quality    int        // JPEG quality for mjpeg output (1-31, lower=better)
}

// FrameSource produces a stream of JPEG frames.
// Implementations: ffmpeg subprocess (v4l2), GoPro HTTP stream, test mock.
type FrameSource interface {
	// Start begins capturing frames. The returned reader yields MJPEG data.
	// Closing the reader terminates the capture.
	Start(ctx context.Context) (io.ReadCloser, error)

	// DeviceInfo returns human-readable info about the capture source.
	DeviceInfo() string
}

// ---------------------------------------------------------------------------
// Daemon Protocol
// ---------------------------------------------------------------------------

// Command constants for the Unix socket protocol.
// Clients send one of these followed by '\n'.
// The daemon responds with length-prefixed binary data.
const (
	CmdFrame  = "FRAME\n"  // Request single latest JPEG frame
	CmdStream = "STREAM\n" // Begin continuous MJPEG frame delivery
	CmdStatus = "STATUS\n" // Request daemon status as JSON
	CmdClip   = "CLIP"     // "CLIP <duration_ms>\n" -- extract from buffer
)

// DaemonStatus is the JSON payload returned by the STATUS command.
type DaemonStatus struct {
	Running        bool      `json:"running"`
	Device         string    `json:"device"`
	Resolution     string    `json:"resolution"`
	FPS            int       `json:"fps"`
	ActualFPS      float64   `json:"actual_fps"`
	BufferDepth    int       `json:"buffer_depth"`
	BufferCapacity int       `json:"buffer_capacity"`
	FramesCaptured uint64    `json:"frames_captured"`
	FramesDropped  uint64    `json:"frames_dropped"`
	ActiveClients  int       `json:"active_clients"`
	UptimeSeconds  float64   `json:"uptime_seconds"`
	LastFrameAt    time.Time `json:"last_frame_at"`
}

// ---------------------------------------------------------------------------
// Vision Client (consumer-side)
// ---------------------------------------------------------------------------

// VisionClient communicates with the vision daemon over the Unix socket.
// @see ADR-NVS-002 for protocol design rationale.
type VisionClient interface {
	// Frame requests a single JPEG frame from the daemon.
	Frame() ([]byte, error)

	// Stream returns a channel that delivers JPEG frames continuously.
	// The channel is closed when ctx is cancelled or the connection drops.
	Stream(ctx context.Context) (<-chan []byte, error)

	// Status requests daemon health information.
	Status() (*DaemonStatus, error)

	// Clip requests frames from the last `duration` of the ring buffer.
	// Returns the frames as a slice of JPEG byte slices.
	Clip(duration time.Duration) ([][]byte, error)

	// Close closes the connection to the daemon.
	Close() error
}

// ---------------------------------------------------------------------------
// Vision Daemon (producer-side)
// ---------------------------------------------------------------------------

// DaemonConfig configures the vision daemon.
type DaemonConfig struct {
	Device      string     // Camera device path (validated at config load)
	Resolution  Resolution // Capture resolution
	FPS         int        // Target framerate
	JPEGQuality int        // JPEG compression quality (1-31)
	BufferDepth int        // Ring buffer capacity in frames
	SocketPath  string     // Unix socket path (e.g., "/run/qraft/vision.sock")
}

// VisionDaemon is the long-lived camera capture service.
// @see ADR-NVS-001 for process model rationale.
type VisionDaemon interface {
	// Run starts the daemon and blocks until ctx is cancelled or a fatal error occurs.
	// This is the main entry point called by the `qraft vision daemon` command.
	Run(ctx context.Context) error

	// Metrics returns current operational metrics.
	Metrics() DaemonMetrics
}

// DaemonMetrics tracks operational metrics for observability.
type DaemonMetrics struct {
	FramesCaptured uint64        `json:"frames_captured"`
	FramesDropped  uint64        `json:"frames_dropped"`
	ActiveClients  int           `json:"active_clients"`
	CaptureLatency time.Duration `json:"capture_latency_us"`
	EncodeLatency  time.Duration `json:"encode_latency_us"`
	Uptime         time.Duration `json:"uptime"`
}

// ---------------------------------------------------------------------------
// Recording
// ---------------------------------------------------------------------------

// RecordConfig configures a recording session.
// All paths must be validated SafePaths (per security finding S9).
type RecordConfig struct {
	OutputDir    string // validated against SafePath
	OutputFormat string // "mp4" (default)
	Codec        string // "h264_vaapi" or "hevc_vaapi"
	RenderDevice string // "/dev/dri/renderD128" -- from config only
	Quality      int    // VA-API quality parameter (1-51, default 23)
}

// RecordingStatus is the JSON-serializable state of a recording.
// Persisted to ~/.qraftworx/run/recording.json for cross-process access.
type RecordingStatus struct {
	Active     bool      `json:"active"`
	OutputPath string    `json:"output_path"`
	StartedAt  time.Time `json:"started_at"`
	Duration   string    `json:"duration"`
	PID        int       `json:"pid"`
	Source     string    `json:"source"` // "daemon" or "gopro"
}

// Recorder manages recording lifecycle.
type Recorder interface {
	// Start begins recording frames from the vision daemon to a file.
	Start(ctx context.Context, cfg RecordConfig) error

	// Stop gracefully terminates the active recording.
	Stop() error

	// Status returns the current recording state.
	Status() (*RecordingStatus, error)
}

// ---------------------------------------------------------------------------
// Streaming
// ---------------------------------------------------------------------------

// StreamConfig configures the MJPEG stream server.
type StreamConfig struct {
	BindAddr string // e.g., "100.x.y.z:8080" or "127.0.0.1:8080"
	Port     int    // e.g., 8080
}

// StreamStatus is the JSON-serializable state of the stream server.
// Persisted to ~/.qraftworx/run/stream.json for cross-process access.
type StreamStatus struct {
	Active    bool      `json:"active"`
	PID       int       `json:"pid"`
	Port      int       `json:"port"`
	BindAddr  string    `json:"bind_addr"`
	StartedAt time.Time `json:"started_at"`
	Clients   int       `json:"clients"`
}

// StreamServer serves MJPEG frames over HTTP.
type StreamServer interface {
	// Start begins serving the MJPEG stream. Blocks until ctx is cancelled.
	Start(ctx context.Context, cfg StreamConfig) error

	// Status returns the current stream server state.
	Status() StreamStatus
}

// ---------------------------------------------------------------------------
// Hydrator Extension (additive to existing HydratedContext)
// ---------------------------------------------------------------------------

// ImageAttachment represents an image to include in a Gemini multimodal prompt.
// Added to the existing HydratedContext.Images field.
type ImageAttachment struct {
	Data     []byte // JPEG bytes
	MIMEType string // "image/jpeg"
	Label    string // human-readable label for logging
}

// ---------------------------------------------------------------------------
// Event Triggers
// ---------------------------------------------------------------------------

// Trigger defines a condition-action pair for event-driven automation.
type Trigger struct {
	Name      string           `toml:"name"`
	Enabled   bool             `toml:"enabled"`
	Condition TriggerCondition `toml:"condition"`
	Action    TriggerAction    `toml:"action"`
}

// TriggerCondition defines what event activates the trigger.
type TriggerCondition struct {
	Source     string `toml:"source"`     // "mqtt" | "http_poll"
	Topic      string `toml:"topic"`      // MQTT topic pattern
	Expression string `toml:"expression"` // JSONPath into the payload
	From       string `toml:"from"`       // previous value (state transition)
	To         string `toml:"to"`         // new value (state transition)
}

// TriggerAction defines what happens when the trigger fires.
// SECURITY: Command and Args come from config file only, never from MQTT payload.
type TriggerAction struct {
	Command string   `toml:"command"` // qraft subcommand
	Args    []string `toml:"args"`    // arguments to pass
}

// EventWatcher monitors event sources and fires triggers.
type EventWatcher interface {
	// Run starts watching for events. Blocks until ctx is cancelled.
	Run(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// Media Retention
// ---------------------------------------------------------------------------

// RetentionConfig defines retention rules for media files.
type RetentionConfig struct {
	RawMaxAge       time.Duration `toml:"raw_max_age"`
	ClipsMaxAge     time.Duration `toml:"clips_max_age"`
	TimelapseMaxAge time.Duration `toml:"timelapse_max_age"`
	CapturesMaxAge  time.Duration `toml:"captures_max_age"`
	MaxTotalSizeGB  int           `toml:"max_total_size_gb"`
	WarnAtPercent   int           `toml:"warn_at_percent"`
}

// CleanupReport summarizes a retention cleanup operation.
type CleanupReport struct {
	FilesDeleted   int            `json:"files_deleted"`
	BytesReclaimed int64          `json:"bytes_reclaimed"`
	RemainingBytes int64          `json:"remaining_bytes"`
	ByCategory     map[string]int `json:"by_category"`
}

// MediaRetention manages disk space for media files.
type MediaRetention interface {
	// Cleanup scans the vault directory and removes files exceeding retention limits.
	Cleanup() (*CleanupReport, error)

	// DiskUsage returns current vault size and percentage of configured cap.
	DiskUsage() (usedBytes int64, percentOfCap float64, err error)
}

// ---------------------------------------------------------------------------
// Tailscale Detection
// ---------------------------------------------------------------------------

// TailscaleInfo represents the state of the Tailscale connection.
type TailscaleInfo struct {
	Connected bool   // Whether Tailscale is running and connected
	IP        string // Tailscale IP address (empty if not connected)
	Hostname  string // Tailscale hostname (empty if not connected)
}

// DetectTailscale checks if Tailscale is running and returns connection info.
// This is a best-effort check -- if tailscale CLI is not installed, returns not connected.
func DetectTailscale() TailscaleInfo {
	// Implementation: exec tailscale status --json, parse response
	panic("interface definition only -- implementation in internal/network/tailscale.go")
}

// ---------------------------------------------------------------------------
// Tool Registration (additive to existing Tool interface)
// ---------------------------------------------------------------------------

// CaptureFrameToolConfig configures the capture_frame tool registered with Gemini.
type CaptureFrameToolConfig struct {
	VisionSocketPath string // Unix socket to connect to daemon
	CaptureDir       string // Directory to save captured frames (validated SafePath)
}

// CaptureFrameResult is returned by the capture_frame tool.
// This is what Gemini sees as the tool response.
type CaptureFrameResult struct {
	FilePath  string    `json:"file_path"`
	Timestamp time.Time `json:"timestamp"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	SizeBytes int       `json:"size_bytes"`
}

// Compile-time interface satisfaction checks would go here in production code:
// var _ VisionClient = (*visionClientImpl)(nil)
// var _ VisionDaemon = (*visionDaemonImpl)(nil)
// var _ Recorder = (*recorderImpl)(nil)
// var _ StreamServer = (*streamServerImpl)(nil)
// var _ EventWatcher = (*eventWatcherImpl)(nil)
// var _ MediaRetention = (*mediaRetentionImpl)(nil)

// Unused imports suppressed for design artifact.
var (
	_ = context.Background
	_ = json.Marshal
	_ io.ReadCloser
	_ net.Conn
	_ = time.Now
)
