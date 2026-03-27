package sensors

import (
	"context"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// --- Mock MQTT infrastructure ---

// mockToken implements mqtt.Token.
type mockToken struct {
	err error
}

func (t *mockToken) Wait() bool                       { return true }
func (t *mockToken) WaitTimeout(_ time.Duration) bool { return true }
func (t *mockToken) Done() <-chan struct{}            { ch := make(chan struct{}); close(ch); return ch }
func (t *mockToken) Error() error                     { return t.err }

// mockMessage implements mqtt.Message.
type mockMessage struct {
	topic   string
	payload []byte
}

func (m *mockMessage) Duplicate() bool   { return false }
func (m *mockMessage) Qos() byte         { return 0 }
func (m *mockMessage) Retained() bool    { return false }
func (m *mockMessage) Topic() string     { return m.topic }
func (m *mockMessage) MessageID() uint16 { return 0 }
func (m *mockMessage) Payload() []byte   { return m.payload }
func (m *mockMessage) Ack()              {}

// mockMQTTClient implements MQTTClient for testing.
type mockMQTTClient struct {
	connectErr   error
	subscribeErr error
	handler      mqtt.MessageHandler
}

func (m *mockMQTTClient) Connect() mqtt.Token {
	return &mockToken{err: m.connectErr}
}

func (m *mockMQTTClient) Subscribe(_ string, _ byte, callback mqtt.MessageHandler) mqtt.Token {
	m.handler = callback
	return &mockToken{err: m.subscribeErr}
}

func (m *mockMQTTClient) Disconnect(_ uint) {}
func (m *mockMQTTClient) IsConnected() bool { return true }

// simulateMessage pushes a message through the handler.
func (m *mockMQTTClient) simulateMessage(topic string, payload []byte) {
	if m.handler != nil {
		m.handler(nil, &mockMessage{topic: topic, payload: payload})
	}
}

// --- Tests ---

// Task 5.1
func TestMQTTConfig_RejectsPlaintext(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"tcp", "tcp://broker:1883"},
		{"mqtt", "mqtt://broker:1883"},
		{"ws", "ws://broker:8083"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := MQTTConfig{
				Name:          "test",
				BrokerURL:     tt.url,
				Topic:         "test/topic",
				AllowInsecure: false,
			}
			err := cfg.Validate()
			if err == nil {
				t.Error("expected error for plaintext URL without AllowInsecure")
			}
		})
	}

	// AllowInsecure = true should pass
	cfg := MQTTConfig{
		Name:          "test",
		BrokerURL:     "tcp://broker:1883",
		Topic:         "test/topic",
		AllowInsecure: true,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("AllowInsecure=true should accept plaintext: %v", err)
	}
}

// Task 5.2
func TestMQTTConfig_AcceptsMQTTS(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"ssl", "ssl://broker:8883"},
		{"tls", "tls://broker:8883"},
		{"mqtts", "mqtts://broker:8883"},
		{"wss", "wss://broker:8884"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := MQTTConfig{
				Name:      "test",
				BrokerURL: tt.url,
				Topic:     "test/topic",
			}
			if err := cfg.Validate(); err != nil {
				t.Errorf("secure URL %q should be accepted: %v", tt.url, err)
			}
		})
	}
}

// Task 5.3
func TestMQTTSensor_SubscribeAndCache(t *testing.T) {
	mock := &mockMQTTClient{}
	cfg := MQTTConfig{
		Name:          "temp-sensor",
		BrokerURL:     "ssl://broker:8883",
		Topic:         "sensors/temp",
		AllowInsecure: false,
	}

	sensor, err := NewMQTTSensor(&cfg, mock, nil)
	if err != nil {
		t.Fatalf("NewMQTTSensor: %v", err)
	}
	defer func() {
		if cerr := sensor.Close(); cerr != nil {
			t.Errorf("Close: %v", cerr)
		}
	}()

	// Simulate a message
	mock.simulateMessage("sensors/temp", []byte(`{"temperature": 42.5}`))

	// Poll should return cached value
	data, err := sensor.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil data after message")
	}
	temp, ok := data["temperature"].(float64)
	if !ok || temp != 42.5 {
		t.Errorf("temperature=%v, want 42.5", data["temperature"])
	}
}

