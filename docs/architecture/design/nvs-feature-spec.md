# Nexus Vision System (NVS) -- Feature Specification

**Status:** Draft
**Date:** 2026-03-26
**Companion to:** [qraft-architecture.md](qraft-architecture.md) (base Qraft architecture)
**Source:** [qraft-vision-feature.md](qraft-vision-feature.md) (original feature request)
**Review inputs:** Principal Architect, Tech Lead, Security Specialist

---

## 1. Feature Summary

NVS adds centralized camera management to Qraft: AI-powered visual analysis ("Eye"), automated recording for content creation ("Chronicler"), and live streaming for remote monitoring ("Viewport"). The first use case is TikTok/YouTube content automation around 3D printing.

### Capabilities

| Capability | CLI Command | Description |
|---|---|---|
| AI Snapshot | `qraft eye` | Capture frame, send to Gemini multimodal for visual analysis |
| Start recording | `qraft rec start` | Begin recording to ~/qraftworx/vault/raw/ |
| Stop recording | `qraft rec stop` | Stop active recording |
| Clip extraction | `qraft rec --clip 30s` | Save last 30s from rolling buffer |
| Start stream | `qraft stream start` | Start authenticated MJPEG server |
| Stop stream | `qraft stream stop` | Stop stream server |
| Timelapse | `qraft timelapse --fps 1 --duration 4h` | Frame-by-frame capture over time |
| Daemon | `qraft vision serve` | Start the vision daemon (managed by systemd) |

---

## 2. Architectural Decision: Vision Daemon as Subcommand

### The Problem

NVS introduces long-running concerns (24/7 camera, rolling buffer, stream server, event-triggered recording) into a CLI designed for request-response interactions. These are fundamentally incompatible lifecycle models.

The camera device (`/dev/video0`) is single-reader on Linux. Without a shared access point, `qraft eye`, `qraft rec`, and `qraft stream` would contend for the device, producing `EBUSY` errors.

### The Decision

The vision daemon is a **subcommand within the existing Qraft binary** (`qraft vision serve`), managed by **systemd**. All other NVS commands are **clients** that talk to the daemon over a Unix domain socket.

**Why not a separate binary:** Version skew between the CLI and daemon becomes a maintenance burden. A single binary ensures the IPC protocol, tool interfaces, and Cerebro dependency are always in sync.

**Why not a fork:** Go's runtime is not fork-safe. Goroutines, the GC, and the scheduler do not survive `fork()` reliably.

**Why systemd:** Handles restart-on-crash, boot ordering, resource limits, and device access control (via `DeviceAllow=`). Already the standard on the target platform (Ubuntu on Dell Vostro, Raspberry Pi OS).

### Process Model

```
systemd
  |
  +-- qraft vision serve (daemon, long-running)
        |
        +-- camera reader goroutine (V4L2, keeps /dev/video0 open)
        +-- frame buffer (ring buffer, configurable depth)
        +-- Unix socket listener (/run/qraft/vision.sock)
        +-- [optional] ffmpeg recording subprocess
        +-- [optional] ffmpeg rolling-buffer subprocess
        +-- [optional] MJPEG stream server goroutine

qraft eye          ---[socket]---> daemon: request frame
qraft rec start    ---[socket]---> daemon: start recording
qraft rec --clip   ---[socket]---> daemon: extract clip from buffer
qraft stream start ---[socket]---> daemon: start stream server
```

---

## 3. IPC: Unix Domain Socket Protocol

The daemon listens on `/run/qraft/vision.sock`. CLI commands connect as clients.

### Protocol

Length-prefixed JSON-RPC over Unix socket. Simple, debuggable, no external dependencies.

```go
// Request
type VisionRequest struct {
    Method string          `json:"method"` // "capture", "rec_start", "rec_stop", "clip", "stream_start", "stream_stop", "status"
    Params json.RawMessage `json:"params,omitempty"`
}

// Response
type VisionResponse struct {
    OK     bool            `json:"ok"`
    Data   json.RawMessage `json:"data,omitempty"`
    Error  string          `json:"error,omitempty"`
}
```

**Socket permissions:** The socket file is created with `0660`, owned by the `qraftworx` group. Only processes running as the `qraftworx` user or group members can connect. This prevents unauthorized camera access from other users on the system.

