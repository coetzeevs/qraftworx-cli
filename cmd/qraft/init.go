package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const defaultConfigTOML = `[gemini]
model = "gemini-2.5-pro"
max_tokens = 8192
max_tool_iterations = 10
timeout = "30s"

[cerebro]
project_dir = "~/.qraftworx/cerebro"

[logging]
path = "~/.qraftworx/logs"
level = "info"

[cost]
daily_budget_usd = 5.00
warn_threshold_usd = 4.00
`

// newInitCmd creates the "init" subcommand which scaffolds
// the ~/.qraftworx/ directory with a default config.
func newInitCmd(homeDir func() (string, error), lookupEnv func(string) (string, bool), w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize QraftWorx configuration directory",
		Long:  "Creates ~/.qraftworx/ with a default config.toml and log directory.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := homeDir()
			if err != nil {
				return fmt.Errorf("determining home directory: %w", err)
			}

			configDir := filepath.Join(home, ".qraftworx")
			configFile := filepath.Join(configDir, "config.toml")
			logDir := filepath.Join(configDir, "logs")

			// Create config directory with 0o700 (S4)
			if err := os.MkdirAll(configDir, 0o700); err != nil {
				return fmt.Errorf("creating config directory: %w", err)
			}

			// Create log directory with 0o700 (S4)
			if err := os.MkdirAll(logDir, 0o700); err != nil {
				return fmt.Errorf("creating log directory: %w", err)
			}

			// Do NOT overwrite existing config
			if _, err := os.Stat(configFile); err == nil {
				_, _ = fmt.Fprintf(w, "Config already exists at %s — not overwriting.\n", configFile)
			} else {
				if err := os.WriteFile(configFile, []byte(defaultConfigTOML), 0o600); err != nil {
					return fmt.Errorf("writing config: %w", err)
				}
				_, _ = fmt.Fprintf(w, "Config written to %s\n", configFile)
			}

			// Warn if GEMINI_API_KEY not set
			if _, ok := lookupEnv("GEMINI_API_KEY"); !ok {
				_, _ = fmt.Fprintf(w, "Warning: GEMINI_API_KEY is not set. Set it before running qraft.\n")
			}

			return nil
		},
	}
}
