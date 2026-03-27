package safepath

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafePath is an opaque type representing a validated filesystem path.
// It guarantees the path:
//   - Is absolute
//   - Has been cleaned (no .., double slashes)
//   - Resolves within one of the allowed base directories
//   - Has had symlinks evaluated (via filepath.EvalSymlinks)
//
// SafePath values can only be constructed via New(), which performs all validation.
// Tools accept SafePath, never raw strings, for filesystem arguments.
type SafePath struct {
	resolved string
}

// New validates and constructs a SafePath. Returns error if the path
// escapes the allowed bases or contains traversal sequences.
func New(raw string, allowedBases []string) (SafePath, error) {
	if len(allowedBases) == 0 {
		return SafePath{}, fmt.Errorf("safepath: no allowed bases provided")
	}

	if !filepath.IsAbs(raw) {
		return SafePath{}, fmt.Errorf("safepath: path must be absolute, got %q", raw)
	}

	// Clean the path first and check for traversal before resolving symlinks.
	// This catches explicit ".." sequences even if the path doesn't exist yet.
	cleaned := filepath.Clean(raw)
	for _, base := range allowedBases {
		cleanBase := filepath.Clean(base)
		if !strings.HasPrefix(cleaned, cleanBase+string(filepath.Separator)) && cleaned != cleanBase {
			continue
		}
		// Cleaned path is within this base — now resolve symlinks to verify the real path.
		resolved, err := filepath.EvalSymlinks(cleaned)
		if err != nil {
			return SafePath{}, fmt.Errorf("safepath: resolving %q: %w", raw, err)
		}

		// Resolve the base too (it may itself be a symlink)
		resolvedBase, err := filepath.EvalSymlinks(cleanBase)
		if err != nil {
			return SafePath{}, fmt.Errorf("safepath: resolving base %q: %w", base, err)
		}

		if !strings.HasPrefix(resolved, resolvedBase+string(filepath.Separator)) && resolved != resolvedBase {
			return SafePath{}, fmt.Errorf("safepath: resolved path %q escapes base %q", resolved, resolvedBase)
		}

		return SafePath{resolved: resolved}, nil
	}

	return SafePath{}, fmt.Errorf("safepath: path %q is not within any allowed base", raw)
}

// NewOutput validates a path for a file that does not yet exist (e.g., ffmpeg output).
// It validates the parent directory exists and is within allowed bases,
// then constructs the full path using the validated parent + filename.
func NewOutput(raw string, allowedBases []string) (SafePath, error) {
	if len(allowedBases) == 0 {
		return SafePath{}, fmt.Errorf("safepath: no allowed bases provided")
	}

	if !filepath.IsAbs(raw) {
		return SafePath{}, fmt.Errorf("safepath: path must be absolute, got %q", raw)
	}

	cleaned := filepath.Clean(raw)
	dir := filepath.Dir(cleaned)
	base := filepath.Base(cleaned)

	// Validate the parent directory exists and is within allowed bases
	parentSafe, err := New(dir, allowedBases)
	if err != nil {
		return SafePath{}, fmt.Errorf("safepath: validating parent of output path: %w", err)
	}

	return SafePath{resolved: filepath.Join(parentSafe.resolved, base)}, nil
}

// String returns the resolved path.
func (p SafePath) String() string {
	return p.resolved
}