// Task 5.4
func TestMQTTSensor_Poll_NoData(t *testing.T) {
	mock := &mockMQTTClient{}
	cfg := MQTTConfig{
		Name:      "temp-sensor",
		BrokerURL: "ssl://broker:8883",
		Topic:     "sensors/temp",
	}

	sensor, err := NewMQTTSensor(&cfg, mock, nil)
	if err != nil {
		t.Fatalf("NewMQTTSensor: %v", err)
	}
	defer func() {
		if cerr := sensor.Close(); cerr != nil {
			t.Errorf("Close: %v", cerr)
		}
	}()

	// No messages published yet
	data, err := sensor.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data before any message, got %v", data)
	}
}

// Task 5.5
func TestMQTTSensor_SchemaValidation(t *testing.T) {
	minTemp := 0.0
	maxTemp := 300.0
	schema := &ValueSchema{
		Fields: map[string]FieldSpec{
			"temperature": {Type: "float64", Min: &minTemp, Max: &maxTemp},
			"status":      {Type: "enum", Allowed: []string{"ok", "warning", "error"}},
		},
	}

	mock := &mockMQTTClient{}
	cfg := MQTTConfig{
		Name:      "temp-sensor",
		BrokerURL: "ssl://broker:8883",
		Topic:     "sensors/temp",
	}

	sensor, err := NewMQTTSensor(&cfg, mock, schema)
	if err != nil {
		t.Fatalf("NewMQTTSensor: %v", err)
	}
	defer func() {
		if cerr := sensor.Close(); cerr != nil {
			t.Errorf("Close: %v", cerr)
		}
	}()

	// Valid message should be cached
	mock.simulateMessage("sensors/temp", []byte(`{"temperature": 42.5, "status": "ok"}`))
	data, err := sensor.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if data == nil {
		t.Fatal("expected valid message to be cached")
	}

	// Out-of-range value should be rejected (not cached)
	mock.simulateMessage("sensors/temp", []byte(`{"temperature": 500.0, "status": "ok"}`))
	data2, err := sensor.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	// Should still have the old valid value, not the out-of-range one
	temp, _ := data2["temperature"].(float64)
	if temp != 42.5 {
		t.Errorf("expected old cached value 42.5, got %v (out-of-range should be rejected)", temp)
	}

	// Invalid enum should be rejected
	mock.simulateMessage("sensors/temp", []byte(`{"temperature": 50.0, "status": "invalid_status"}`))
	data3, err := sensor.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	status, _ := data3["status"].(string)
	if status != "ok" {
		t.Errorf("expected old cached status 'ok', got %q (invalid enum should be rejected)", status)
	}
}

// Task 5.6
func TestMQTTSensor_RejectsInjection(t *testing.T) {
	schema := &ValueSchema{
		Fields: map[string]FieldSpec{
			"temperature": {Type: "float64"},
		},
	}

	mock := &mockMQTTClient{}
	cfg := MQTTConfig{
		Name:      "temp-sensor",
		BrokerURL: "ssl://broker:8883",
		Topic:     "sensors/temp",
	}

	sensor, err := NewMQTTSensor(&cfg, mock, schema)
	if err != nil {
		t.Fatalf("NewMQTTSensor: %v", err)
	}
	defer func() {
		if cerr := sensor.Close(); cerr != nil {
			t.Errorf("Close: %v", cerr)
		}
	}()

	// String value in float64 field should be rejected (S3)
	mock.simulateMessage("sensors/temp", []byte(`{"temperature": "drop table sensors"}`))
	data, err := sensor.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if data != nil {
		t.Error("expected nil: string injection in float64 field should be rejected")
	}
}

