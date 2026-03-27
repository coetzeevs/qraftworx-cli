package logging

import (
	"time"
)

// InteractionLog is the structured record of one Qraft interaction.
// All fields are JSON-serializable for structured logging.
type InteractionLog struct {
	Timestamp     time.Time       `json:"ts"`
	RequestID     string          `json:"request_id"`
	UserPrompt    string          `json:"user_prompt"`
	MemoriesUsed  int             `json:"memories_used"`
	SensorsPolled map[string]bool `json:"sensors_polled"`
	TokensSent    int             `json:"tokens_sent"`
	TokensRecvd   int             `json:"tokens_received"`
	ToolCalls     []ToolCallLog   `json:"tool_calls"`
	GeminiLatency time.Duration   `json:"gemini_latency_ms"`
	TotalLatency  time.Duration   `json:"total_latency_ms"`
	CostUSD       float64         `json:"cost_usd"`
	Error         string          `json:"error,omitempty"`
}

// ToolCallLog records a single tool invocation.
// Uses a sanitized Summary string, NOT raw args (S4).
type ToolCallLog struct {
	Name     string        `json:"name"`
	Summary  string        `json:"summary"` // sanitized, NOT raw args (S4)
	Duration time.Duration `json:"duration_ms"`
	Error    string        `json:"error,omitempty"`
}
