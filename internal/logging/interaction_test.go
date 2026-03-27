package logging

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// Task 4.5: InteractionLog round-trip JSON serialization.

func TestInteractionLog_MarshalJSON(t *testing.T) {
	original := InteractionLog{
		Timestamp:     time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC),
		RequestID:     "req-001",
		UserPrompt:    "What is the PLA temperature?",
		MemoriesUsed:  3,
		SensorsPolled: map[string]bool{"temperature": true, "humidity": false},
		TokensSent:    150,
		TokensRecvd:   80,
		ToolCalls: []ToolCallLog{
			{
				Name:     "memory_search",
				Summary:  "searched for PLA temperature",
				Duration: 50 * time.Millisecond,
			},
		},
		GeminiLatency: 200 * time.Millisecond,
		TotalLatency:  300 * time.Millisecond,
		CostUSD:       0.0025,
		Error:         "",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var roundTripped InteractionLog
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if original.RequestID != roundTripped.RequestID {
		t.Errorf("RequestID: got %q, want %q", roundTripped.RequestID, original.RequestID)
	}
	if original.UserPrompt != roundTripped.UserPrompt {
		t.Errorf("UserPrompt: got %q, want %q", roundTripped.UserPrompt, original.UserPrompt)
	}
	if original.MemoriesUsed != roundTripped.MemoriesUsed {
		t.Errorf("MemoriesUsed: got %d, want %d", roundTripped.MemoriesUsed, original.MemoriesUsed)
	}
	if original.TokensSent != roundTripped.TokensSent {
		t.Errorf("TokensSent: got %d, want %d", roundTripped.TokensSent, original.TokensSent)
	}
	if original.TokensRecvd != roundTripped.TokensRecvd {
		t.Errorf("TokensRecvd: got %d, want %d", roundTripped.TokensRecvd, original.TokensRecvd)
	}
	if original.CostUSD != roundTripped.CostUSD {
		t.Errorf("CostUSD: got %f, want %f", roundTripped.CostUSD, original.CostUSD)
	}
	if !reflect.DeepEqual(original.SensorsPolled, roundTripped.SensorsPolled) {
		t.Errorf("SensorsPolled mismatch")
	}
	if len(roundTripped.ToolCalls) != 1 {
		t.Fatalf("ToolCalls: got %d, want 1", len(roundTripped.ToolCalls))
	}
	if roundTripped.ToolCalls[0].Name != "memory_search" {
		t.Errorf("ToolCall.Name: got %q, want %q", roundTripped.ToolCalls[0].Name, "memory_search")
	}
}

func TestInteractionLog_MarshalJSON_WithError(t *testing.T) {
	log := InteractionLog{
		Timestamp:  time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC),
		RequestID:  "req-002",
		UserPrompt: "fail",
		Error:      "budget exhausted",
	}

	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	if m["error"] != "budget exhausted" {
		t.Errorf("error = %v, want 'budget exhausted'", m["error"])
	}
}

func TestInteractionLog_MarshalJSON_OmitsEmptyError(t *testing.T) {
	log := InteractionLog{
		Timestamp:  time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC),
		RequestID:  "req-003",
		UserPrompt: "ok",
	}

	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	if _, exists := m["error"]; exists {
		t.Errorf("expected 'error' key to be omitted when empty, got: %v", m["error"])
	}
}

// Task 4.6: ToolCallLog uses summary, NOT raw args.

func TestToolCallLog_NoRawArgs(t *testing.T) {
	// Verify ToolCallLog has no json.RawMessage field via reflection
	typ := reflect.TypeOf(ToolCallLog{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type == reflect.TypeOf(json.RawMessage{}) {
			t.Errorf("ToolCallLog has json.RawMessage field %q — raw args must not be logged (S4)", field.Name)
		}
	}
}

func TestToolCallLog_SummaryIsString(t *testing.T) {
	tc := ToolCallLog{
		Name:     "memory_search",
		Summary:  "searched for PLA settings",
		Duration: 100 * time.Millisecond,
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	summary, ok := m["summary"].(string)
	if !ok {
		t.Fatalf("summary is not a string: %T", m["summary"])
	}
	if summary != "searched for PLA settings" {
		t.Errorf("summary = %q, want 'searched for PLA settings'", summary)
	}
}

func TestToolCallLog_OmitsEmptyError(t *testing.T) {
	tc := ToolCallLog{
		Name:     "memory_search",
		Summary:  "ok",
		Duration: 50 * time.Millisecond,
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if _, exists := m["error"]; exists {
		t.Errorf("expected 'error' to be omitted when empty")
	}
}
