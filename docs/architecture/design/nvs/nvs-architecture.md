# Nexus Vision System (NVS) Architecture Specification

**Status:** Proposed
**Date:** 2026-03-26
**Parent:** [qraft-architecture.md](../qraft-architecture.md)
**ADRs:** [ADR-NVS-001](./ADR-NVS-001-vision-daemon-process-model.md), [ADR-NVS-002](./ADR-NVS-002-frame-sharing-mechanism.md)

---

## 1. Overview

NVS adds a centralized camera service to Qraft with four capabilities:
- **Eye**: AI-powered visual snapshot analysis via Gemini multimodal
- **Chronicler**: Automated recording with hardware-accelerated encoding
- **Viewport**: Live MJPEG streaming for remote viewing
- **Timelapse/Clip**: Specialized capture modes (frame-by-frame, rolling buffer extraction)

NVS is an add-on to the existing Qraft architecture. It introduces one new long-lived process (the vision daemon) and several new CLI commands, tools, and configuration sections. It does not replace or modify the existing Gemini tool loop, hydrator, or executor.

---

## 2. System Architecture

```
                            +-----------------------+
                            |   systemd (manages)   |
                            +-----------+-----------+
                                        |
                                        v
+-----------------------------------------------------------------------+
|                    qraft vision daemon                                 |
|                    (long-lived process)                                |
|                                                                        |
|  +---------------+    +----------------+    +---------------------+   |
|  | Camera Reader |    | Frame Store    |    | Socket Server       |   |
|  |               |    |                |    |                     |   |
|  | v4l2/ffmpeg   |--->| Ring Buffer    |--->| /run/qraft/         |   |
|  | -> raw frames |    | (N frames,     |    | vision.sock         |   |
|  |               |    |  JPEG encoded) |    |                     |   |
|  +---------------+    +----------------+    | FRAME / STREAM /    |   |
|                                             | STATUS commands     |   |
+-----------------------------------------------------------------------+
        ^                       |                       |
        |                       |                       |
  /dev/video0              Unix Socket             Unix Socket
  (Logitech C920)              |                       |
                               |                       |
              +----------------+----------+------------+--------+
              |                |          |            |         |
              v                v          v            v         v
        +-----------+   +-----------+  +--------+  +------+  +-------+
        | qraft eye |   | qraft rec |  | qraft  |  |qraft |  |qraft  |
        |           |   |           |  | stream |  |rec   |  |time-  |
        | FRAME cmd |   | STREAM    |  | start  |  |--clip|  |lapse  |
        | -> Gemini |   | cmd ->    |  |        |  |      |  |       |
        | multimodal|   | ffmpeg    |  | STREAM |  |FRAME |  |FRAME  |
        |           |   | pipe      |  | -> HTTP|  |x N   |  |@ 1fps |
        +-----------+   +-----------+  +--------+  +------+  +-------+
              |                |              |
              v                v              v
        +-----------+   +-----------+  +-----------+
        | Gemini API|   | ~/qraft-  |  | HTTP :8080|
        | (visual   |   | worx/vault|  | (MJPEG    |
        | analysis) |   | /raw/     |  |  relay)   |
        +-----------+   +-----------+  +-----------+
```

### Key Design Principles

1. **Daemon owns the camera.** No other process opens `/dev/video0`. All frame access goes through the Unix socket.
2. **Consumers are short-lived or independently managed.** The CLI commands are thin clients that connect to the daemon socket, request data, and process it.
3. **ffmpeg is always a subprocess.** Go does not link against ffmpeg. All encoding/transcoding is via `exec.CommandContext` with argument slices -- never shell interpolation. (Per security finding S2 from the Qraft architecture review.)
4. **The daemon is stateless.** It can be restarted by systemd without data loss. Recording state, stream server state, and timelapse state are managed by the consumer processes, not the daemon.

---

## 3. Process Architecture (ADR-NVS-001)

### 3.1 Daemon Lifecycle

The daemon is started by systemd and runs as a single goroutine-based Go process:

```
qraft vision daemon [--device /dev/video0] [--resolution 1920x1080] [--fps 30]
```

Flags fall back to config file values, which fall back to compiled defaults.

**systemd unit file:**

```ini
# /etc/systemd/system/qraft-vision.service
[Unit]
Description=Qraft Vision Daemon
After=network.target

[Service]
Type=simple
User=qraftworx
ExecStart=/usr/local/bin/qraft vision daemon
Restart=on-failure
RestartSec=2s
RuntimeDirectory=qraft
RuntimeDirectoryMode=0750

# Resource limits
MemoryMax=512M
CPUQuota=50%

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/run/qraft /home/qraftworx/.qraftworx
PrivateTmp=true
DeviceAllow=/dev/video0 rw
DeviceAllow=/dev/dri/renderD128 rw

[Install]
WantedBy=multi-user.target
```

Key points:
- `RuntimeDirectory=qraft` creates `/run/qraft/` with correct permissions. The socket lives here.
- `DeviceAllow` restricts device access to exactly the camera and the Intel iGPU render node.
- `ProtectHome=read-only` prevents the daemon from writing outside its designated paths.
- `MemoryMax=512M` bounds the ring buffer. The daemon must respect this.

### 3.2 Daemon Internal Architecture

```go
// internal/vision/daemon.go

// Daemon is the long-lived vision capture process.
type Daemon struct {
    device     string           // e.g., "/dev/video0"
    resolution Resolution       // e.g., 1920x1080
    fps        int              // e.g., 30
    ringBuf    *RingBuffer      // circular buffer of JPEG frames
    socketPath string           // e.g., "/run/qraft/vision.sock"
    logger     *slog.Logger
    metrics    *DaemonMetrics

    mu         sync.RWMutex     // protects ringBuf writes
    cancel     context.CancelFunc
}

// Resolution represents a video resolution.
type Resolution struct {
    Width  int
    Height int
}

// DaemonMetrics tracks operational metrics.
type DaemonMetrics struct {
    FramesCaptured   atomic.Uint64
    FramesDropped    atomic.Uint64
    ActiveClients    atomic.Int32
    CaptureLatencyUs atomic.Int64  // last frame capture latency in microseconds
    EncodeLatencyUs  atomic.Int64  // last JPEG encode latency in microseconds
    UptimeStarted   time.Time
}
```

