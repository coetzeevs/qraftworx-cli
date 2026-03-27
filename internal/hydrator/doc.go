// Package hydrator assembles context for Gemini API calls by combining the user
// prompt with Cerebro memories and live sensor state.
//
// # Key Types
//
//   - Hydrator: the main context assembler. Accepts a Cerebro client, sensor poller,
//     and token budget. Produces HydratedContext.
//   - HydratedContext: the assembled context containing user prompt, memories,
//     sensor state, system prompt, and token estimate.
//   - SensorPoller: interface for sensor data sources (satisfied by sensors.Poller).
//   - FormatForGemini: converts HydratedContext into gemini.Part slices for the API.
//
// # Architecture Role
//
// The hydrator is the first stage in the request pipeline. It sits between the
// user input and the Gemini client, enriching prompts with relevant context:
//
//  1. Searches Cerebro for memories matching the user prompt
//  2. Polls sensors for current hardware state (if configured)
//  3. Estimates total token count and truncates lowest-scored memories to stay
//     within the configured budget
//  4. Formats everything into Gemini-compatible Parts with security sanitization
//
// Search failure is non-fatal: the hydrator proceeds without memories rather
// than blocking the interaction. Similarly, sensor polling proceeds without
// data if sensors are unavailable.
//
// # Security Considerations
//
//   - S3 (Indirect Prompt Injection): Memories are wrapped in <memories> delimiters
//     with an explicit anti-injection preamble instructing the model to treat them
//     as historical context only. Memory content is length-capped to 2000 characters
//     and scanned for instruction-like patterns ("ignore previous", "disregard",
//     "new instructions", "system prompt") which are flagged with a warning prefix.
//
// # Testing
//
// Tests use nil Cerebro client (memories empty) and mock SensorPoller. Token
// budgeting and memory truncation are tested with synthetic ScoredNode slices.
package hydrator