---

## 4. Frame Sharing

The daemon captures frames from the camera continuously and holds them in an in-memory ring buffer.

```go
type FrameBuffer struct {
    mu     sync.RWMutex
    frames []Frame         // ring buffer
    head   int
    size   int             // configurable: 30s * fps
}

type Frame struct {
    Data      []byte    // JPEG-encoded
    Timestamp time.Time
    Seq       uint64    // monotonic sequence number
}
```

**Why in-memory, not mmap:** Go's stdlib has no `mmap` synchronization primitives. The latency difference between a mutex-protected slice and mmap is irrelevant for 200-400KB JPEG frames at 30fps. In-memory is simpler, safer, and testable.

**Buffer sizing:**

| FPS | Duration | Frames | RAM (at ~300KB/frame avg) |
|---|---|---|---|
| 30 | 30s | 900 | ~270 MB |
| 15 | 30s | 450 | ~135 MB |
| 10 | 30s | 300 | ~90 MB |
| 5 | 30s | 150 | ~45 MB |

Default: 15 fps, 30s = ~135 MB. Configurable based on hardware profile.

---

## 5. Feature Modules

### 5.1 Eye (AI Snapshot Engine)

**Data flow:**

```
qraft eye "inspect the print head"
    |
    +-- connect to daemon via Unix socket
    +-- request: method="capture"
    +-- daemon returns: latest JPEG frame
    |
    +-- Hydrator assembles context:
    |     - user prompt
    |     - cerebro recall results
    |     - sensor telemetry (printer state)
    |     - JPEG frame as ImageAttachment
    |
    +-- Gemini multimodal API call:
    |     - text parts (prompt + context)
    |     - inline image (genai.Blob{Data: jpeg, MIMEType: "image/jpeg"})
    |
    +-- Display response to user
```

**Integration with existing Qraft architecture:**

The `HydratedContext` struct gains an optional image attachment:

```go
type HydratedContext struct {
    UserPrompt    string
    Memories      []store.ScoredNode
    SensorState   map[string]any
    SystemPrompt  string
    TokenEstimate int
    Images        []ImageAttachment  // NEW for NVS
}

type ImageAttachment struct {
    Data     []byte
    MIMEType string  // "image/jpeg"
    Source   string  // "eye_capture", for audit logging
}
```

The Gemini client's `buildParts()` method includes `genai.Blob` when images are present.

**Tool registration:**

Two tools are registered, with different trust levels:

| Tool | RequiresConfirmation | Rationale |
|---|---|---|
| `eye_capture` | **YES, always** | Captures a frame AND sends it to an external API. Surveillance + data exfiltration combined. |
| `eye_preview` (future) | YES (recommended) | Local-only low-res thumbnail for context. No external transmission. |

**Eye is a direct CLI command (`qraft eye`), not primarily a Gemini-invoked tool.** When the user types `qraft eye`, it runs the capture-hydrate-call flow directly. The `eye_capture` tool in the Gemini registry exists for cases where the model determines mid-conversation that visual information would help -- but it always requires confirmation.

### 5.2 Chronicler (Automated Recording)

**Recording pipeline:**

```
Daemon frame buffer
    |
    +--[pipe]--> ffmpeg -f image2pipe -i - [encoder flags] output.mkv
```

The daemon streams frames from its buffer to ffmpeg's stdin. ffmpeg transcodes to H.264/H.265.

**Hardware acceleration with platform detection:**

```go
type EncoderProfile struct {
    Codec       string   // "h264_vaapi", "h264_v4l2m2m", "libx264"
    Device      string   // "/dev/dri/renderD128", "/dev/video11", ""
    ExtraArgs   []string // platform-specific flags
    CPUFallback bool     // true if this is the software fallback
}

func DetectEncoder() EncoderProfile {
    // 1. Check VA-API: /dev/dri/renderD128 exists + ffmpeg -hwaccels includes vaapi
    // 2. Check V4L2 M2M: ffmpeg -encoders includes h264_v4l2m2m
    // 3. Fallback: libx264 -preset ultrafast
}
```

