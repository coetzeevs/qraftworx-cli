// Package tools defines the Tool interface, the compile-time Registry, and all
// tool implementations for QraftWorx.
//
// # Key Types
//
//   - Tool: the interface every Qraft tool must implement (Name, Description,
//     Parameters, Execute, RequiresConfirmation, Permissions).
//   - ToolPermission: declares what capabilities a tool needs (Network, FileSystem,
//     Hardware, MediaCapture, Upload).
//   - Registry: holds all registered tools keyed by name. Panics on duplicate
//     registration to catch errors at startup.
//
// # Tool Implementations
//
//   - MemoryAddTool: adds a memory node to Cerebro (episode, concept, procedure,
//     or reflection). Does not require confirmation.
//   - MemorySearchTool: searches Cerebro for relevant memories by query. Does not
//     require confirmation.
//   - CaptureMediaTool: captures a frame from a V4L2 device using ffmpeg. Requires
//     confirmation (hardware + media capture).
//   - ProcessVideoTool: transcodes video files using ffmpeg with validated codec,
//     resolution, FPS, and duration. Does not require confirmation.
//   - UploadTool: uploads media to external platforms (YouTube, TikTok). Requires
//     confirmation (upload = data exfiltration). Rate limited to 1 per hour.
//
// # Platform Uploaders
//
//   - YouTubeUploader: multipart upload via YouTube API with OAuth.
//   - TikTokUploader: Content Posting API upload with OAuth.
//   - Both implement the Uploader interface.
//
// # Supporting Types
//
//   - FFmpegBuilder: constructs validated ffmpeg commands. Binary validated at
//     startup. All paths must be SafePath. Codecs validated against allowlist.
//     Numeric parameters (FPS, duration) are clamped to safe ranges.
//   - ValidateMediaFile: checks MIME type by reading file header bytes, not
//     extension. Rejects files that are not video/mp4 or image/jpeg.
//
// # Architecture Role
//
// Tools are the action layer of QraftWorx. Gemini decides which tools to call,
// the Executor enforces permissions and confirmation gates, and the tools
// themselves perform the actual work. All tools are registered at compile time --
// no dynamic loading or directory scanning.
//
// # Security Considerations
//
//   - S2 (Command Injection): FFmpegBuilder uses exec.CommandContext with separate
//     argument slices, never sh -c. Device paths come from config, not LLM args.
//     Codecs are validated against an allowlist.
//   - S7 (Upload Hardening): Upload paths are validated via SafePath (symlink-resolved,
//     within media directory). MIME types are validated by file header. Rate limited
//     to 1 upload per hour.
//   - S9 (Filesystem Boundaries): All file paths use SafePath. CaptureMediaTool
//     validates output filenames have no path separators.
//
// # Testing
//
// Tool tests use testCerebroClient() with noop embedder, SafePath with t.TempDir()
// bases, and httptest.Server for upload platform mocks.
package tools
