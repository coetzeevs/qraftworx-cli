// Package config handles TOML configuration loading and validation for QraftWorx.
//
// # Key Types
//
//   - Config: top-level configuration loaded from ~/.qraftworx/config.toml.
//     Contains sections for Gemini, Cerebro, Sensors, Media, Logging, and Cost.
//   - GeminiConfig: model name, max tokens, tool iteration limit, timeout.
//   - CerebroConfig: project directory path.
//   - SensorConfig: per-sensor configuration (type, URL, broker, TLS certs, timeout).
//   - MediaConfig: webcam and GoPro device settings.
//   - LoggingConfig: log file path and level.
//   - CostConfig: daily budget and warning threshold in USD.
//   - Duration: TOML-friendly wrapper around time.Duration with UnmarshalText.
//   - Load: reads and validates config from a file path.
//
// # Architecture Role
//
// The config package is loaded at startup by the CLI entry point. It provides
// validated configuration to all other packages. Validation runs automatically
// during Load() and reports all errors at once (multi-error accumulation).
//
// Default values are applied for optional fields:
//   - MaxTokens: 8192
//   - MaxToolIterations: 10
//   - Timeout: 30s
//   - Logging.Level: "info"
//   - Logging.Path: "~/.qraftworx/logs"
//   - DailyBudgetUSD: 5.00
//   - WarnThresholdUSD: 4.00
//   - Cerebro.ProjectDir: "~/.qraftworx/cerebro"
//
// # Validation Rules
//
//   - gemini.model is required (no default)
//   - logging.level must be one of: debug, info, warn, error
//   - cost.daily_budget_usd must not be negative
//   - cost.warn_threshold_usd must not be negative
//   - S5: MQTT sensors with plaintext broker URLs (tcp://) require explicit
//     allow_insecure_mqtt = true
//
// # Testing
//
// Tests use TOML fixture files in testdata/. Validation tests construct Config
// structs directly and verify error messages. Duration parsing is tested with
// various format strings (30s, 2m, 1h, 500ms, 1m30s).
package config
