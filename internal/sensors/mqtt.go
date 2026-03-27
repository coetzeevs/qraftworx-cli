package sensors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClient is a thin interface over mqtt.Client for testability.
type MQTTClient interface {
	Connect() mqtt.Token
	Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token
	Disconnect(quiesce uint)
	IsConnected() bool
}

// MQTTConfig configures an MQTT sensor.
type MQTTConfig struct {
	Name          string
	BrokerURL     string // S5: must be ssl:// or tls:// unless AllowInsecure
	Topic         string
	CACert        string // S5: CA cert path
	ClientCert    string // S5: client cert path
	ClientKey     string // S5: client key path
	Username      string
	Password      string
	AllowInsecure bool // S5: requires explicit opt-in for plaintext
}

// Validate checks MQTTConfig for security compliance (S5).
func (c *MQTTConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("mqtt config: name is required")
	}
	if c.BrokerURL == "" {
		return fmt.Errorf("mqtt config: broker URL is required")
	}
	if c.Topic == "" {
		return fmt.Errorf("mqtt config: topic is required")
	}

	lower := strings.ToLower(c.BrokerURL)
	isSecure := strings.HasPrefix(lower, "ssl://") || strings.HasPrefix(lower, "tls://") ||
		strings.HasPrefix(lower, "mqtts://") || strings.HasPrefix(lower, "wss://")
	isPlaintext := strings.HasPrefix(lower, "tcp://") || strings.HasPrefix(lower, "mqtt://") ||
		strings.HasPrefix(lower, "ws://")

	if isPlaintext && !c.AllowInsecure {
		return fmt.Errorf("mqtt config: plaintext broker URL %q requires AllowInsecure=true (S5)", c.BrokerURL)
	}
	if !isSecure && !isPlaintext {
		return fmt.Errorf("mqtt config: unsupported broker URL scheme %q", c.BrokerURL)
	}

	return nil
}

// ValueSchema defines expected types and ranges for MQTT values (S3).
type ValueSchema struct {
	Fields map[string]FieldSpec
}

// FieldSpec describes a single field's expected type, range, and allowed values.
type FieldSpec struct {
	Type    string   // "float64", "int", "string", "enum"
	Min     *float64 // numeric range (for float64 and int)
	Max     *float64
	Allowed []string // enum values (for "enum" type)
}

// ValidateValue checks a parsed JSON map against the schema (S3).
func (s *ValueSchema) ValidateValue(data map[string]any) error {
	if s == nil || s.Fields == nil {
		return nil
	}
	for fieldName, spec := range s.Fields {
		val, ok := data[fieldName]
		if !ok {
			continue // missing fields are not an error
		}
		if err := spec.validate(fieldName, val); err != nil {
			return err
		}
	}
	return nil
}

func (f *FieldSpec) validate(name string, val any) error {
	switch f.Type {
	case "float64":
		v, ok := val.(float64)
		if !ok {
			return fmt.Errorf("field %q: expected float64, got %T (S3)", name, val)
		}
		return f.checkRange(name, v)
	case "int":
		// JSON numbers are float64; accept if whole number
		v, ok := val.(float64)
		if !ok {
			return fmt.Errorf("field %q: expected int (numeric), got %T (S3)", name, val)
		}
		if v != float64(int64(v)) {
			return fmt.Errorf("field %q: expected integer, got %v (S3)", name, v)
		}
		return f.checkRange(name, v)
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("field %q: expected string, got %T (S3)", name, val)
		}
	case "enum":
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("field %q: expected enum string, got %T (S3)", name, val)
		}
		for _, a := range f.Allowed {
			if s == a {
				return nil
			}
		}
		return fmt.Errorf("field %q: value %q not in allowed set %v (S3)", name, s, f.Allowed)
	default:
		return fmt.Errorf("field %q: unknown type %q in schema", name, f.Type)
	}
	return nil
}

func (f *FieldSpec) checkRange(name string, v float64) error {
	if f.Min != nil && v < *f.Min {
		return fmt.Errorf("field %q: value %v below minimum %v (S3)", name, v, *f.Min)
	}
	if f.Max != nil && v > *f.Max {
		return fmt.Errorf("field %q: value %v above maximum %v (S3)", name, v, *f.Max)
	}
	return nil
}

// MQTTSensor subscribes to an MQTT topic and caches the last message.
type MQTTSensor struct {
	name   string
	topic  string
	client MQTTClient
	schema *ValueSchema

	mu        sync.RWMutex
	lastValue map[string]any
}

// NewMQTTSensor creates and connects an MQTTSensor from the given config.
// The client parameter allows injection for testing.
func NewMQTTSensor(cfg *MQTTConfig, client MQTTClient, schema *ValueSchema) (*MQTTSensor, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	s := &MQTTSensor{
		name:   cfg.Name,
		topic:  cfg.Topic,
		client: client,
		schema: schema,
	}

	// Connect
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("mqtt connect: %w", token.Error())
	}

	// Subscribe
	if token := client.Subscribe(cfg.Topic, 0, s.handleMessage); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("mqtt subscribe: %w", token.Error())
	}

	return s, nil
}

// handleMessage processes incoming MQTT messages and caches them.
func (s *MQTTSensor) handleMessage(_ mqtt.Client, msg mqtt.Message) {
	var data map[string]any
	if err := json.Unmarshal(msg.Payload(), &data); err != nil {
		// Invalid JSON is silently dropped (logged in production)
		return
	}

	// Validate against schema before caching
	if s.schema != nil {
		if err := s.schema.ValidateValue(data); err != nil {
			// Schema violation: do not cache (S3)
			return
		}
	}

	s.mu.Lock()
	s.lastValue = data
	s.mu.Unlock()
}

// Name returns the sensor name.
func (s *MQTTSensor) Name() string {
	return s.name
}

// Poll returns the last cached value.
// Returns nil, nil if no data has been received yet (graceful degradation).
func (s *MQTTSensor) Poll(_ context.Context) (map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.lastValue == nil {
		return nil, nil
	}

	// Return a copy to prevent mutation
	result := make(map[string]any, len(s.lastValue))
	for k, v := range s.lastValue {
		result[k] = v
	}
	return result, nil
}

// Close disconnects from the broker.
func (s *MQTTSensor) Close() error {
	s.client.Disconnect(250)
	return nil
}
