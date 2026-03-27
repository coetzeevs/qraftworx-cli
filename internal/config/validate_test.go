package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Task 8.5: Validate missing model
func TestValidate_MissingModel(t *testing.T) {
	cfg := Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing model, got nil")
	}
	if !strings.Contains(err.Error(), "gemini.model is required") {
		t.Errorf("error should mention gemini.model, got: %v", err)
	}
}

// Task 8.6: Validate invalid log level
func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := Config{
		Gemini: GeminiConfig{Model: "gemini-2.5-pro"},
		Logging: LoggingConfig{
			Level: "verbose",
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid log level, got nil")
	}
	if !strings.Contains(err.Error(), "logging.level") {
		t.Errorf("error should mention logging.level, got: %v", err)
	}
}

// Task 8.7: Validate negative budget
func TestValidate_NegativeBudget(t *testing.T) {
	cfg := Config{
		Gemini: GeminiConfig{Model: "gemini-2.5-pro"},
		Cost: CostConfig{
			DailyBudgetUSD: -1.0,
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative budget, got nil")
	}
	if !strings.Contains(err.Error(), "daily_budget_usd") {
		t.Errorf("error should mention daily_budget_usd, got: %v", err)
	}
}

// Task 8.8: Validate MQTT plaintext without opt-in (S5)
func TestValidate_MQTTPlaintextWithoutOptIn(t *testing.T) {
	cfg := Config{
		Gemini: GeminiConfig{Model: "gemini-2.5-pro"},
		Sensors: map[string]SensorConfig{
			"mqtt": {
				Type:      "mqtt",
				BrokerURL: "tcp://broker.example.com:1883",
				Topic:     "printer/telemetry",
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for plaintext MQTT without opt-in, got nil")
	}
	if !strings.Contains(err.Error(), "allow_insecure_mqtt") {
		t.Errorf("error should mention allow_insecure_mqtt, got: %v", err)
	}
}

func TestValidate_MQTTPlaintextWithOptIn(t *testing.T) {
	cfg := Config{
		Gemini: GeminiConfig{Model: "gemini-2.5-pro"},
		Sensors: map[string]SensorConfig{
			"mqtt": {
				Type:          "mqtt",
				BrokerURL:     "tcp://broker.example.com:1883",
				Topic:         "printer/telemetry",
				AllowInsecure: true,
			},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected plaintext MQTT with opt-in to pass, got: %v", err)
	}
}

func TestValidate_MQTTPlaintextFromFile(t *testing.T) {
	// This loads the mqtt_plaintext.toml fixture which has tcp:// without allow_insecure_mqtt
	_, err := Load(filepath.Join("testdata", "mqtt_plaintext.toml"))
	if err == nil {
		t.Fatal("expected error loading mqtt_plaintext.toml, got nil")
	}
	if !strings.Contains(err.Error(), "allow_insecure_mqtt") {
		t.Errorf("error should mention allow_insecure_mqtt, got: %v", err)
	}
}

// Task 8.9: Validate multi-error accumulation
func TestValidate_MultipleErrors(t *testing.T) {
	cfg := Config{
		// Missing model
		Logging: LoggingConfig{
			Level: "verbose", // invalid level
		},
		Cost: CostConfig{
			DailyBudgetUSD: -1.0, // negative budget
		},
		Sensors: map[string]SensorConfig{
			"mqtt": {
				Type:      "mqtt",
				BrokerURL: "tcp://broker.example.com:1883",
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected multiple errors, got nil")
	}

	errStr := err.Error()
	expectedParts := []string{
		"gemini.model is required",
		"logging.level",
		"daily_budget_usd",
		"allow_insecure_mqtt",
	}
	for _, part := range expectedParts {
		if !strings.Contains(errStr, part) {
			t.Errorf("multi-error should contain %q, got: %v", part, errStr)
		}
	}
}

// Task 8.10: Validate defaults populated
func TestValidate_Defaults(t *testing.T) {
	cfg := Config{
		Gemini: GeminiConfig{
			Model: "gemini-2.5-pro",
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected minimal valid config to pass, got: %v", err)
	}

	if cfg.Gemini.MaxTokens != 8192 {
		t.Errorf("default MaxTokens = %d, want 8192", cfg.Gemini.MaxTokens)
	}
	if cfg.Gemini.MaxToolIterations != 10 {
		t.Errorf("default MaxToolIterations = %d, want 10", cfg.Gemini.MaxToolIterations)
	}
	if cfg.Gemini.Timeout.Duration != 30*time.Second {
		t.Errorf("default Timeout = %v, want 30s", cfg.Gemini.Timeout.Duration)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("default Logging.Level = %q, want %q", cfg.Logging.Level, "info")
	}
	if cfg.Logging.Path != "~/.qraftworx/logs" {
		t.Errorf("default Logging.Path = %q, want %q", cfg.Logging.Path, "~/.qraftworx/logs")
	}
	if cfg.Cost.DailyBudgetUSD != 5.00 {
		t.Errorf("default DailyBudgetUSD = %g, want 5", cfg.Cost.DailyBudgetUSD)
	}
	if cfg.Cost.WarnThresholdUSD != 4.00 {
		t.Errorf("default WarnThresholdUSD = %g, want 4", cfg.Cost.WarnThresholdUSD)
	}
	if cfg.Cerebro.ProjectDir != "~/.qraftworx/cerebro" {
		t.Errorf("default Cerebro.ProjectDir = %q, want %q", cfg.Cerebro.ProjectDir, "~/.qraftworx/cerebro")
	}
}

// Also test missing_model.toml fixture
func TestLoad_MissingModel(t *testing.T) {
	_, err := Load(filepath.Join("testdata", "missing_model.toml"))
	if err == nil {
		t.Fatal("expected error for missing model, got nil")
	}
	if !strings.Contains(err.Error(), "gemini.model is required") {
		t.Errorf("error should mention gemini.model, got: %v", err)
	}
}
