package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// Uploader is the interface for platform-specific upload logic.
type Uploader interface {
	Upload(ctx context.Context, file safepath.SafePath, metadata UploadMetadata) (*UploadResult, error)
	Platform() string
}

// UploadMetadata configures the upload.
type UploadMetadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Privacy     string   `json:"privacy"` // "private", "unlisted", "public"
}

// UploadResult is returned after a successful upload.
type UploadResult struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
	ID       string `json:"id"`
}

// UploadTool uploads media to external platforms.
type UploadTool struct {
	mediaDir    safepath.SafePath   // S7: only files within this dir
	maxPerHour  int                 // S7: rate limit (default: 1)
	platforms   map[string]Uploader // platform name -> uploader
	mu          sync.Mutex          // guards uploadCount
	uploads     int                 // uploads in current interaction window
	windowStart time.Time           // start of rate limit window
}

// NewUploadTool creates an UploadTool.
func NewUploadTool(mediaDir safepath.SafePath, platforms map[string]Uploader) *UploadTool {
	return &UploadTool{
		mediaDir:   mediaDir,
		maxPerHour: 1,
		platforms:  platforms,
	}
}

func (t *UploadTool) Name() string        { return "upload_media" }
func (t *UploadTool) Description() string { return "Upload media to an external platform" }
func (t *UploadTool) Parameters() map[string]any {
	return map[string]any{
		"file": map[string]any{
			"type":        "STRING",
			"description": "path to media file (must be within media directory)",
			"required":    true,
		},
		"platform": map[string]any{
			"type":        "STRING",
			"description": "target platform (e.g., youtube, tiktok)",
			"required":    true,
		},
		"title": map[string]any{
			"type":        "STRING",
			"description": "title for the upload",
			"required":    true,
		},
		"description": map[string]any{
			"type":        "STRING",
			"description": "description for the upload",
		},
		"tags": map[string]any{
			"type":        "ARRAY",
			"description": "tags for the upload",
		},
		"privacy": map[string]any{
			"type":        "STRING",
			"description": "privacy setting: private, unlisted, or public",
		},
	}
}

// RequiresConfirmation returns true (upload = data exfiltration).
func (t *UploadTool) RequiresConfirmation() bool { return true }

// Permissions declares upload and network capabilities.
func (t *UploadTool) Permissions() ToolPermission {
	return ToolPermission{Network: true, Upload: true, FileSystem: true}
}

type uploadArgs struct {
	File        string   `json:"file"`
	Platform    string   `json:"platform"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Privacy     string   `json:"privacy"`
}

// Execute uploads a media file to the specified platform.
func (t *UploadTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	var a uploadArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("upload_media: invalid args: %w", err)
	}
	if a.File == "" {
		return nil, fmt.Errorf("upload_media: file is required")
	}
	if a.Platform == "" {
		return nil, fmt.Errorf("upload_media: platform is required")
	}
	if a.Title == "" {
		return nil, fmt.Errorf("upload_media: title is required")
	}

	// S7: Rate limit — max 1 upload per interaction (hour window)
	if err := t.checkRateLimit(); err != nil {
		return nil, err
	}

	// S7: Validate path is within mediaDir after symlink resolution
	filePath, err := t.validateFilePath(a.File)
	if err != nil {
		return nil, err
	}

	// S7: Validate MIME type by reading file header
	mimeType, err := ValidateMediaFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("upload_media: %w", err)
	}

	// Look up platform uploader
	uploader, ok := t.platforms[strings.ToLower(a.Platform)]
	if !ok {
		return nil, fmt.Errorf("upload_media: unknown platform %q", a.Platform)
	}

	metadata := UploadMetadata{
		Title:       a.Title,
		Description: a.Description,
		Tags:        a.Tags,
		Privacy:     a.Privacy,
	}

	// Execute the upload with context (S10: timeout support)
	result, err := uploader.Upload(ctx, filePath, metadata)
	if err != nil {
		return nil, fmt.Errorf("upload_media: upload failed: %w", err)
	}

	// Record the upload for rate limiting
	t.recordUpload()

	return map[string]any{
		"status":   "uploaded",
		"platform": result.Platform,
		"url":      result.URL,
		"id":       result.ID,
		"mime":     mimeType,
	}, nil
}

// validateFilePath validates the file path is within mediaDir using SafePath.
func (t *UploadTool) validateFilePath(raw string) (safepath.SafePath, error) {
	// Ensure absolute path
	if !filepath.IsAbs(raw) {
		return safepath.SafePath{}, fmt.Errorf("upload_media: file path must be absolute")
	}

	// Use SafePath.New which resolves symlinks and validates containment
	sp, err := safepath.New(raw, []string{t.mediaDir.String()})
	if err != nil {
		return safepath.SafePath{}, fmt.Errorf("upload_media: path validation: %w", err)
	}
	return sp, nil
}

// checkRateLimit enforces the per-interaction upload limit (S7).
func (t *UploadTool) checkRateLimit() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	// Reset window if more than an hour has passed
	if t.windowStart.IsZero() || now.Sub(t.windowStart) > time.Hour {
		t.uploads = 0
		t.windowStart = now
	}

	if t.uploads >= t.maxPerHour {
		return fmt.Errorf("upload_media: rate limit exceeded (max %d per hour)", t.maxPerHour)
	}

	return nil
}

// recordUpload records a successful upload for rate limiting.
func (t *UploadTool) recordUpload() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.windowStart.IsZero() {
		t.windowStart = time.Now()
	}
	t.uploads++
}
