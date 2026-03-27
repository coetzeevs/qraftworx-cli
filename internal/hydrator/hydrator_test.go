package hydrator

import (
	"context"
	"testing"

	"github.com/coetzeevs/cerebro/brain"
)

// mockSensorPoller implements SensorPoller for testing.
type mockSensorPoller struct {
	data map[string]any
}

func (m *mockSensorPoller) PollAll(_ context.Context) map[string]any {
	return m.data
}

// Task 3.14
func TestHydrator_Hydrate_BasicPrompt(t *testing.T) {
	h := New(nil, 8192) // no cerebro client — memories will be empty

	hc, err := h.Hydrate(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Hydrate: %v", err)
	}
	if hc.UserPrompt != "hello world" {
		t.Errorf("prompt=%q, want %q", hc.UserPrompt, "hello world")
	}
	if hc.SystemPrompt == "" {
		t.Error("expected non-empty system prompt")
	}
	if len(hc.Memories) != 0 {
		t.Errorf("expected 0 memories without cerebro, got %d", len(hc.Memories))
	}
}

// Task 3.15
func TestHydrator_Hydrate_TruncatesAtBudget(t *testing.T) {
	h := New(nil, 10) // very tight budget

	hc := &HydratedContext{
		UserPrompt:   "hi",
		SystemPrompt: "system",
		Memories: []brain.ScoredNode{
			{Node: brain.Node{Content: "high score memory with lots of content that takes many tokens"}, Score: 0.9},
			{Node: brain.Node{Content: "medium score"}, Score: 0.5},
			{Node: brain.Node{Content: "low score"}, Score: 0.3},
		},
	}

	// Estimate exceeds budget — should truncate
	est := h.estimateTokens(hc)
	if est <= 10 {
		t.Skipf("estimate %d already within budget, adjust test", est)
	}

	hc.truncateMemories(10)
	// After truncation, some memories should be removed
	if hc.TokenEstimate > 10 {
		t.Errorf("token estimate %d still exceeds budget 10", hc.TokenEstimate)
	}
}

// Task 3.16
func TestHydrator_Hydrate_NoMemories(t *testing.T) {
	h := New(nil, 8192)

	hc, err := h.Hydrate(context.Background(), "test prompt")
	if err != nil {
		t.Fatal(err)
	}
	if len(hc.Memories) != 0 {
		t.Errorf("expected nil/empty memories, got %d", len(hc.Memories))
	}
}

// Task 5.15
func TestHydrator_Hydrate_WithSensors(t *testing.T) {
	sensorData := map[string]any{
		"printer1": map[string]any{
			"extruder_temp_c": 205.3,
			"bed_temp_c":      60.1,
			"print_progress":  0.42,
			"state":           "printing",
			"filename":        "benchy.gcode",
		},
	}

	poller := &mockSensorPoller{data: sensorData}
	h := New(nil, 8192).WithSensors(poller)

	hc, err := h.Hydrate(context.Background(), "what is the printer status?")
	if err != nil {
		t.Fatalf("Hydrate: %v", err)
	}

	if hc.SensorState == nil {
		t.Fatal("expected non-nil SensorState")
	}

	printer, ok := hc.SensorState["printer1"].(map[string]any)
	if !ok {
		t.Fatal("expected printer1 in SensorState")
	}
	if printer["extruder_temp_c"] != 205.3 {
		t.Errorf("extruder_temp_c=%v, want 205.3", printer["extruder_temp_c"])
	}
	if printer["state"] != "printing" {
		t.Errorf("state=%v, want 'printing'", printer["state"])
	}
}

func TestHydrator_Hydrate_WithoutSensors(t *testing.T) {
	h := New(nil, 8192) // no sensors attached

	hc, err := h.Hydrate(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("Hydrate: %v", err)
	}

	if hc.SensorState != nil {
		t.Errorf("expected nil SensorState without sensors, got %v", hc.SensorState)
	}
}