### 3.3 Daemon Startup Sequence

```
1. Parse flags + config
2. Validate device exists (stat /dev/video0)
3. Remove stale socket file if present
4. Open camera device via ffmpeg subprocess (v4l2 input)
5. Allocate ring buffer (configurable depth, default 300 frames = 10s at 30fps)
6. Start capture goroutine (reads raw frames, JPEG-encodes, writes to ring buffer)
7. Start socket server goroutine (listens on Unix socket, serves clients)
8. Start metrics goroutine (periodic health check, log camera fps)
9. Block on signal (SIGTERM, SIGINT) -> graceful shutdown
```

### 3.4 CLI Detection of Daemon State

Every CLI command that needs the daemon must check for it first:

```go
// internal/vision/client.go

// Connect establishes a connection to the vision daemon.
// Returns ErrDaemonNotRunning if the socket does not exist or is not listening.
func Connect(socketPath string) (*Client, error) {
    conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
    if err != nil {
        if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENOENT) {
            return nil, fmt.Errorf("%w: is 'qraft vision daemon' running? (socket: %s)", ErrDaemonNotRunning, socketPath)
        }
        return nil, fmt.Errorf("connecting to vision daemon: %w", err)
    }
    return &Client{conn: conn}, nil
}

var ErrDaemonNotRunning = errors.New("vision daemon not running")
```

---

## 4. Frame Sharing Mechanism (ADR-NVS-002)

### 4.1 Ring Buffer

The daemon maintains a fixed-size circular buffer of JPEG-encoded frames in memory.

```go
// internal/vision/ringbuf.go

// Frame is a single captured and encoded video frame.
type Frame struct {
    Data      []byte    // JPEG-encoded image data
    Timestamp time.Time // capture timestamp (monotonic)
    SeqNum    uint64    // monotonically increasing sequence number
}

// RingBuffer is a fixed-capacity circular buffer of Frames.
// Thread-safe for single-writer, multiple-reader access.
type RingBuffer struct {
    frames   []Frame
    capacity int
    head     atomic.Uint64  // next write position
    mu       sync.RWMutex
}

// NewRingBuffer creates a ring buffer with the given capacity.
// For 30fps with 10s depth: capacity=300.
// Memory estimate: 300 frames * ~300KB/frame = ~90MB.
func NewRingBuffer(capacity int) *RingBuffer

// Write adds a frame to the buffer, overwriting the oldest if full.
func (rb *RingBuffer) Write(f Frame)

// Latest returns the most recently written frame.
// Returns ErrNoFrames if the buffer is empty.
func (rb *RingBuffer) Latest() (Frame, error)

// Range returns frames from startSeq to the latest.
// Used by --clip to extract a time window.
// Returns at most `limit` frames. Frames older than the buffer are silently skipped.
func (rb *RingBuffer) Range(startSeq uint64, limit int) []Frame

// Since returns all frames captured after the given timestamp.
// Used by --clip 30s: startTime = time.Now().Add(-30 * time.Second).
func (rb *RingBuffer) Since(t time.Time, limit int) []Frame
```

### 4.2 Socket Protocol

```go
// internal/vision/protocol.go

// Command types sent by clients to the daemon.
const (
    CmdFrame  = "FRAME\n"   // Request single latest frame
    CmdStream = "STREAM\n"  // Begin continuous frame streaming
    CmdStatus = "STATUS\n"  // Request daemon status as JSON
)

// DaemonStatus is the JSON payload returned by STATUS command.
type DaemonStatus struct {
    Running         bool      `json:"running"`
    Device          string    `json:"device"`
    Resolution      string    `json:"resolution"`       // "1920x1080"
    FPS             int       `json:"fps"`
    ActualFPS       float64   `json:"actual_fps"`       // measured
    BufferDepth     int       `json:"buffer_depth"`     // frames in ring buffer
    BufferCapacity  int       `json:"buffer_capacity"`
    FramesCaptured  uint64    `json:"frames_captured"`
    FramesDropped   uint64    `json:"frames_dropped"`
    ActiveClients   int       `json:"active_clients"`
    UptimeSeconds   float64   `json:"uptime_seconds"`
    LastFrameAt     time.Time `json:"last_frame_at"`
}
```

### 4.3 Client Implementation

```go
// internal/vision/client.go

// Client communicates with the vision daemon over the Unix socket.
type Client struct {
    conn net.Conn
}

// Frame requests a single JPEG frame from the daemon.
func (c *Client) Frame() ([]byte, error) {
    if _, err := c.conn.Write([]byte(CmdFrame)); err != nil {
        return nil, fmt.Errorf("sending FRAME command: %w", err)
    }
    return c.readLengthPrefixed()
}

// Stream returns a channel that receives JPEG frames continuously.
// The channel is closed when the context is cancelled or the connection drops.
func (c *Client) Stream(ctx context.Context) (<-chan []byte, error) {
    if _, err := c.conn.Write([]byte(CmdStream)); err != nil {
        return nil, fmt.Errorf("sending STREAM command: %w", err)
    }
    ch := make(chan []byte, 4) // small buffer to absorb jitter
    go func() {
        defer close(ch)
        for {
            select {
            case <-ctx.Done():
                return
            default:
                data, err := c.readLengthPrefixed()
                if err != nil {
                    return
                }
                select {
                case ch <- data:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()
    return ch, nil
}

// Status requests daemon health information.
func (c *Client) Status() (*DaemonStatus, error)

// Close closes the connection to the daemon.
func (c *Client) Close() error

// readLengthPrefixed reads a 4-byte big-endian length header followed by that many bytes.
func (c *Client) readLengthPrefixed() ([]byte, error)
```

---

## 5. Camera Capture Pipeline

### 5.1 Capture via ffmpeg Subprocess

The daemon does NOT use a Go V4L2 library. It uses ffmpeg as a subprocess to read from the camera device. This decision is deliberate:

- ffmpeg handles the full matrix of V4L2 pixel formats, resolution negotiation, and device quirks
- No CGO dependency for V4L2 (keeps the build simple)
- ffmpeg is already required for recording and transcoding
- The subprocess boundary provides crash isolation (camera driver bug does not crash the daemon)