| Platform | Encoder | Device | Notes |
|---|---|---|---|
| Dell Vostro (Intel i7) | `h264_vaapi` | `/dev/dri/renderD128` | Near-zero CPU |
| Raspberry Pi 5 | `h264_v4l2m2m` | `/dev/video11` | Hardware-assisted |
| Raspberry Pi 4 | `h264_v4l2m2m` | `/dev/video11` | Marginal, 720p recommended |
| Fallback (any) | `libx264 -preset ultrafast` | None | CPU-intensive, 720p max on Pi |

**Output format:** MKV (Matroska). Unlike MP4, MKV is seekable on partial writes -- if ffmpeg is killed mid-recording, the file is still playable. This directly addresses the Day 5 failure scenario (Ctrl+C corrupts recording).

**Clip extraction (`qraft rec --clip 30s`):**

The daemon extracts the last N seconds of frames from the in-memory ring buffer, pipes them to a short-lived ffmpeg process for encoding, and writes the output to a clip file. This is a daemon operation, not a CLI-side operation.

**Event-driven triggers:**

MQTT messages or sensor state changes can trigger recording start. Trigger conditions and actions come **exclusively from TOML config**, never from MQTT payloads. This is critical for security (see Section 8, N5).

```toml
[triggers.print_start]
source = "mqtt"
topic = "bambu/printer/status"
condition = "state_transition"
from = "IDLE"
to = "RUNNING"
action = "rec_start"
```

### 5.3 Viewport (Live Streaming)

**Protocol decision: MJPEG for v1.**

| Factor | MJPEG | HLS |
|---|---|---|
| Latency | 50-200ms | 2-10 seconds |
| CPU on server | Near zero (relay JPEGs from camera) | High (transcode + segment) |
| Client compatibility | Any browser, curl, `<img>` tag | Requires hls.js or Safari |
| Multi-client bandwidth | N clients = N * stream bandwidth | Cacheable segments, efficient |
| Implementation complexity | Simple HTTP multipart | Transcoder + segment manager + manifest |

MJPEG wins for v1: 1-3 clients over Tailscale, latency matters for lab monitoring, and on the Pi the CPU savings are the difference between "works" and "CPU pegged at 100%."

**Stream server is a goroutine within the daemon**, not a separate process. It reads from the same frame buffer that feeds recording and Eye captures. Client disconnects do not affect the daemon or other consumers.

**Authentication (mandatory):**

```go
func streamHandler(token string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Authorization") != "Bearer "+token {
            http.Error(w, "Unauthorized", 401)
            return
        }
        // ... serve MJPEG multipart stream
    }
}
```

The bearer token is configured in `~/.qraftworx/config.toml` (or generated on first `qraft init`). Tailscale provides network-layer security; the bearer token provides application-layer authentication. Defense in depth.

**Bind address:** The stream server binds to the Tailscale interface IP only, never `0.0.0.0`. If Tailscale is not running, it binds to `127.0.0.1` only and logs a warning.

---

## 6. Hardware Capability Tiers

The NVS spec assumes all features run on all hardware. They cannot. The Pi 4 cannot sustain simultaneous recording + streaming + the existing Qraft workload.

| Tier | Hardware | Available Features | Constraints |
|---|---|---|---|
| **Full** | Dell Vostro (Intel i7, 16GB+, SSD) | All: Eye, Chronicler, Viewport, Timelapse, Rolling Buffer | VA-API encoding, full resolution |
| **Lite** | Raspberry Pi 5 (8GB, USB SSD) | Eye, Chronicler OR Viewport (not both), Timelapse | V4L2 M2M encoding, 720p recommended, 15fps buffer |
| **Minimal** | Raspberry Pi 4 (4GB, SD card) | Eye only. No continuous recording, no streaming. | On-demand frame capture only. No rolling buffer. |

Tier is auto-detected at daemon startup based on available hardware (VA-API probe, memory, CPU cores) and can be overridden in config.

```toml
[vision]
tier = "auto"  # "auto", "full", "lite", "minimal"
```

If a user requests a feature unavailable in their tier, the daemon returns an error with a clear explanation:

```
Error: recording requires "lite" tier or higher.
Detected tier: minimal (Raspberry Pi 4, 4GB RAM).
Reason: insufficient RAM for rolling buffer + ffmpeg transcode.
Override with: [vision] tier = "lite" in config (at your own risk).
```

---

## 7. ffmpeg Subprocess Management

NVS has 6 distinct ffmpeg invocation sites. All MUST use the same `FFmpegBuilder` abstraction to prevent command injection (base finding S2).

### FFmpegBuilder

```go
type FFmpegBuilder struct {
    binary       string     // validated at startup: must exist, must be ffmpeg
    allowedIn    []SafePath // allowed input device paths (from config)
    allowedOut   []SafePath // allowed output directories (from config)
    encoderProf  EncoderProfile
}

func (b *FFmpegBuilder) CaptureFrame(device SafePath, output SafePath) *exec.Cmd { ... }
func (b *FFmpegBuilder) StartRecording(input io.Reader, output SafePath) *exec.Cmd { ... }
func (b *FFmpegBuilder) ExtractClip(input SafePath, output SafePath, duration time.Duration) *exec.Cmd { ... }
func (b *FFmpegBuilder) StartTimelapse(device SafePath, output SafePath, fps int, maxDuration time.Duration) *exec.Cmd { ... }
```

**Rules (non-negotiable):**
1. All paths come from validated config (SafePath), never from LLM tool arguments.
2. All numeric parameters (fps, duration, bitrate) are parsed to typed Go values with clamped ranges before being passed to exec.Command.
3. Never use `sh -c`. Always use `exec.CommandContext` with separate argument slices.
4. VA-API device path is validated against `/dev/dri/renderD*` pattern using `os.Lstat` (no symlink following).

### Process Lifecycle

Long-running ffmpeg processes (recording, rolling buffer) are managed with proper lifecycle:

```go
type FFmpegProcess struct {
    cmd     *exec.Cmd
    ctx     context.Context
    cancel  context.CancelFunc
    done    chan error
    started time.Time
}

func (p *FFmpegProcess) Stop() error {
    // 1. Send SIGINT (ffmpeg writes moov atom, closes cleanly)
    p.cmd.Process.Signal(syscall.SIGINT)

    // 2. Wait up to 5s for clean exit
    select {
    case err := <-p.done:
        return err
    case <-time.After(5 * time.Second):
        // 3. Escalate to SIGKILL
        p.cmd.Process.Kill()
        return <-p.done
    }
}
```

### Graceful Shutdown Coordinator

```
SIGINT/SIGTERM received
    |
    v
Cancel root context
    |
    +-- Stream server: stop accepting clients, drain existing (5s)
    +-- Recording ffmpeg: SIGINT -> wait 5s -> SIGKILL
    +-- Rolling buffer ffmpeg: SIGINT -> wait 5s -> SIGKILL
    +-- Timelapse: stop ticker, finish current frame
    +-- Camera reader: close device
    |
    v
errgroup.Wait() with 10s hard deadline
    |
    v
If anything alive: SIGKILL all, log error, exit
```

---

## 8. Security Requirements

NVS introduces the most sensitive security surface in Qraft: persistent camera access, AI-directed capture, and network-exposed video streaming. The following findings were identified by the security specialist and are **mandatory prerequisites** before implementation.

### Critical Findings

#### N1: Hardcoded API Key (CRITICAL -- Immediate Action)

A live Google AI API key was found hardcoded in `/Users/q/projects/qraftworx/handshake.py`. This must be rotated immediately and replaced with an environment variable. Add `gitleaks` to the pre-commit hook pipeline.

#### N2: Vision Daemon Enables AI-Triggerable Surveillance (CRITICAL)

The daemon keeps the camera open 24/7. The Eye tool in the Gemini tool loop means the AI can trigger frame capture. Combined with the existing prompt injection risk (S3), a compromised memory node could direct Gemini to capture frames without the user's knowledge.

**Required mitigations:**
- Eye tool MUST have `RequiresConfirmation() = true`, always.
- Socket file permissions restrict access to the `qraftworx` user/group only.
- Frame captures logged to a separate, append-only audit log (`~/.qraftworx/logs/captures.log`) with `O_APPEND|O_SYNC` -- separate from the Gemini interaction log.
- Rate limit: max 1 eye capture per interaction, max N per hour (configurable).

