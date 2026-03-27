// Package executor runs tools with confirmation gates, permission checks,
// panic recovery, and per-interaction error caps.
//
// # Key Types
//
//   - Executor: the tool execution engine. Wraps a tools.Registry and enforces
//     security policies before dispatching to tool implementations.
//   - ConfirmFunc: injectable function for user confirmation prompts. Receives
//     the tool name and a sanitized summary of the action.
//   - Option: functional options for configuring the Executor (WithConfirmFunc,
//     WithTTY, WithMaxErrors).
//
// # Sentinel Errors
//
//   - ErrToolNotFound: requested tool is not in the registry.
//   - ErrNonInteractiveDenied: confirmation-required tool denied in non-TTY mode (S1).
//   - ErrUserDenied: user declined the confirmation prompt.
//   - ErrTooManyErrors: per-interaction error cap (default: 3) reached.
//
// # Architecture Role
//
// The Executor is the security gateway between the Gemini tool loop and the
// actual tool implementations. Every tool call passes through Execute(), which:
//
//  1. Checks the per-interaction error cap
//  2. Looks up the tool in the registry
//  3. Sets up panic recovery
//  4. Enforces the confirmation gate (if RequiresConfirmation() is true)
//  5. Dispatches to the tool's Execute method
//  6. Increments the error counter on failure
//
// The Executor satisfies the gemini.ToolExecutor interface.
//
// # Security Considerations
//
//   - S1 (Confirmation Gate): Tools with RequiresConfirmation()=true are blocked
//     in non-TTY environments (ErrNonInteractiveDenied). When running in a TTY,
//     the injectable ConfirmFunc is called with ANSI-stripped arguments.
//   - S1 (ANSI Injection): stripControlChars removes all ANSI escape sequences
//     and non-printable characters from tool arguments before display, preventing
//     terminal manipulation attacks.
//   - Panic Recovery: Tool panics are caught by defer/recover, logged, and
//     returned as errors rather than crashing the process.
//   - Error Cap: After maxErrors (default: 3) tool failures in a single
//     interaction, all subsequent calls return ErrTooManyErrors to prevent
//     infinite error loops.
//
// # Testing
//
// Tests use mock Tool implementations with injectable execFn. TTY detection is
// controlled via WithTTY(). ConfirmFunc is injected to test approval/denial paths.
// Panic recovery is tested with a tool that calls panic().
package executor
