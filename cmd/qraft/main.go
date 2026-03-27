package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "qraft",
		Short: "QraftWorx CLI - AI-powered content automation",
	}

	rootCmd.AddCommand(newInitCmd(os.UserHomeDir, os.LookupEnv, os.Stderr))

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
