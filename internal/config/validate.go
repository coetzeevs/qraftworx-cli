package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// validLogLevels enumerates the accepted log levels.
var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// Validate checks all config values for correctness.
// Returns a multi-error with all validation failures.
// It also populates defaults for unset optional fields.
func (c *Config) Validate() error {
	c.applyDefaults()

	var errs []error

	if c.Gemini.Model == "" {
		errs = append(errs, fmt.Errorf("gemini.model is required"))
	}

	if c.Logging.Level != "" && !validLogLevels[c.Logging.Level] {
		errs = append(errs, fmt.Errorf("logging.level %q is not valid; must be one of: debug, info, warn, error", c.Logging.Level))
	}

	if c.Cost.DailyBudgetUSD < 0 {
		errs = append(errs, fmt.Errorf("cost.daily_budget_usd must not be negative, got %g", c.Cost.DailyBudgetUSD))
	}

	if c.Cost.WarnThresholdUSD < 0 {
		errs = append(errs, fmt.Errorf("cost.warn_threshold_usd must not be negative, got %g", c.Cost.WarnThresholdUSD))
	}

	// S5: MQTT plaintext without explicit opt-in
	for name := range c.Sensors {
		sensor := c.Sensors[name]
		if sensor.Type == "mqtt" && sensor.BrokerURL != "" {
			if strings.HasPrefix(sensor.BrokerURL, "tcp://") && !sensor.AllowInsecure {
				errs = append(errs, fmt.Errorf(
					"sensors.%s: plaintext MQTT broker_url %q requires allow_insecure_mqtt = true",
					name, sensor.BrokerURL,
				))
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// applyDefaults populates unset optional fields with sensible defaults.
func (c *Config) applyDefaults() {
	if c.Gemini.MaxTokens == 0 {
		c.Gemini.MaxTokens = 8192
	}
	if c.Gemini.MaxToolIterations == 0 {
		c.Gemini.MaxToolIterations = 10
	}
	if c.Gemini.Timeout.Duration == 0 {
		c.Gemini.Timeout.Duration = 30 * time.Second
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Path == "" {
		c.Logging.Path = "~/.qraftworx/logs"
	}
	if c.Cost.DailyBudgetUSD == 0 {
		c.Cost.DailyBudgetUSD = 5.00
	}
	if c.Cost.WarnThresholdUSD == 0 {
		c.Cost.WarnThresholdUSD = 4.00
	}
	if c.Cerebro.ProjectDir == "" {
		c.Cerebro.ProjectDir = "~/.qraftworx/cerebro"
	}
}
