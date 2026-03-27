# ADR-NVS-001: Vision Daemon Process Model

## Status
Proposed

## Context

The NVS feature requires a long-lived process that keeps a camera device open continuously while serving frames to multiple short-lived consumers (CLI commands like `qraft eye`, `qraft rec`, `qraft stream`). Qraft is currently designed as a single-binary CLI with a request-response model. We must decide how the daemon fits into this architecture.

Three options were evaluated:
1. Separate binary (`qraftd`) managed by systemd
2. Subcommand within qraft binary (`qraft daemon start`) managed by systemd
3. Fork-and-detach from CLI invocation

## Decision

**Option 2: Subcommand within the qraft binary, managed by systemd.**

The vision daemon is invoked as `qraft vision daemon` and runs as a long-lived process. It is managed by a systemd unit file, not by daemonization logic in Go. The CLI binary is the same binary -- there is no separate build artifact.

### Rationale

| Factor | Separate binary | Subcommand (chosen) | Fork-and-detach |
|--------|----------------|---------------------|-----------------|
| Build complexity | Two binaries, two build targets, two release artifacts | One binary, one GoReleaser config | One binary but fork() in Go is undefined behavior with goroutines |
| Deployment | Two files to scp to Pi | One file to scp to Pi | One file, but process management is in-process |
| Shared code | Shared via Go module import | Shared natively within the same binary | Same binary |
| Process management | systemd | systemd | Custom PID file, signal handling, orphan processes |
| Version skew | CLI and daemon can drift | Always the same version | Same version |
| Testability | Integration tests need two binaries | One binary, same test harness | Fork makes testing fragile |

Fork-and-detach is rejected outright. Go's runtime does not support `fork()` safely -- goroutines, the garbage collector, and file descriptors all break across a fork boundary. This is a well-documented Go limitation.

A separate binary adds build complexity and version skew risk for no benefit. The daemon is not a general-purpose service -- it exists solely to serve Qraft's vision subsystem. Keeping it in the same binary means `qraft vision daemon --version` always matches `qraft --version`.

## Context7 References
- Library: `google.golang.org/genai` (Go Gemini SDK)
- Verified: Multimodal `InlineData` with `genai.Blob{Data: []byte, MIMEType: "image/jpeg"}` is the correct pattern for sending camera frames to Gemini.

## Consequences

### Positive
- Single binary deployment model preserved
- No version skew between CLI and daemon
- systemd handles restart, logging, resource limits -- no custom process management code
- The daemon subcommand is testable with standard Go test patterns

### Negative
- The binary size increases (daemon code compiled in even when not used)
- systemd is a hard dependency for daemon management (acceptable: target is Linux only)

### Risks
- **Risk:** Daemon crashes take down the camera feed for all consumers.
  **Mitigation:** systemd `Restart=on-failure` with `RestartSec=2s`. The daemon must be stateless enough that a restart recovers cleanly (re-open camera device, re-establish shared memory).

## Alternatives Considered

1. **Separate binary (`qraftd`)** -- Rejected because it doubles build/release complexity and introduces version skew for a component that is tightly coupled to the CLI.
2. **Fork-and-detach** -- Rejected because Go's runtime does not support `fork()` safely. Goroutine scheduling and GC break across fork boundaries.
3. **No daemon (open camera per-command)** -- Rejected because USB camera initialization takes 1-3 seconds, camera device can only be opened by one process at a time, and the rolling buffer for `--clip` requires a continuously-running capture loop.
