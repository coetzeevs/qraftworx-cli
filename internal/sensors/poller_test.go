package sensors

import (
	"context"
	"testing"
	"time"
)

// mockProvider is a mock SensorProvider for testing the Poller.
type mockProvider struct {
	name  string
	data  map[string]any
	err   error
	delay time.Duration
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Poll(ctx context.Context) (map[string]any, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, nil //nolint:nilerr // context cancellation is graceful in mock
		}
	}
	return m.data, m.err
}

func (m *mockProvider) Close() error { return nil }

// Task 5.12
func TestPoller_PollAll_MultipleSensors(t *testing.T) {
	p1 := &mockProvider{
		name: "sensor-a",
		data: map[string]any{"temp": 42.0},
	}
	p2 := &mockProvider{
		name: "sensor-b",
		data: map[string]any{"humidity": 65.0},
	}

	poller := NewPoller(5*time.Second, p1, p2)
	result := poller.PollAll(context.Background())

	if len(result) != 2 {
		t.Fatalf("expected 2 sensors in result, got %d", len(result))
	}

	sensorA, ok := result["sensor-a"].(map[string]any)
	if !ok {
		t.Fatal("sensor-a not in result")
	}
	if sensorA["temp"] != 42.0 {
		t.Errorf("sensor-a temp=%v, want 42.0", sensorA["temp"])
	}

	sensorB, ok := result["sensor-b"].(map[string]any)
	if !ok {
		t.Fatal("sensor-b not in result")
	}
	if sensorB["humidity"] != 65.0 {
		t.Errorf("sensor-b humidity=%v, want 65.0", sensorB["humidity"])
	}
}

// Task 5.13
func TestPoller_PollAll_TimeoutSkipsSlow(t *testing.T) {
	fast := &mockProvider{
		name: "fast-sensor",
		data: map[string]any{"value": 1.0},
	}
	slow := &mockProvider{
		name:  "slow-sensor",
		data:  map[string]any{"value": 2.0},
		delay: 2 * time.Second,
	}

	poller := NewPoller(200*time.Millisecond, fast, slow)
	result := poller.PollAll(context.Background())

	// Fast sensor should be present
	if _, ok := result["fast-sensor"]; !ok {
		t.Error("expected fast-sensor in result")
	}

	// Slow sensor should be skipped (timeout)
	if _, ok := result["slow-sensor"]; ok {
		t.Error("expected slow-sensor to be skipped due to timeout")
	}
}

// Task 5.14
func TestPoller_PollAll_AllDown(t *testing.T) {
	down1 := &mockProvider{
		name: "down-1",
		data: nil, // returns nil, nil (unreachable)
	}
	down2 := &mockProvider{
		name: "down-2",
		data: nil,
	}

	poller := NewPoller(1*time.Second, down1, down2)
	result := poller.PollAll(context.Background())

	// Should return empty map, not nil
	if result == nil {
		t.Fatal("expected non-nil (empty) map")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestPoller_PollAll_NoProviders(t *testing.T) {
	poller := NewPoller(1 * time.Second)
	result := poller.PollAll(context.Background())

	if result == nil {
		t.Fatal("expected non-nil (empty) map")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestPoller_PollAll_ErrorSensor(t *testing.T) {
	errProvider := &mockProvider{
		name: "error-sensor",
		err:  context.DeadlineExceeded,
	}
	okProvider := &mockProvider{
		name: "ok-sensor",
		data: map[string]any{"status": "ok"},
	}

	poller := NewPoller(1*time.Second, errProvider, okProvider)
	result := poller.PollAll(context.Background())

	// Error sensor should be skipped
	if _, ok := result["error-sensor"]; ok {
		t.Error("expected error sensor to be skipped")
	}
	// OK sensor should be present
	if _, ok := result["ok-sensor"]; !ok {
		t.Error("expected ok-sensor in result")
	}
}