#### N3: Stream Server Authentication (CRITICAL)

Tailscale is not a substitute for application-layer auth. The stream server MUST require a bearer token on every request. MUST bind to Tailscale interface only (never `0.0.0.0`). If Tailscale is not running, bind localhost only.

#### N4: ffmpeg Injection Surface (CRITICAL)

NVS expands ffmpeg from 1 invocation site to 6+. All must use FFmpegBuilder (Section 7). No paths from LLM arguments. No shell interpolation. Typed, clamped numeric parameters only.

### High Findings

#### N5: MQTT-Triggered Recording (HIGH)

Existing S5 (unauthenticated MQTT) now escalates to unauthorized surveillance. MQTTS + authentication is a **hard prerequisite** for event-triggered recording. Trigger conditions come from config only, never from MQTT payloads. HMAC-signed events recommended. Rate limit: max 1 recording start per 5 minutes from MQTT events.

#### N6: AI-Directed Covert Capture (HIGH)

The Eye tool in the Gemini loop can be triggered by indirect prompt injection. Out-of-band audit logging (before capture, in a location Gemini cannot see) is required. Consider splitting into `eye_preview` (low-res, local) and `eye_capture` (high-res, sends to API) with different trust levels.

#### N7: Image Data Exfiltration (HIGH)

Eye sends camera frames to Google's Gemini API. The system prompt for Eye calls must strip sensitive memory content. Maximum JPEG resolution should be configurable. Never log raw image bytes. Review Google's Gemini API data use policy before implementation.

#### N8: Rolling Buffer Privacy (HIGH)

30 seconds of continuous video exists at all times when the buffer is active. Required: `0600` permissions, stored in `tmpfs` (not persistent disk), securely deleted on shutdown, **opt-in only** (disabled by default), max duration capped at 60s.

#### N9: Device File Symlink Attacks (HIGH)

Validate at every use that device paths resolve to character devices (not symlinks, not regular files) using `os.Lstat`. Validate against allowlist (`/dev/video*`, `/dev/dri/renderD*`). Re-validate at each ffmpeg invocation.

#### N10: Disk Exhaustion (HIGH)

Pre-recording disk space check (refuse if < 2GB free). Maximum file size per recording. Storage quota for vault directory. ffmpeg called with `-fs <max_bytes>`. Timelapse requires explicit `--duration` (no open-ended capture). Periodic space monitoring during recording.

### Medium Findings

| # | Finding | Mitigation |
|---|---|---|
| N11 | Tailscale as sole boundary | Enable MFA, define Tailscale ACLs, layer app-level auth |
| N12 | No encryption at rest for video | `~/qraftworx/vault/` permissions `0700`, evaluate LUKS |
| N13 | VA-API device validation | Validate path resolves to `/dev/dri/renderD*` char device |
| N14 | No max duration on timelapse/recording | Hard deadline via `context.WithTimeout`, 24h cap |
| N15 | MJPEG timing leaks motion metadata | Accept for v1, HLS for v2 if privacy critical |
| N16 | systemd unit needs hardening | `NoNewPrivileges=true`, `ProtectSystem=strict`, `DeviceAllow` |

### NVS Tool Confirmation Gate Requirements

| Tool | RequiresConfirmation | Reason |
|---|---|---|
| `eye_capture` | **YES** | Surveillance + external data transmission |
| `chronicler_start` | **YES** | Initiates recording |
| `chronicler_stop` | No | Reduces exposure |
| `chronicler_clip` | **YES** | Persists + potential exfiltration |
| `viewport_start` | **YES** | Network-exposed live camera feed |
| `viewport_stop` | No | Reduces exposure |
| `timelapse_start` | **YES** | Long-running capture |
| `timelapse_stop` | No | Reduces exposure |

### Base Finding Escalations Due to NVS