```go
// internal/vision/capture.go

// CaptureConfig configures the ffmpeg capture subprocess.
type CaptureConfig struct {
    Device     string     // "/dev/video0" -- from config, NEVER from user/LLM input
    Resolution Resolution // from config
    FPS        int        // from config
    Quality    int        // JPEG quality 1-31 (lower = better), default 5
}

// StartCapture launches an ffmpeg subprocess that reads from the camera
// and outputs JPEG frames to stdout.
//
// The returned io.ReadCloser yields a continuous MJPEG stream.
// Closing it terminates the ffmpeg process.
func StartCapture(ctx context.Context, cfg CaptureConfig) (io.ReadCloser, error) {
    // SECURITY: Device path comes from validated config only.
    // SECURITY: Arguments are passed as separate slice elements, never via shell.
    args := []string{
        "-f", "v4l2",
        "-video_size", fmt.Sprintf("%dx%d", cfg.Resolution.Width, cfg.Resolution.Height),
        "-framerate", strconv.Itoa(cfg.FPS),
        "-i", cfg.Device,
        "-f", "mjpeg",
        "-q:v", strconv.Itoa(cfg.Quality),
        "-an",       // no audio
        "pipe:1",    // output to stdout
    }

    cmd := exec.CommandContext(ctx, "ffmpeg", args...)
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, fmt.Errorf("creating stdout pipe: %w", err)
    }
    cmd.Stderr = nil // TODO: capture to logger for diagnostics

    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("starting ffmpeg capture: %w", err)
    }

    return stdout, nil
}
```

### 5.2 MJPEG Frame Splitting

ffmpeg outputs an MJPEG stream (concatenated JPEG frames). The daemon splits these at JPEG boundaries:

```go
// internal/vision/mjpeg.go

// SplitMJPEG reads an MJPEG stream and splits it into individual JPEG frames.
// Each frame starts with SOI (0xFF 0xD8) and ends with EOI (0xFF 0xD9).
// Calls onFrame for each complete frame. Blocks until the reader is closed or ctx is cancelled.
func SplitMJPEG(ctx context.Context, r io.Reader, onFrame func(Frame)) error
```

---

## 6. Gemini Multimodal Integration ("Eye" Feature)

### 6.1 Data Flow

```
User runs: qraft eye ["inspect the print bed"]
    |
    v
[1] Connect to vision daemon socket
    |
    v
[2] Send FRAME command, receive JPEG bytes
    |
    v
[3] Hydrator assembles context:
    - User prompt (or default: "Analyze this lab image and provide a health report")
    - Cerebro recall (relevant memories about the lab, printer state, etc.)
    - Sensor state (printer telemetry, temperature, humidity)
    - Image data (JPEG bytes from step 2)
    |
    v
[4] Gemini API call with multimodal content:
    - System prompt (with delimited memories + sensor data)
    - User content: [Text part + InlineData part (JPEG)]
    |
    v
[5] Gemini responds with text analysis
    |
    v
[6] Display to user. Optionally store observation as Cerebro memory.
```

### 6.2 The "Eye" is a CLI Command, NOT a Gemini Tool

This is a critical design decision. The original proposal frames `qraft eye` as a command that "captures a frame and asks Gemini for a Lab Health Report." There are two ways to implement this:

**Option A (chosen): `qraft eye` is a top-level CLI command that uses Gemini.**
The CLI captures the frame, assembles the multimodal prompt, calls Gemini, and displays the result. Gemini never "asks" for a frame -- the frame is provided as part of the initial prompt.

**Option B (rejected): `capture_frame` is a Gemini tool.**
Gemini could call a `capture_frame` tool during the tool loop, receive the image, and then reason about it. This requires Gemini to decide when to capture, adds a tool call round-trip, and complicates the tool response format (returning binary image data as a tool result).

Option A is correct because:
- The user explicitly requested a visual analysis. There is no ambiguity about whether to capture.
- The frame must be in the initial prompt for Gemini to reason about it. Tool results in subsequent turns are less effective for visual analysis than images in the initial content.
- It avoids the complexity of returning binary data from a tool execution.
- It is consistent with the existing CLI model: `qraft eye` is a command, like `qraft rec start`.

However, we ALSO register a `capture_frame` tool for use in the general `qraft` prompt flow. This allows Gemini to request a frame during an open-ended conversation (e.g., user says "check on the printer" and Gemini decides it needs to see it). This tool returns a text description, not the image -- Gemini cannot receive images via tool responses in the current SDK. Instead, the tool captures the frame, saves it, and returns a description:

```go
// internal/tools/vision_eye.go

// CaptureFrameTool captures a frame from the vision daemon.
// When called by Gemini, it saves the JPEG and returns metadata.
// The actual visual analysis happens in a follow-up multimodal turn.
type CaptureFrameTool struct {
    visionClient *vision.Client
    workDir      string // ~/.qraftworx/media/captures/
}

func (t *CaptureFrameTool) Name() string        { return "capture_frame" }
func (t *CaptureFrameTool) Description() string  {
    return "Captures a frame from the lab camera. Returns the file path and metadata. Use this when you need to visually inspect something."
}
func (t *CaptureFrameTool) RequiresConfirmation() bool { return false } // read-only
func (t *CaptureFrameTool) Permissions() ToolPermission {
    return ToolPermission{MediaCapture: true, FileSystem: true}
}
```

### 6.3 Hydrator Changes

The existing `HydratedContext` struct is extended to support image data:

```go
// internal/hydrator/hydrator.go

type HydratedContext struct {
    UserPrompt    string
    Memories      []store.ScoredNode
    SensorState   map[string]any
    SystemPrompt  string
    TokenEstimate int

    // NVS addition: optional image for multimodal prompts
    Images        []ImageAttachment  // NEW
}

// ImageAttachment represents an image to include in the Gemini prompt.
type ImageAttachment struct {
    Data     []byte // JPEG bytes
    MIMEType string // "image/jpeg"
    Label    string // human-readable label for logging, e.g., "lab-camera-capture"
}
```

The Gemini client must be updated to construct `[]*genai.Part` from the hydrated context:

```go
// internal/gemini/client.go

func (c *Client) buildParts(hc *HydratedContext) []*genai.Part {
    parts := make([]*genai.Part, 0, 2+len(hc.Images))
    parts = append(parts, &genai.Part{Text: hc.SystemPrompt + "\n\n" + hc.UserPrompt})

    for _, img := range hc.Images {
        parts = append(parts, &genai.Part{
            InlineData: &genai.Blob{
                Data:     img.Data,
                MIMEType: img.MIMEType,
            },
        })
    }

    return parts
}
```

### 6.4 Token Budget Impact

A 1920x1080 JPEG image at quality 5 is approximately 200-400KB. Gemini counts image tokens based on resolution:
- 1080p image: ~1100 tokens (Gemini's tile-based counting)
- This is a meaningful addition to the token budget

The hydrator must account for image tokens when computing `TokenEstimate`. If images plus text exceed the budget, the hydrator truncates memories (not images -- the image is the point of the command).

---

## 7. Recording Subsystem ("Chronicler")

### 7.1 Architecture

Recording is a separate long-lived process, NOT part of the daemon. The daemon provides frames; the recorder consumes them and transcodes via ffmpeg with hardware acceleration.

```
qraft rec start [--source daemon|gopro] [--quality high|medium]
    |
    v
[1] Connect to vision daemon socket (or GoPro HTTP stream)
    |
    v
[2] Send STREAM command, begin receiving MJPEG frames
    |
    v
[3] Pipe frames to ffmpeg subprocess for H.264/H.265 encoding
    |    with VA-API hardware acceleration
    |
    v
[4] ffmpeg writes to ~/qraftworx/vault/raw/<timestamp>.mp4
    |
    v
[5] Recording state written to ~/.qraftworx/run/recording.json
    |    (PID, start time, output path, source)
    |
    v
qraft rec stop
    |
    v
[6] Sends SIGTERM to ffmpeg PID from recording.json
[7] Cleans up recording.json
```

### 7.2 ffmpeg with VA-API Hardware Acceleration

```go
// internal/vision/recorder.go

// RecordConfig configures a recording session.
type RecordConfig struct {
    Source       FrameSource // daemon socket or GoPro URL
    OutputDir    string      // validated against SafePath (S9)
    OutputFormat string      // "mp4" (default)
    Codec        string      // "h264_vaapi" (default) or "hevc_vaapi"
    RenderDevice string      // "/dev/dri/renderD128" -- from config, not LLM
    Quality      int         // CRF-equivalent for VA-API (1-51, default 23)
}

// StartRecording begins recording frames to a file.
// Returns a RecordingHandle for status and stop operations.
func StartRecording(ctx context.Context, cfg RecordConfig) (*RecordingHandle, error) {
    timestamp := time.Now().Format("2006-01-02T15-04-05")
    outputPath := filepath.Join(cfg.OutputDir, timestamp+"."+cfg.OutputFormat)

    // SECURITY: All paths from validated config. Render device from config.
    args := []string{
        "-vaapi_device", cfg.RenderDevice,
        "-f", "mjpeg",
        "-i", "pipe:0",           // read MJPEG from stdin
        "-vf", "format=nv12,hwupload",
        "-c:v", cfg.Codec,
        "-qp", strconv.Itoa(cfg.Quality),
        "-movflags", "+faststart", // enable progressive download
        outputPath,
    }

    cmd := exec.CommandContext(ctx, "ffmpeg", args...)
    stdin, err := cmd.StdinPipe()
    if err != nil {
        return nil, err
    }

    // ... start cmd, connect stdin to frame stream from daemon ...

    return &RecordingHandle{
        cmd:        cmd,
        stdin:      stdin,
        outputPath: outputPath,
        startedAt:  time.Now(),
    }, nil
}

// RecordingHandle manages an active recording session.
type RecordingHandle struct {
    cmd        *exec.Cmd
    stdin      io.WriteCloser
    outputPath string
    startedAt  time.Time
}

// Stop gracefully terminates the recording.
func (rh *RecordingHandle) Stop() error {
    // Close stdin pipe -- ffmpeg finishes writing and exits cleanly
    return rh.stdin.Close()
}

// Status returns recording metadata.
func (rh *RecordingHandle) Status() RecordingStatus {
    return RecordingStatus{
        Active:     true,
        OutputPath: rh.outputPath,
        StartedAt:  rh.startedAt,
        Duration:   time.Since(rh.startedAt),
    }
}

// RecordingStatus is the JSON-serializable state of a recording.
type RecordingStatus struct {
    Active     bool          `json:"active"`
    OutputPath string        `json:"output_path"`
    StartedAt  time.Time     `json:"started_at"`
    Duration   time.Duration `json:"duration"`
    PID        int           `json:"pid"`
}
```

### 7.3 Recording State Persistence

Recording state is written to `~/.qraftworx/run/recording.json` so that `qraft rec stop` and `qraft rec status` can find the active recording even though they are separate process invocations.

```go
// internal/vision/recording_state.go

const RecordingStatePath = "~/.qraftworx/run/recording.json"

// SaveRecordingState persists the state of an active recording.
func SaveRecordingState(state RecordingStatus) error

// LoadRecordingState reads the current recording state.
// Returns ErrNoActiveRecording if no state file exists.
func LoadRecordingState() (*RecordingStatus, error)

// ClearRecordingState removes the state file.
func ClearRecordingState() error

var ErrNoActiveRecording = errors.New("no active recording")
```

### 7.4 Rolling Buffer and --clip

`qraft rec --clip 30s` extracts the last 30 seconds of footage from the daemon's ring buffer. This does NOT require an active recording session. The daemon's ring buffer always holds the last N seconds of frames.

```
qraft rec --clip 30s
    |
    v
[1] Connect to daemon, send STATUS to get buffer_depth and fps
    |
    v
[2] Calculate: 30s * 30fps = 900 frames needed
    |    If buffer_depth < 900, warn user and clip what is available
    |
    v
[3] Request frames from daemon via a CLIP command:
    |    "CLIP <duration_ms>\n" -> daemon sends frames from buffer
    |
    v
[4] Pipe frames to ffmpeg for encoding (same as recording, but finite)
    |
    v
[5] Output: ~/qraftworx/vault/clips/<timestamp>-30s.mp4
```

This means the daemon ring buffer must be sized for the maximum expected clip duration. For 30s at 30fps = 900 frames at ~300KB each = ~270MB. The systemd `MemoryMax=512M` must accommodate this plus overhead.

The protocol adds one more command:

```go
const CmdClip = "CLIP"  // "CLIP 30000\n" -> send last 30000ms of frames
```

### 7.5 Timelapse

`qraft timelapse --fps 1 --duration 2h` captures one frame per second for 2 hours.

This is a separate long-running CLI process (not the daemon). It connects to the daemon, requests a FRAME every N seconds, and writes individual JPEG files to a timelapse directory. A post-processing step assembles them into video.

```
qraft timelapse --fps 1 --duration 2h [--output-dir ~/qraftworx/vault/timelapse/]
    |
    v
[1] Connect to daemon
    |
    v
[2] Every 1/fps seconds: send FRAME, receive JPEG, write to disk
    |    Filename: frame_000001.jpg, frame_000002.jpg, ...
    |
    v
[3] On completion or Ctrl+C:
    |    Run ffmpeg to assemble JPEGs into video
    |    ffmpeg -framerate 30 -i frame_%06d.jpg -c:v h264_vaapi output.mp4
    |
    v
[4] Output: ~/qraftworx/vault/timelapse/<timestamp>/output.mp4
```

---

## 8. Streaming Subsystem ("Viewport")

### 8.1 Design Decision: Embedded HTTP Server

The stream server is an HTTP server that runs as a long-lived process, similar to recording. It is NOT embedded in the daemon -- it is a separate process started by `qraft stream start`.

```
qraft stream start [--port 8080] [--bind 0.0.0.0]
    |
    v
[1] Connect to vision daemon socket
    |
    v
[2] Send STREAM command, begin receiving frames
    |
    v
[3] Start HTTP server on configured port
    |    GET /          -> simple HTML page with <img> tag
    |    GET /mjpeg     -> multipart/x-mixed-replace MJPEG stream
    |    GET /health    -> JSON health check
    |    GET /snapshot  -> single JPEG frame
    |
    v
[4] Server runs until: qraft stream stop, SIGTERM, or Ctrl+C
```

### 8.2 Why a Separate Process, Not a Daemon Feature

The stream server is a consumer of the daemon, not part of it, because:

1. **Fault isolation.** If the HTTP server panics or leaks memory, the camera daemon keeps running. Recording is not affected.
2. **Independent lifecycle.** You might want the daemon running 24/7 but only stream when viewing remotely. Starting/stopping the stream does not affect other consumers.
3. **Security surface.** The HTTP server is network-exposed (even if only via Tailscale). Keeping it in a separate process limits the blast radius of a vulnerability. The daemon only listens on a Unix socket.
4. **Resource accounting.** systemd can apply different resource limits to the stream server vs. the daemon.

### 8.3 MJPEG Relay Implementation

```go
// internal/vision/stream_server.go

// StreamServer serves MJPEG over HTTP.
type StreamServer struct {
    visionClient *vision.Client
    bindAddr     string
    logger       *slog.Logger

    // Track connected clients for metrics
    clients      atomic.Int32
}

// MJPEGHandler implements the multipart/x-mixed-replace MJPEG stream.
func (s *StreamServer) MJPEGHandler(w http.ResponseWriter, r *http.Request) {
    s.clients.Add(1)
    defer s.clients.Add(-1)

    // Set headers for MJPEG stream
    w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
    w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
    w.Header().Set("Connection", "close")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    ctx := r.Context()
    frames, err := s.visionClient.Stream(ctx)
    if err != nil {
        http.Error(w, "failed to connect to vision daemon", http.StatusServiceUnavailable)
        return
    }

    for frame := range frames {
        _, err := fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame))
        if err != nil {
            return // client disconnected
        }
        if _, err := w.Write(frame); err != nil {
            return
        }
        if _, err := w.Write([]byte("\r\n")); err != nil {
            return
        }
        flusher.Flush()
    }
}
```

### 8.4 Stream State Persistence

Like recording, stream state is persisted for cross-process commands:

```go
// ~/.qraftworx/run/stream.json
type StreamStatus struct {
    Active  bool      `json:"active"`
    PID     int       `json:"pid"`
    Port    int       `json:"port"`
    Bind    string    `json:"bind"`
    Started time.Time `json:"started_at"`
    Clients int       `json:"clients"`
}
```

---

## 9. Event-Driven Recording Triggers

### 9.1 Architecture

Event-driven recording (e.g., "start recording when the 3D printer starts a print") integrates with Qraft's existing sensor subsystem. It does NOT live in the vision daemon. It is a separate watcher process or a feature of the main Qraft event loop.

```
MQTT Broker (Bambu Lab printer)
    |
    | topic: device/<serial>/report
    v
+---------------------+
| Qraft Event Watcher |
| (qraft watch)       |
|                     |
| Subscribes to MQTT  |
| Evaluates triggers  |
| Executes actions    |
+---------------------+
    |
    | trigger: printer.state == "RUNNING" && printer.prev_state == "IDLE"
    | action:  start_recording
    v
+-------------------+
| qraft rec start   |
| (spawned as       |
|  subprocess)      |
+-------------------+
```

### 9.2 Trigger Configuration

```toml
# ~/.qraftworx/config.toml

[[triggers]]
name = "record-on-print-start"
enabled = true

  [triggers.condition]
  source = "mqtt"
  topic = "device/+/report"
  # JSONPath expression evaluated against the MQTT payload
  expression = "$.print.gcode_state"
  from = "IDLE"
  to = "RUNNING"

  [triggers.action]
  command = "rec"
  args = ["start", "--source", "daemon"]

[[triggers]]
name = "stop-recording-on-print-end"
enabled = true

  [triggers.condition]
  source = "mqtt"
  topic = "device/+/report"
  expression = "$.print.gcode_state"
  from = "RUNNING"
  to = "IDLE"

  [triggers.action]
  command = "rec"
  args = ["stop"]
```

### 9.3 Event Watcher Interface

```go
// internal/events/watcher.go

// Trigger defines a condition-action pair.
type Trigger struct {
    Name       string          `toml:"name"`
    Enabled    bool            `toml:"enabled"`
    Condition  TriggerCondition `toml:"condition"`
    Action     TriggerAction    `toml:"action"`
}

// TriggerCondition defines what event activates the trigger.
type TriggerCondition struct {
    Source     string `toml:"source"`     // "mqtt" | "http_poll" | "sensor"
    Topic      string `toml:"topic"`      // MQTT topic pattern
    Expression string `toml:"expression"` // JSONPath into the payload
    From       string `toml:"from"`       // previous value (state transition)
    To         string `toml:"to"`         // new value (state transition)
}

// TriggerAction defines what happens when the trigger fires.
type TriggerAction struct {
    Command string   `toml:"command"` // qraft subcommand
    Args    []string `toml:"args"`    // arguments to pass
}

// Watcher monitors event sources and fires triggers.
type Watcher struct {
    triggers    []Trigger
    mqttClient  mqtt.Client
    logger      *slog.Logger
    qraftBinary string       // path to the qraft binary for subprocess execution

    // State tracking for transition detection
    lastValues  map[string]string  // trigger_name -> last observed value
    mu          sync.Mutex
}

// EvaluateTrigger checks if a trigger's condition is met.
// Returns true if the state transition matches.
func (w *Watcher) EvaluateTrigger(t Trigger, payload []byte) bool

// ExecuteAction runs the trigger's action as a subprocess.
// SECURITY: command and args come from config file only, never from MQTT payload.
func (w *Watcher) ExecuteAction(t Trigger) error {
    args := append([]string{t.Action.Command}, t.Action.Args...)
    cmd := exec.CommandContext(context.Background(), w.qraftBinary, args...)
    return cmd.Run()
}
```

### 9.4 Security Constraints

- Trigger actions are TOML config values, never derived from MQTT payload content.
- MQTT payloads are parsed with strict JSON schema validation before evaluation.
- The JSONPath expression operates on parsed JSON, not raw strings.
- This aligns with security finding S5 (MQTT transport security) -- triggers require authenticated MQTT.

---

## 10. Storage and Lifecycle

### 10.1 Directory Structure

```
~/qraftworx/
  vault/
    raw/                    # Full recordings
      2026-03-26T14-30-00.mp4
      2026-03-26T16-45-00.mp4
    clips/                  # Extracted clips from --clip
      2026-03-26T14-35-00-30s.mp4
    timelapse/              # Timelapse sessions
      2026-03-26T10-00-00/
        frames/             # Individual JPEGs
        output.mp4          # Assembled video
    captures/               # Eye snapshots
      2026-03-26T14-30-05.jpg
  run/                      # Runtime state (PIDs, status files)
    recording.json
    stream.json
  logs/
    qraft.jsonl
    vision-daemon.jsonl
```

### 10.2 Retention Policy

Media files accumulate quickly. A 1080p H.264 recording at reasonable quality produces ~1-2GB per hour.

```toml
# ~/.qraftworx/config.toml

[media.retention]
raw_max_age = "72h"         # Delete raw recordings older than 3 days
clips_max_age = "168h"      # Delete clips older than 7 days
timelapse_max_age = "720h"  # Delete timelapse projects older than 30 days
captures_max_age = "168h"   # Delete snapshots older than 7 days
max_total_size_gb = 50      # Hard cap on total media storage
warn_at_percent = 80        # Log warning when 80% of cap is used
```

### 10.3 Cleanup Automation

```go
// internal/vision/retention.go

// MediaRetention manages disk space for media files.
type MediaRetention struct {
    VaultDir string
    Config   RetentionConfig
    Logger   *slog.Logger
}

// RetentionConfig defines retention rules.
type RetentionConfig struct {
    RawMaxAge       time.Duration
    ClipsMaxAge     time.Duration
    TimelapseMaxAge time.Duration
    CapturesMaxAge  time.Duration
    MaxTotalSizeGB  int
    WarnAtPercent   int
}

// Cleanup scans the vault directory and removes files that exceed retention limits.
// Returns a report of what was deleted and total space reclaimed.
func (mr *MediaRetention) Cleanup() (*CleanupReport, error)

// DiskUsage returns current vault size and percentage of cap.
func (mr *MediaRetention) DiskUsage() (usedBytes int64, percentOfCap float64, err error)

// CleanupReport summarizes a cleanup operation.
type CleanupReport struct {
    FilesDeleted  int   `json:"files_deleted"`
    BytesReclaimed int64 `json:"bytes_reclaimed"`
    RemainingBytes int64 `json:"remaining_bytes"`
    ByCategory    map[string]int `json:"by_category"` // "raw" -> 3, "clips" -> 1
}
```

Cleanup runs:
- Automatically before each new recording starts (fail-safe: ensure disk space)
- Via `qraft vault cleanup` manual command
- Optionally on a systemd timer (e.g., daily at 3 AM)

### 10.4 Disk Space Guard

Before starting any recording or timelapse, check available disk space:

```go
// internal/vision/diskguard.go

// EnsureDiskSpace checks that there is enough free space to record.
// Returns ErrInsufficientDiskSpace if free space is below minFreeGB.
func EnsureDiskSpace(vaultDir string, minFreeGB int) error

var ErrInsufficientDiskSpace = errors.New("insufficient disk space for recording")
```

---

## 11. Tailscale/VPN Integration

### 11.1 Qraft Does NOT Manage Tailscale

Tailscale is a system-level service. Qraft does not install, configure, or manage it. The integration is:

1. **Documentation-only for setup:** The Qraft docs explain how to install Tailscale on CORE-01 and access the stream via `http://<tailscale-hostname>:8080/mjpeg`.

2. **Bind address awareness:** `qraft stream start` defaults to `--bind 100.0.0.0/8` (Tailscale CGNAT range) when Tailscale is detected, and `127.0.0.1` otherwise. This prevents accidental exposure on the LAN.

3. **Tailscale detection:** A simple check for Tailscale connectivity at stream startup:

```go
// internal/network/tailscale.go

// TailscaleStatus checks if Tailscale is running and connected.
// Returns the Tailscale IP if available, or empty string if not.
func TailscaleStatus() (tailscaleIP string, connected bool) {
    cmd := exec.CommandContext(context.Background(), "tailscale", "status", "--json")
    out, err := cmd.Output()
    if err != nil {
        return "", false
    }
    var status struct {
        Self struct {
            TailscaleIPs []string `json:"TailscaleIPs"`
        } `json:"Self"`
        BackendState string `json:"BackendState"`
    }
    if err := json.Unmarshal(out, &status); err != nil {
        return "", false
    }
    if status.BackendState != "Running" || len(status.Self.TailscaleIPs) == 0 {
        return "", false
    }
    return status.Self.TailscaleIPs[0], true
}
```

### 11.2 Stream Server Bind Logic

```go
func defaultBindAddr() string {
    if ip, ok := TailscaleStatus(); ok {
        return ip + ":8080"
    }
    return "127.0.0.1:8080" // localhost-only fallback
}
```

The user can always override with `--bind`.

---

## 12. Impact on Existing Architecture

### 12.1 Changes to Existing Components

| Component | Change | Scope |
|-----------|--------|-------|
| **Hydrator** | Add `Images []ImageAttachment` field to `HydratedContext` | Additive -- existing prompts without images are unchanged |
| **Gemini client** | Update `buildParts()` to include `genai.Blob` InlineData parts when images are present | Additive -- existing text-only codepath unchanged |
| **Tool Registry** | Register new tools: `capture_frame` | Additive -- append to existing registration |
| **Config** | Add `[vision]`, `[media]`, `[media.retention]`, `[[triggers]]` sections to config.toml | Additive -- all new sections, no existing sections modified |
| **Executor** | No changes. `capture_frame` returns `RequiresConfirmation() == false` (read-only). | None |
| **Observability** | Add vision daemon metrics to structured logging | Additive -- new log entries, existing format unchanged |

### 12.2 New Configuration Sections

```toml
# ~/.qraftworx/config.toml additions

[vision]
enabled = true
socket_path = "/run/qraft/vision.sock"

  [vision.camera]
  device = "/dev/video0"
  resolution = "1920x1080"
  fps = 30
  jpeg_quality = 5

  [vision.buffer]
  depth_seconds = 30          # ring buffer holds 30s of frames
  # Calculated: 30s * 30fps = 900 frames * ~300KB = ~270MB

  [vision.recording]
  output_dir = "~/qraftworx/vault/raw"
  codec = "h264_vaapi"
  render_device = "/dev/dri/renderD128"
  quality = 23

  [vision.streaming]
  default_port = 8080
  # bind_addr determined automatically (see Tailscale section)

[media.retention]
raw_max_age = "72h"
clips_max_age = "168h"
timelapse_max_age = "720h"
captures_max_age = "168h"
max_total_size_gb = 50
warn_at_percent = 80
```

### 12.3 New Tools Registered

| Tool | Description | Confirmation | Permissions |
|------|-------------|-------------|-------------|
| `capture_frame` | Captures a single frame from the vision daemon | No | MediaCapture, FileSystem |

Note: Recording and streaming are CLI commands, not Gemini tools. Gemini should not autonomously start/stop recordings. If event-driven recording is needed, it goes through the trigger system (section 9), not through Gemini tool calls.

### 12.4 New CLI Commands

| Command | Description | Long-lived? |
|---------|-------------|-------------|
| `qraft vision daemon` | Start the vision daemon (systemd-managed) | Yes |
| `qraft vision status` | Check daemon health | No |
| `qraft eye [prompt]` | Capture frame + Gemini visual analysis | No |
| `qraft rec start` | Start recording | Yes (ffmpeg subprocess) |
| `qraft rec stop` | Stop active recording | No |
| `qraft rec status` | Check recording state | No |
| `qraft rec --clip 30s` | Extract last 30s from buffer | No |
| `qraft stream start` | Start MJPEG stream server | Yes (HTTP server) |
| `qraft stream stop` | Stop stream server | No |
| `qraft stream status` | Check stream state | No |
| `qraft timelapse` | Start timelapse capture | Yes (periodic FRAME requests) |
| `qraft vault cleanup` | Run media retention cleanup | No |
| `qraft vault status` | Show disk usage and retention info | No |
| `qraft watch` | Start event watcher (trigger evaluator) | Yes |

---

## 13. Project Structure Changes

New packages added to the Qraft project:

```
qraftworx/
  internal/
    vision/                 # NEW: Vision subsystem
      daemon.go             # Daemon main loop, lifecycle
      ringbuf.go            # Ring buffer implementation
      ringbuf_test.go
      capture.go            # ffmpeg capture subprocess management
      capture_test.go
      mjpeg.go              # MJPEG stream splitter
      mjpeg_test.go
      protocol.go           # Socket protocol constants and types
      client.go             # Client for connecting to daemon
      client_test.go
      server.go             # Unix socket server (daemon-side)
      server_test.go
      recorder.go           # Recording management (ffmpeg transcoding)
      recorder_test.go
      recording_state.go    # Recording state persistence
      stream_server.go      # MJPEG HTTP relay server
      stream_server_test.go
      retention.go          # Media retention/cleanup
      retention_test.go
      diskguard.go          # Disk space checks
      diskguard_test.go
    events/                 # NEW: Event-driven triggers
      watcher.go            # MQTT subscription + trigger evaluation
      watcher_test.go
      trigger.go            # Trigger types and configuration
      trigger_test.go
    network/                # NEW: Network utilities
      tailscale.go          # Tailscale detection
      tailscale_test.go
    tools/
      vision_eye.go         # NEW: capture_frame tool
      vision_eye_test.go
  cmd/qraft/
    cmd_vision.go           # NEW: vision daemon, vision status
    cmd_eye.go              # NEW: eye command
    cmd_rec.go              # NEW: rec start/stop/status/--clip
    cmd_stream.go           # NEW: stream start/stop/status
    cmd_timelapse.go        # NEW: timelapse command
    cmd_vault.go            # NEW: vault cleanup/status
    cmd_watch.go            # NEW: watch command (event watcher)
  deploy/
    systemd/                # NEW: systemd unit files
      qraft-vision.service
      qraft-watch.service   # Optional: event watcher as a service
```

---

## 14. Build Order: NVS Phases

NVS phases into the existing 8-phase Qraft build plan as phases 9-13. Each phase depends on the previous and produces a tested, working increment.

**Prerequisites:** Phases 1-6 of the existing plan must be complete. NVS requires:
- Working Gemini client with tool loop (Phase 2-3)
- Hydrator with sensor support (Phase 5)
- Media capture tool foundation (Phase 6)
- MQTT sensor integration (Phase 5)

| Phase | Deliverable | Dependencies | Definition of Done |
|-------|-------------|-------------|-------------------|
| **9** | **Vision Daemon (core)** | Phase 6 | Daemon opens camera via ffmpeg, writes to ring buffer, serves FRAME/STATUS over Unix socket. Client library connects and retrieves frames. Tested with mock ffmpeg output. systemd unit file included. |
| **10** | **Eye Command** | Phase 9 + Phase 3 | `qraft eye` captures frame, assembles multimodal prompt (hydrator extended with `ImageAttachment`), sends to Gemini, displays analysis. Tested with recorded Gemini responses + mock daemon. |
| **11** | **Recording Subsystem** | Phase 9 | `qraft rec start/stop/status` works with VA-API hardware encoding. `qraft rec --clip 30s` extracts from ring buffer. Recording state persisted. `qraft vault cleanup/status` manages retention. Tested with mock frame stream. |
| **12** | **Streaming Subsystem** | Phase 9 | `qraft stream start/stop/status` serves MJPEG over HTTP. Tailscale-aware bind address. Multi-client support. Tested with httptest. |
| **13** | **Event Triggers + Timelapse** | Phase 9 + Phase 5 (MQTT) | `qraft watch` evaluates MQTT triggers and executes actions. `qraft timelapse` captures periodic frames. Tested with mock MQTT broker. |

### Phase Dependency Graph

```
Existing Phases:
  1 (CerebroClient) -> 2 (Gemini) -> 3 (Single-pass loop)
                                        -> 4 (Observability)
                                        -> 5 (Sensors/MQTT)
                                        -> 6 (Media tools)
                                           -> 7 (Upload)
                                           -> 8 (Config)

NVS Phases:
  6 (Media tools) -----> 9 (Vision Daemon)
                              |
                              +--> 10 (Eye) [also needs 3]
                              |
                              +--> 11 (Recording)
                              |
                              +--> 12 (Streaming)
                              |
  5 (Sensors/MQTT) -----> 13 (Event Triggers + Timelapse) [also needs 9]
```

Phases 10, 11, and 12 can be built in parallel once Phase 9 is complete. Phase 13 requires both Phase 9 and Phase 5.

---

## 15. Security Considerations (NVS-Specific)

All security findings from the Qraft architecture security assessment (Appendix A) apply. NVS adds these specific concerns:

| # | Finding | Severity | Mitigation |
|---|---------|----------|------------|
| NVS-S1 | Daemon socket world-readable could expose camera feed | HIGH | Socket created with `0750` permissions. systemd `RuntimeDirectoryMode=0750`. Only `qraftworx` user/group can connect. |
| NVS-S2 | Stream server exposed to network | HIGH | Default bind to Tailscale IP or localhost. Never `0.0.0.0` by default. No authentication on MJPEG stream (acceptable for private Tailscale network; document this limitation). |
| NVS-S3 | ffmpeg arguments constructed from config values | MEDIUM | All paths validated at config load time via `SafePath`. No config values derived from LLM output or MQTT payloads. |
| NVS-S4 | MQTT trigger actions could be replayed | MEDIUM | Trigger evaluation includes state transition check (from/to). A repeated MQTT message with the same state does not re-trigger. Rate limit: max 1 trigger fire per trigger per minute. |
| NVS-S5 | Camera feed captures private spaces | LOW | This is a deployment concern, not a software concern. Documentation should advise on camera placement and privacy implications. |

---

## 16. Open Questions

1. **GoPro integration.** The GoPro has an HTTP API for live streaming. Should the daemon support multiple camera sources, or should GoPro be a separate daemon instance? Recommendation: single-source per daemon, multiple daemon instances with different socket paths.

2. **Audio.** The current design captures video only. Should the Chronicler capture audio from a USB microphone? If yes, the ffmpeg command gains an audio input and the recording format must support audio (MP4 already does). Deferred to post-v1.

3. **HLS vs MJPEG.** MJPEG is simpler but bandwidth-hungry (no inter-frame compression). HLS is more efficient for remote viewing but adds latency (segment-based). Recommendation: MJPEG for v1 (simplicity), HLS as a future enhancement if bandwidth over Tailscale is a problem.

4. **Ring buffer memory pressure.** 900 frames at 300KB = 270MB is significant on a resource-constrained system. Should the buffer depth be dynamically adjustable? Recommendation: configurable via `[vision.buffer]` with a sensible default (10s). `--clip` duration capped at buffer depth with a warning.

---

## 17. Decision Log

| # | Decision | Rationale |
|---|----------|-----------|
| NVS-D1 | Subcommand in same binary, not separate binary | Single deployment artifact. No version skew. systemd manages lifecycle. |
| NVS-D2 | Unix domain socket, not mmap | Standard Go stdlib. Clean process isolation. Negligible latency difference for frame sizes. |
| NVS-D3 | ffmpeg subprocess for camera capture, not Go V4L2 library | ffmpeg handles device quirks. No CGO dependency. Crash isolation. |
| NVS-D4 | `qraft eye` is a CLI command that uses Gemini, not a Gemini tool | Frame is always captured on user request. No ambiguity. Image in initial prompt is more effective than tool-response images. |
| NVS-D5 | Recording and streaming are NOT Gemini tools | Gemini should not autonomously control media capture. Event triggers are the mechanism for automated recording. |
| NVS-D6 | Stream server is a separate process from daemon | Fault isolation. Independent lifecycle. Reduced security surface on the daemon. |
| NVS-D7 | Tailscale is external; Qraft only detects it | Network infrastructure is not Qraft's responsibility. Detection informs default bind address. |
| NVS-D8 | NVS phases after Phase 6 | Requires media tool foundation, Gemini client, and MQTT sensor integration to be in place. |