func TestMQTTConfig_RejectsEmptyFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  MQTTConfig
	}{
		{"empty name", MQTTConfig{BrokerURL: "ssl://broker:8883", Topic: "t"}},
		{"empty broker", MQTTConfig{Name: "s", Topic: "t"}},
		{"empty topic", MQTTConfig{Name: "s", BrokerURL: "ssl://broker:8883"}},
		{"unknown scheme", MQTTConfig{Name: "s", BrokerURL: "ftp://broker:21", Topic: "t"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestMQTTSensor_InvalidJSON(t *testing.T) {
	mock := &mockMQTTClient{}
	cfg := MQTTConfig{
		Name:      "test",
		BrokerURL: "ssl://broker:8883",
		Topic:     "test/topic",
	}

	sensor, err := NewMQTTSensor(&cfg, mock, nil)
	if err != nil {
		t.Fatalf("NewMQTTSensor: %v", err)
	}
	defer func() {
		if cerr := sensor.Close(); cerr != nil {
			t.Errorf("Close: %v", cerr)
		}
	}()

	// Invalid JSON should be silently dropped
	mock.simulateMessage("test/topic", []byte(`not json`))
	data, err := sensor.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if data != nil {
		t.Error("expected nil: invalid JSON should be dropped")
	}
}

func TestValueSchema_ValidateValue(t *testing.T) {
	minVal := 0.0
	maxVal := 100.0
	schema := &ValueSchema{
		Fields: map[string]FieldSpec{
			"count":  {Type: "int", Min: &minVal, Max: &maxVal},
			"label":  {Type: "string"},
			"status": {Type: "enum", Allowed: []string{"on", "off"}},
		},
	}

	t.Run("valid int", func(t *testing.T) {
		err := schema.ValidateValue(map[string]any{"count": float64(42)})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("non-integer in int field", func(t *testing.T) {
		err := schema.ValidateValue(map[string]any{"count": 42.5})
		if err == nil {
			t.Error("expected error for non-integer value")
		}
	})

	t.Run("string in int field", func(t *testing.T) {
		err := schema.ValidateValue(map[string]any{"count": "abc"})
		if err == nil {
			t.Error("expected error for string in int field")
		}
	})

	t.Run("valid string", func(t *testing.T) {
		err := schema.ValidateValue(map[string]any{"label": "hello"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("non-string in string field", func(t *testing.T) {
		err := schema.ValidateValue(map[string]any{"label": 123.0})
		if err == nil {
			t.Error("expected error for non-string in string field")
		}
	})

	t.Run("valid enum", func(t *testing.T) {
		err := schema.ValidateValue(map[string]any{"status": "on"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid enum", func(t *testing.T) {
		err := schema.ValidateValue(map[string]any{"status": "maybe"})
		if err == nil {
			t.Error("expected error for invalid enum value")
		}
	})

	t.Run("non-string in enum field", func(t *testing.T) {
		err := schema.ValidateValue(map[string]any{"status": 42.0})
		if err == nil {
			t.Error("expected error for non-string in enum field")
		}
	})

	t.Run("nil schema is no-op", func(t *testing.T) {
		var s *ValueSchema
		err := s.ValidateValue(map[string]any{"anything": "goes"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing field is ok", func(t *testing.T) {
		err := schema.ValidateValue(map[string]any{"unknown_field": "value"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("unknown type in schema", func(t *testing.T) {
		badSchema := &ValueSchema{
			Fields: map[string]FieldSpec{
				"x": {Type: "complex128"},
			},
		}
		err := badSchema.ValidateValue(map[string]any{"x": "val"})
		if err == nil {
			t.Error("expected error for unknown type in schema")
		}
	})
}

func TestMQTTSensor_Name(t *testing.T) {
	mock := &mockMQTTClient{}
	cfg := MQTTConfig{
		Name:      "my-sensor",
		BrokerURL: "ssl://broker:8883",
		Topic:     "test/topic",
	}

	sensor, err := NewMQTTSensor(&cfg, mock, nil)
	if err != nil {
		t.Fatalf("NewMQTTSensor: %v", err)
	}
	defer func() {
		if cerr := sensor.Close(); cerr != nil {
			t.Errorf("Close: %v", cerr)
		}
	}()

	if got := sensor.Name(); got != "my-sensor" {
		t.Errorf("Name()=%q, want %q", got, "my-sensor")
	}
}