| Base Finding | Original Severity | NVS Context | Escalated Severity |
|---|---|---|---|
| S1 (confirmation gate ANSI injection) | CRITICAL | More tools need gates, media tools have privacy consequences | CRITICAL (unchanged, but scope grows) |
| S2 (ffmpeg command injection) | CRITICAL | 1 site -> 6+ sites | CRITICAL (unchanged, but remediation scope expands) |
| S3 (indirect prompt injection) | HIGH | Can now trigger covert camera capture | **CRITICAL** |
| S5 (MQTT unauthenticated) | HIGH | Can now trigger unauthorized recording | **CRITICAL** |
| S9 (filesystem boundary) | MEDIUM | Rolling buffer, vault, timelapse all need SafePath | MEDIUM (unchanged, scope grows) |
| S11 (media retention) | MEDIUM | Vault accumulates raw video indefinitely | **HIGH** |

---

## 9. Storage and Lifecycle

### Directory Structure

```
~/qraftworx/
  vault/
    raw/              # full recordings (Chronicler)
    clips/            # extracted clips
    timelapse/         # timelapse outputs
    captures/          # eye snapshots (JPEG)
  logs/
    qraft.jsonl       # interaction log (existing)
    captures.log      # out-of-band capture audit log (NEW)
  run/
    vision.pid        # daemon PID file
    recording.json    # active recording state
/run/qraft/
    vision.sock       # daemon Unix socket
    buffer/           # rolling buffer (tmpfs, ephemeral)
```

### Retention Policy

```toml
[vault]
max_total_gb = 100          # hard quota, oldest files deleted first
max_file_age_days = 30      # auto-delete after 30 days
min_free_disk_gb = 2        # refuse new recordings below this
```

Cleanup runs: before starting any new recording, and via `qraft vault cleanup`.

---

## 10. Testing Strategy

| Component | Test Approach |
|---|---|
| **Frame buffer** | Unit test: write N frames, read back, verify ring wraps correctly. No hardware needed. |
| **FrameSource interface** | Production: V4L2 device reader. Test: returns pre-recorded JPEGs from `testdata/`. Same pattern as Cerebro's `embed.Provider` interface. |
| **FFmpegBuilder** | Unit: verify argument slices for each invocation pattern. No subprocess execution. |
| **ffmpeg pipeline** | Integration: use `ffmpeg -f lavfi -i testsrc=duration=5:size=1280x720:rate=30` as synthetic input. Runs in CI without camera. |
| **Stream server** | Use `httptest.Server`. Spawn N goroutine clients, connect/disconnect randomly. Verify no goroutine leaks (`goleak`). |
| **Daemon lifecycle** | Start daemon, cancel context, assert all goroutines exit within 5s deadline. `goleak.VerifyNone(t)` in TestMain. |
| **Unix socket IPC** | Integration: start daemon in-process, connect via socket, send requests, verify responses. |
| **Shutdown coordinator** | Test each shutdown path: recording active, stream active, both, with hung ffmpeg (mock: ignore SIGINT). |
| **VA-API / V4L2 M2M** | Cannot be tested in CI. Manual acceptance tests on target hardware. CI uses libx264 fallback. |
| **Disk space checks** | Unit: mock `syscall.Statfs` or use a tiny tmpfs mount in test. |

---

## 11. Build Order (Phases 9-13)

NVS phases follow the existing 8-phase Qraft build plan.

| Phase | Deliverable | Prerequisites | Definition of Done |
|---|---|---|---|
| **9** | Vision daemon core | Phases 1-3 (base Qraft working) | Daemon starts, opens camera (or test source), serves frames via Unix socket. Graceful shutdown. systemd unit with hardening. |
| **10** | Eye (AI Snapshot) | Phase 9 + Phase 2 (Gemini client) | `qraft eye` captures frame, sends multimodal prompt, returns analysis. Confirmation gate. Audit log. |
| **11** | Chronicler (Recording) | Phase 9 + FFmpegBuilder | `qraft rec start/stop`, hardware-accelerated encoding, MKV output, disk space checks. |
| **12** | Viewport (Streaming) | Phase 9 | `qraft stream start/stop`, MJPEG server, bearer token auth, Tailscale-aware binding. |
| **13** | Event triggers + Timelapse + Clip | Phase 9 + Phase 5 (MQTT sensors) + Phase 11 | MQTT-triggered recording (MQTTS required), `--clip` from rolling buffer, timelapse with duration cap. |

