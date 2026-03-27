package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is the top-level configuration for Qraft.
// Loaded from ~/.qraftworx/config.toml.
type Config struct {
	Gemini  GeminiConfig            `toml:"gemini"`
	Cerebro CerebroConfig           `toml:"cerebro"`
	Sensors map[string]SensorConfig `toml:"sensors"`
	Media   MediaConfig             `toml:"media"`
	Logging LoggingConfig           `toml:"logging"`
	Cost    CostConfig              `toml:"cost"`
}

// GeminiConfig holds Gemini API settings.
type GeminiConfig struct {
	Model             string   `toml:"model"`
	MaxTokens         int      `toml:"max_tokens"`
	MaxToolIterations int      `toml:"max_tool_iterations"`
	Timeout           Duration `toml:"timeout"`
}

// CerebroConfig holds Cerebro memory settings.
type CerebroConfig struct {
	ProjectDir string `toml:"project_dir"`
}

// SensorConfig holds configuration for a single sensor source.
type SensorConfig struct {
	Type          string   `toml:"type"`
	URL           string   `toml:"url,omitempty"`
	BrokerURL     string   `toml:"broker_url,omitempty"`
	Topic         string   `toml:"topic,omitempty"`
	CACert        string   `toml:"ca_cert,omitempty"`
	ClientCert    string   `toml:"client_cert,omitempty"`
	ClientKey     string   `toml:"client_key,omitempty"`
	AllowInsecure bool     `toml:"allow_insecure_mqtt,omitempty"`
	PollTimeout   Duration `toml:"poll_timeout"`
}

// MediaConfig holds media device settings.
type MediaConfig struct {
	Webcam DeviceConfig `toml:"webcam"`
	GoPro  DeviceConfig `toml:"gopro"`
}

// DeviceConfig holds configuration for a single media device.
type DeviceConfig struct {
	Type       string `toml:"type,omitempty"`
	Device     string `toml:"device,omitempty"`
	URL        string `toml:"url,omitempty"`
	Resolution string `toml:"resolution,omitempty"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Path  string `toml:"path"`
	Level string `toml:"level"`
}

// CostConfig holds API cost tracking settings.
type CostConfig struct {
	DailyBudgetUSD   float64 `toml:"daily_budget_usd"`
	WarnThresholdUSD float64 `toml:"warn_threshold_usd"`
}

// Duration is a TOML-friendly wrapper around time.Duration.
type Duration struct {
	time.Duration
}

// UnmarshalText parses a duration string (e.g. "30s", "2m", "1h").
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", string(text), err)
	}
	d.Duration = parsed
	return nil
}

// Load reads and validates config from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
