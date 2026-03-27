// Package gemini provides the Gemini API client, tool declaration builder,
// retry logic, and the tool execution loop for QraftWorx.
//
// # Key Types
//
//   - Client: wraps the Gemini GenerateContent API with retry, timeout, and
//     tool support. Reads GEMINI_API_KEY from the environment.
//   - GenerateContentResult: parsed API response containing text, function calls,
//     usage metadata, and finish reason.
//   - FunctionCall: a single tool invocation requested by the model (name + args).
//   - Part: interface for prompt parts (TextPart, FunctionResponsePart).
//   - ToolDeclarer: interface for types that can declare themselves as Gemini tools.
//   - ToolExecutor: interface for executing tools by name (satisfied by executor.Executor).
//   - RunLoop: the tool execution loop that orchestrates Generate -> Execute -> Respond
//     cycles until a text response or max iterations.
//
// # Architecture Role
//
// The gemini package is the bridge between QraftWorx and the Google Gemini API.
// The Hydrator formats context into Parts, which are sent via Client.Generate().
// When Gemini returns function calls, RunLoop dispatches them through the
// ToolExecutor and feeds results back as FunctionResponseParts.
//
// The tool loop integrates with the cost tracker (pre-call budget gate, post-call
// usage recording) and the logger (structured InteractionLog per interaction).
//
// # Retry Strategy
//
//   - 429 (rate limit): exponential backoff (1s, 2s, 4s, ...) up to maxRetries
//   - 500/503 (server error): single retry after 2s
//   - 400/other client errors: no retry, immediate failure
//   - Context cancellation: immediate failure
//
// # Security Considerations
//
//   - S6: Missing UsageMetadata is treated as maximum possible cost (MaxCostUsage sentinel).
//   - S10: Every Generate call enforces context.WithTimeout.
//   - Architectural rec #4: Function calls naming unregistered tools are rejected.
//   - S4: Tool call summaries use sanitized key names only, never raw argument values.
//
// # Testing
//
// Tests inject a mock generateFunc into Client, bypassing the real API entirely.
// The sleepFn field is set to a no-op for fast tests. atomic.Int32 tracks
// attempt counts for retry verification. No live API calls are made in tests.
package gemini
