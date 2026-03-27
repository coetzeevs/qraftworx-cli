package config

import (
	"path/filepath"
	"testing"
	"time"
)

// Task 8.1: Load valid config
func TestLoad_ValidConfig(t *testing.T) {
	cfg, err := Load(filepath.Join("testdata", "valid.toml"))
	if err != nil {
		t.Fatalf("expected valid config to load, got: %v", err)
	}

	if cfg.Gemini.Model != "gemini-2.5-pro" {
		t.Errorf("Gemini.Model = %q, want %q", cfg.Gemini.Model, "gemini-2.5-pro")
	}
	if cfg.Gemini.MaxTokens != 8192 {
		t.Errorf("Gemini.MaxTokens = %d, want 8192", cfg.Gemini.MaxTokens)
	}
	if cfg.Gemini.MaxToolIterations != 10 {
		t.Errorf("Gemini.MaxToolIterations = %d, want 10", cfg.Gemini.MaxToolIterations)
	}
	if cfg.Gemini.Timeout.Duration != 30*time.Second {
		t.Errorf("Gemini.Timeout = %v, want 30s", cfg.Gemini.Timeout.Duration)
	}
	if cfg.Cerebro.ProjectDir != "~/.qraftworx/cerebro" {
		t.Errorf("Cerebro.ProjectDir = %q, want %q", cfg.Cerebro.ProjectDir, "~/.qraftworx/cerebro")
	}

	// Sensors
	if len(cfg.Sensors) != 2 {
		t.Fatalf("expected 2 sensors, got %d", len(cfg.Sensors))
	}
	moonraker, ok := cfg.Sensors["moonraker"]
	if !ok {
		t.Fatal("missing moonraker sensor")
	}
	if moonraker.Type != "moonraker" {
		t.Errorf("moonraker.Type = %q, want %q", moonraker.Type, "moonraker")
	}
	if moonraker.URL != "http://localhost:7125" {
		t.Errorf("moonraker.URL = %q, want %q", moonraker.URL, "http://localhost:7125")
	}
	if moonraker.PollTimeout.Duration != 5*time.Second {
		t.Errorf("moonraker.PollTimeout = %v, want 5s", moonraker.PollTimeout.Duration)
	}

	mqtt, ok := cfg.Sensors["mqtt"]
	if !ok {
		t.Fatal("missing mqtt sensor")
	}
	if mqtt.Type != "mqtt" {
		t.Errorf("mqtt.Type = %q, want %q", mqtt.Type, "mqtt")
	}
	if mqtt.BrokerURL != "tls://broker.example.com:8883" {
		t.Errorf("mqtt.BrokerURL = %q", mqtt.BrokerURL)
	}

	// Media
	if cfg.Media.Webcam.Type != "v4l2" {
		t.Errorf("Media.Webcam.Type = %q, want %q", cfg.Media.Webcam.Type, "v4l2")
	}
	if cfg.Media.GoPro.Type != "http" {
		t.Errorf("Media.GoPro.Type = %q, want %q", cfg.Media.GoPro.Type, "http")
	}

	// Logging
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "info")
	}

	// Cost
	if cfg.Cost.DailyBudgetUSD != 5.00 {
		t.Errorf("Cost.DailyBudgetUSD = %g, want 5", cfg.Cost.DailyBudgetUSD)
	}
	if cfg.Cost.WarnThresholdUSD != 4.00 {
		t.Errorf("Cost.WarnThresholdUSD = %g, want 4", cfg.Cost.WarnThresholdUSD)
	}
}

// Task 8.2: Load missing file
func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("testdata/nonexistent.toml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// Task 8.3: Load invalid TOML syntax
func TestLoad_InvalidSyntax(t *testing.T) {
	_, err := Load(filepath.Join("testdata", "invalid_syntax.toml"))
	if err == nil {
		t.Fatal("expected error for invalid syntax, got nil")
	}
}

// Task 8.4: Duration UnmarshalText
func TestDuration_UnmarshalText(t *testing.T) {
	cases := []struct {
		input    string
		expected time.Duration
	}{
		{"30s", 30 * time.Second},
		{"2m", 2 * time.Minute},
		{"1h", 1 * time.Hour},
		{"500ms", 500 * time.Millisecond},
		{"1m30s", 90 * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(tc.input))
			if err != nil {
				t.Fatalf("UnmarshalText(%q) error: %v", tc.input, err)
			}
			if d.Duration != tc.expected {
				t.Errorf("UnmarshalText(%q) = %v, want %v", tc.input, d.Duration, tc.expected)
			}
		})
	}
}

func TestDuration_UnmarshalText_Invalid(t *testing.T) {
	var d Duration
	err := d.UnmarshalText([]byte("not-a-duration"))
	if err == nil {
		t.Fatal("expected error for invalid duration, got nil")
	}
}
