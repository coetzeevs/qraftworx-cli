// Package safepath provides the SafePath opaque type for filesystem boundary
// enforcement across QraftWorx.
//
// # Key Types
//
//   - SafePath: an opaque type representing a validated filesystem path. Can only
//     be constructed via New() or NewOutput(), which perform all validation.
//   - New: validates an existing path against allowed base directories. Rejects
//     relative paths, traversal sequences, and symlinks that escape boundaries.
//   - NewOutput: validates a path for a file that does not yet exist. Validates
//     the parent directory and constructs the full path.
//   - String: returns the resolved, absolute path.
//
// # Validation Guarantees
//
// A SafePath value guarantees that the path:
//   - Is absolute (not relative)
//   - Has been cleaned (filepath.Clean: no .., double slashes)
//   - Resolves within one of the allowed base directories
//   - Has had symlinks evaluated (filepath.EvalSymlinks) for both the path
//     and the base directory
//
// # Architecture Role
//
// SafePath is used throughout the codebase wherever filesystem paths are handled.
// Tools accept SafePath instead of raw strings, preventing path traversal and
// symlink escape attacks at the type level. This is a compile-time enforcement
// mechanism: code that needs a filesystem path must construct a SafePath first,
// and construction fails if the path violates boundaries.
//
// Key consumers:
//   - FFmpegBuilder: device paths and output paths are SafePath
//   - CaptureMediaTool: work directory and device path are SafePath
//   - ProcessVideoTool: input and output paths validated via SafePath
//   - UploadTool: media file paths validated via SafePath
//
// # Security Considerations
//
//   - S9 (Filesystem Boundaries): SafePath transforms the filesystem boundary
//     policy from a runtime check into a type-level constraint. Functions that
//     accept SafePath cannot receive unvalidated paths.
//   - S2 (Command Injection): FFmpegBuilder uses SafePath for all file arguments,
//     preventing path traversal in ffmpeg commands.
//   - Symlink-aware: Both the path and its base directory are resolved through
//     filepath.EvalSymlinks, preventing symlink escape attacks where a symlink
//     inside an allowed directory points to a file outside it.
//
// # Testing
//
// Tests create real files, directories, and symlinks in t.TempDir(). Traversal
// attempts with "..", escaping symlinks, relative paths, and non-existent paths
// are all verified to fail. Multiple allowed bases are tested. NewOutput is
// tested for non-existent files with existing parent directories.
package safepath
