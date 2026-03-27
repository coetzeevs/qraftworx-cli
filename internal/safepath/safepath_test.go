package safepath

import (
	"os"
	"path/filepath"
	"testing"
)

// Task 1.1: reject traversal, relative paths, escaping symlinks

func TestNew_RejectsTraversal(t *testing.T) {
	base := t.TempDir()

	cases := []struct {
		name string
		raw  string
	}{
		{"dotdot in middle", filepath.Join(base, "subdir", "..", "..", "etc", "passwd")},
		{"dotdot at start", filepath.Join(base, "..", "escape")},
		{"double slash dotdot", base + "/foo/../../bar"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.raw, []string{base})
			if err == nil {
				t.Errorf("expected error for path %q, got nil", tc.raw)
			}
		})
	}
}

func TestNew_RejectsRelativePath(t *testing.T) {
	base := t.TempDir()

	relatives := []string{"relative/path", "./here", "../there", "just-a-name"}
	for _, raw := range relatives {
		t.Run(raw, func(t *testing.T) {
			_, err := New(raw, []string{base})
			if err == nil {
				t.Errorf("expected error for relative path %q, got nil", raw)
			}
		})
	}
}

func TestNew_RejectsEscapingSymlink(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()

	// Create a target file outside the base
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside base pointing outside
	link := filepath.Join(base, "escape-link")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Fatal(err)
	}

	_, err := New(link, []string{base})
	if err == nil {
		t.Errorf("expected error for symlink escaping base, got nil")
	}
}

// Task 1.2: accept valid paths

func TestNew_AcceptsAbsoluteWithinBase(t *testing.T) {
	base := t.TempDir()

	// Create a real file within base
	subdir := filepath.Join(base, "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(subdir, "file.txt")
	if err := os.WriteFile(file, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}

	sp, err := New(file, []string{base})
	if err != nil {
		t.Fatalf("expected valid path to succeed, got: %v", err)
	}
	if sp.String() == "" {
		t.Error("expected non-empty resolved path")
	}
}

func TestNew_ResolvesSymlinksWithinBase(t *testing.T) {
	base := t.TempDir()

	// Create a real file
	realFile := filepath.Join(base, "real.txt")
	if err := os.WriteFile(realFile, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create a symlink within the same base
	link := filepath.Join(base, "link.txt")
	if err := os.Symlink(realFile, link); err != nil {
		t.Fatal(err)
	}

	sp, err := New(link, []string{base})
	if err != nil {
		t.Fatalf("expected symlink within base to succeed, got: %v", err)
	}

	// Resolved path should point to the real file
	resolved, _ := filepath.EvalSymlinks(realFile)
	if sp.String() != resolved {
		t.Errorf("expected resolved=%q, got %q", resolved, sp.String())
	}
}

func TestNew_AcceptsMultipleBases(t *testing.T) {
	base1 := t.TempDir()
	base2 := t.TempDir()

	file := filepath.Join(base2, "file.txt")
	if err := os.WriteFile(file, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := New(file, []string{base1, base2})
	if err != nil {
		t.Fatalf("expected path in second base to succeed, got: %v", err)
	}
}

func TestNew_RejectsEmptyBases(t *testing.T) {
	_, err := New("/some/path", nil)
	if err == nil {
		t.Error("expected error for empty allowed bases")
	}
}

func TestNew_RejectsNonExistentPath(t *testing.T) {
	base := t.TempDir()
	_, err := New(filepath.Join(base, "nonexistent"), []string{base})
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

// Task 1.3: String() returns resolved path

func TestSafePath_String(t *testing.T) {
	base := t.TempDir()

	file := filepath.Join(base, "test.txt")
	if err := os.WriteFile(file, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}

	sp, err := New(file, []string{base})
	if err != nil {
		t.Fatal(err)
	}

	expected, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}

	if sp.String() != expected {
		t.Errorf("String()=%q, want %q", sp.String(), expected)
	}
}

func TestNewOutput_NonExistentFile(t *testing.T) {
	base := t.TempDir()

	// File doesn't exist yet, but parent dir does
	sp, err := NewOutput(filepath.Join(base, "new_file.txt"), []string{base})
	if err != nil {
		t.Fatalf("expected success for non-existent output file, got: %v", err)
	}
	if sp.String() == "" {
		t.Error("expected non-empty resolved path")
	}
}

func TestNewOutput_RejectsTraversal(t *testing.T) {
	base := t.TempDir()

	_, err := NewOutput(filepath.Join(base, "..", "escape.txt"), []string{base})
	if err == nil {
		t.Error("expected error for traversal in output path")
	}
}

func TestNewOutput_RejectsNonExistentParent(t *testing.T) {
	base := t.TempDir()

	_, err := NewOutput(filepath.Join(base, "nodir", "file.txt"), []string{base})
	if err == nil {
		t.Error("expected error when parent directory doesn't exist")
	}
}
