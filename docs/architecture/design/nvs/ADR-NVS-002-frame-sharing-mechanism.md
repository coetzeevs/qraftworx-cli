# ADR-NVS-002: Frame Sharing Mechanism

## Status
Proposed

## Context

The vision daemon captures frames continuously. Multiple consumers need access to the latest frame:
- `qraft eye` needs a single high-quality JPEG (short-lived CLI process)
- `qraft rec` needs a continuous raw frame stream (long-lived ffmpeg process)
- `qraft stream` needs a continuous MJPEG stream (long-lived HTTP server)

We need a mechanism for the daemon to share frames with these consumers. The original proposal suggested memory-mapped files or Unix sockets.

Three options were evaluated:
1. Memory-mapped file (mmap) with a shared ring buffer
2. Unix domain socket with a request-response protocol
3. Unix domain socket with a streaming protocol (chosen hybrid)

## Decision

**Unix domain socket with a dual-mode protocol: request-response for single frames, streaming for continuous consumers.**

The daemon listens on a Unix domain socket at a well-known path (`/run/qraft/vision.sock` or `~/.qraftworx/run/vision.sock` if not running as root). Consumers connect and issue commands:

- `FRAME` -- returns the latest JPEG-encoded frame (for `qraft eye`)
- `STREAM` -- begins streaming MJPEG frames until the client disconnects (for recording, streaming)
- `STATUS` -- returns daemon health as JSON (camera state, fps, buffer depth)

### Why not mmap

Memory-mapped files are fast but create significant complexity in Go:
- Go's `mmap` support is via `syscall.Mmap` or third-party packages -- no standard library support
- Reader-writer synchronization across processes requires a separate futex/semaphore mechanism (another mmap or a Unix socket anyway)
- Frame boundaries in a ring buffer require a header protocol that is effectively reimplementing a socket protocol but with shared memory
- Debugging mmap corruption is significantly harder than debugging socket protocol errors
- The latency difference is irrelevant: a 1920x1080 JPEG is ~200-400KB, which transfers over a Unix socket in <1ms on localhost

Unix sockets give us:
- Standard Go `net.Listener`/`net.Conn` with full stdlib support
- Built-in flow control (kernel manages backpressure)
- Clean process isolation (daemon crash does not corrupt consumer memory)
- Standard tooling for debugging (`socat`, `nc`)
- Filesystem permissions for access control

### Protocol

```
Client -> Daemon: "FRAME\n"
Daemon -> Client: <4-byte big-endian length><JPEG bytes>

Client -> Daemon: "STREAM\n"
Daemon -> Client: <4-byte big-endian length><JPEG bytes> (repeating until disconnect)

Client -> Daemon: "STATUS\n"
Daemon -> Client: <4-byte big-endian length><JSON bytes>
```

The length-prefixed framing avoids delimiter scanning and supports binary payloads efficiently.

## Context7 References
- Library: N/A (standard library `net`, `encoding/binary`)
- No third-party library needed for this mechanism.

## Consequences

### Positive
- Pure Go standard library implementation -- no CGO, no third-party deps
- Clean process isolation between daemon and consumers
- Debuggable with standard Unix tooling
- Filesystem permissions on the socket provide access control
- Backpressure handled by kernel (slow consumer does not corrupt fast producer)

### Negative
- One memory copy per frame delivery (kernel copies from daemon to consumer). For a 400KB JPEG at 30fps this is ~12MB/s -- negligible on any modern hardware.
- Requires the daemon to be running before any consumer command works. CLI commands must detect "daemon not running" and give a clear error.

### Risks
- **Risk:** Socket file left behind after unclean daemon shutdown.
  **Mitigation:** Daemon removes socket on startup if it exists and no process is listening. systemd `RemoveOnStop=yes` in the socket unit.

## Alternatives Considered

1. **Memory-mapped ring buffer** -- Rejected. Requires process synchronization primitives (futex/semaphore) that are not in Go's standard library. Adds complexity without meaningful latency benefit for frame sizes under 1MB.
2. **Named pipes (FIFO)** -- Rejected. Single-reader only. Cannot serve multiple consumers simultaneously.
3. **D-Bus** -- Rejected. Heavyweight IPC framework. Not appropriate for binary frame data at 30fps.
4. **gRPC over Unix socket** -- Considered viable but rejected for v1. Adds protobuf code generation dependency. Can be adopted later if the protocol grows complex enough to warrant it. The simple length-prefixed protocol is sufficient for 3 message types.