**Phase 9 is the gate.** It resolves the daemon architecture, frame sharing, and device contention. Phases 10-12 can proceed in parallel after Phase 9. Phase 13 requires both Phase 9 (daemon) and Phase 5 (MQTT sensors from base plan).

---

## 12. What Would Break First (Risk-Ordered)

Based on the tech lead's failure scenario analysis, in order of likely occurrence:

| Day | Failure | Root Cause | Mitigation (in this spec) |
|---|---|---|---|
| 1 | Pi falls over running rec + stream simultaneously | No hardware capability tier enforcement | Section 6: auto-detected tiers |
| 3 | SD card fills up from overnight recording | No disk quota or pre-recording space check | Section 9: retention policy, N10 mitigations |
| 5 | Ctrl+C corrupts recording | SIGKILL instead of SIGINT to ffmpeg | Section 7: graceful shutdown coordinator |
| 7 | Rolling buffer ffmpeg dies silently | No health monitoring or restart logic | Daemon watchdog goroutine, systemd restart |
| 10 | Stream server leaks goroutines | No write timeout on client connections | HTTP write deadline, `goleak` in tests |
| 14 | Device contention between eye + recording | Multiple processes opening /dev/video0 | Section 2: daemon as single camera reader |

---

## 13. Config Additions

```toml
[vision]
tier = "auto"                          # "auto", "full", "lite", "minimal"
device = "/dev/video0"                 # camera device
fps = 15                               # frame buffer capture rate
buffer_duration = "30s"                # rolling buffer depth (0 to disable)
vaapi_device = "/dev/dri/renderD128"   # VA-API device (auto-detected if omitted)

[vision.stream]
enabled = false                        # must be explicitly enabled
port = 8080
bind = "tailscale"                     # "tailscale", "localhost", or specific IP
token = ""                             # bearer token (generated by qraft init if empty)
max_clients = 5

[vision.recording]
output_dir = "~/qraftworx/vault/raw"
codec = "auto"                         # "auto", "h264_vaapi", "h264_v4l2m2m", "libx264"
max_file_size_gb = 10
container = "mkv"                      # MKV for crash resilience

[vision.timelapse]
output_dir = "~/qraftworx/vault/timelapse"
max_duration = "24h"                   # hard cap

[vault]
max_total_gb = 100
max_file_age_days = 30
min_free_disk_gb = 2
```

---

## 14. Excluded from v1

- **HLS streaming.** MJPEG for v1. HLS only if client count or bandwidth becomes a problem.
- **Multi-camera support.** Single camera per daemon instance. Multi-camera requires multi-daemon with per-device config.
- **Cloud recording.** All storage is local. Cloud upload is a separate tool (the existing `upload_content` tool).
- **Audio capture.** Video only.
- **AI-triggered recording.** The Gemini tool loop can read camera state but cannot autonomously start recording. Recording start always requires either user command or config-defined event trigger. This is a security boundary, not a limitation.

---

## 15. Decision Log

| # | Decision | Rationale |
|---|---|---|
| NVS-D1 | Daemon as subcommand + systemd over separate binary | Version sync, shared packages, single build pipeline |
| NVS-D2 | Unix socket over mmap for frame sharing | Simpler, no sync primitives needed, sufficient for JPEG frames at 15-30fps |
| NVS-D3 | MJPEG over HLS for v1 streaming | Lower latency, zero transcoding CPU, simple implementation, sufficient for 1-3 clients |
| NVS-D4 | MKV container over MP4 | Crash-resilient: seekable on partial writes, no moov atom dependency |
| NVS-D5 | Hardware tier auto-detection | Prevents Pi from being overloaded by features it cannot sustain |
| NVS-D6 | Rolling buffer opt-in, disabled by default | Privacy: 30s continuous recording should not be on without explicit consent |
| NVS-D7 | Bearer token auth on stream server | Tailscale is network-layer, not application-layer. Defense in depth. |
| NVS-D8 | Trigger conditions from config only, never MQTT payloads | Prevents MQTT spoofing from triggering unauthorized recording |
| NVS-D9 | Eye requires confirmation gate, always | Camera capture is a surveillance act. No exceptions. |
| NVS-D10 | Separate capture audit log | Out-of-band from Gemini interaction log. Cannot be influenced by the model. |
