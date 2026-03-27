package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeHomeDir(dir string) func() (string, error) {
	return func() (string, error) { return dir, nil }
}

func envWith(key, val string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		if k == key {
			return val, true
		}
		return "", false
	}
}

func envWithout() func(string) (string, bool) {
	return func(_ string) (string, bool) { return "", false }
}

// Task 8.11: Init creates config dir
func TestInitCommand_CreatesConfigDir(t *testing.T) {
	home := t.TempDir()
	var buf bytes.Buffer

	cmd := newInitCmd(fakeHomeDir(home), envWith("GEMINI_API_KEY", "fake-key"), &buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	configDir := filepath.Join(home, ".qraftworx")
	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("config dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory, got file")
	}
	// Check permissions (mask out type bits)
	perm := info.Mode().Perm()
	if perm != 0o700 {
		t.Errorf("config dir permissions = %o, want 0700", perm)
	}
}

// Task 8.12: Init writes default config
func TestInitCommand_WritesDefaultConfig(t *testing.T) {
	home := t.TempDir()
	var buf bytes.Buffer

	cmd := newInitCmd(fakeHomeDir(home), envWith("GEMINI_API_KEY", "fake-key"), &buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	configFile := filepath.Join(home, ".qraftworx", "config.toml")
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "gemini-2.5-pro") {
		t.Error("config should contain default model")
	}
	if !strings.Contains(content, "[gemini]") {
		t.Error("config should contain [gemini] section")
	}
	if !strings.Contains(content, "[logging]") {
		t.Error("config should contain [logging] section")
	}
	if !strings.Contains(content, "[cost]") {
		t.Error("config should contain [cost] section")
	}

	// Check file permissions
	info, err := os.Stat(configFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("config file permissions = %o, want 0600", info.Mode().Perm())
	}
}

// Task 8.13: Init creates log directory
func TestInitCommand_CreatesLogDir(t *testing.T) {
	home := t.TempDir()
	var buf bytes.Buffer

	cmd := newInitCmd(fakeHomeDir(home), envWith("GEMINI_API_KEY", "fake-key"), &buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	logDir := filepath.Join(home, ".qraftworx", "logs")
	info, err := os.Stat(logDir)
	if err != nil {
		t.Fatalf("log dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory, got file")
	}
	perm := info.Mode().Perm()
	if perm != 0o700 {
		t.Errorf("log dir permissions = %o, want 0700", perm)
	}
}

// Task 8.14: Init does not overwrite existing config
func TestInitCommand_DoesNotOverwrite(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".qraftworx")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(configDir, "config.toml")
	existingContent := "# my custom config\n"
	if err := os.WriteFile(configFile, []byte(existingContent), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	cmd := newInitCmd(fakeHomeDir(home), envWith("GEMINI_API_KEY", "fake-key"), &buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	// Verify existing content was preserved
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != existingContent {
		t.Errorf("existing config was overwritten: got %q, want %q", string(data), existingContent)
	}

	// Verify warning message was printed
	if !strings.Contains(buf.String(), "not overwriting") {
		t.Errorf("expected 'not overwriting' warning, got: %q", buf.String())
	}
}

// Task 8.15: Init warns no API key
func TestInitCommand_WarnsNoAPIKey(t *testing.T) {
	home := t.TempDir()
	var buf bytes.Buffer

	cmd := newInitCmd(fakeHomeDir(home), envWithout(), &buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	if !strings.Contains(buf.String(), "GEMINI_API_KEY is not set") {
		t.Errorf("expected GEMINI_API_KEY warning, got: %q", buf.String())
	}
}

func TestInitCommand_NoWarningWhenAPIKeySet(t *testing.T) {
	home := t.TempDir()
	var buf bytes.Buffer

	cmd := newInitCmd(fakeHomeDir(home), envWith("GEMINI_API_KEY", "fake-key"), &buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	if strings.Contains(buf.String(), "GEMINI_API_KEY") {
		t.Errorf("should not warn when API key is set, got: %q", buf.String())
	}
}
