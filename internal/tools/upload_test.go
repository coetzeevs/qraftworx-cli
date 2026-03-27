package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// mockUploader records calls for testing.
type mockUploader struct {
	platform string
	called   bool
	file     safepath.SafePath
	metadata UploadMetadata
	err      error
	delay    time.Duration
}

func (m *mockUploader) Platform() string { return m.platform }

func (m *mockUploader) Upload(ctx context.Context, file safepath.SafePath, metadata UploadMetadata) (*UploadResult, error) {
	m.called = true
	m.file = file
	m.metadata = metadata

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.err != nil {
		return nil, m.err
	}
	return &UploadResult{
		Platform: m.platform,
		URL:      fmt.Sprintf("https://%s.com/video/test123", m.platform),
		ID:       "test123",
	}, nil
}

// createMediaDir creates a temp dir with a valid MP4 file for upload tests.
// Returns the SafePath for the directory and the resolved absolute path to the MP4 file.
func createMediaDir(t *testing.T) (mediaDir safepath.SafePath, mp4Path string) {
	t.Helper()
	dir := t.TempDir()

	// Resolve symlinks on the temp dir itself (macOS /var -> /private/var)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}

	mp4Path = filepath.Join(resolvedDir, "video.mp4")
	if err := os.WriteFile(mp4Path, testMP4Header, 0o600); err != nil {
		t.Fatalf("write test mp4: %v", err)
	}
	sp, err := safepath.New(resolvedDir, []string{resolvedDir})
	if err != nil {
		t.Fatalf("safepath for media dir: %v", err)
	}
	return sp, mp4Path
}

// Task 7.5: UploadTool rejects path outside mediaDir.
func TestUploadTool_RejectsPathOutsideMediaDir(t *testing.T) {
	mediaDir, _ := createMediaDir(t)
	mock := &mockUploader{platform: "youtube"}

	tool := NewUploadTool(mediaDir, map[string]Uploader{"youtube": mock})

	// Create a file outside the media directory
	outsideDir := t.TempDir()
	resolvedOutside, err := filepath.EvalSymlinks(outsideDir)
	if err != nil {
		t.Fatalf("resolve outside dir: %v", err)
	}
	outsidePath := filepath.Join(resolvedOutside, "outside.mp4")
	if err := os.WriteFile(outsidePath, testMP4Header, 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	args := json.RawMessage(fmt.Sprintf(`{"file": %q, "platform": "youtube", "title": "test"}`, outsidePath))
	_, err = tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for path outside mediaDir, got nil")
	}
}

// Task 7.6: UploadTool rejects symlink escape.
func TestUploadTool_RejectsSymlinkEscape(t *testing.T) {
	mediaDir, _ := createMediaDir(t)
	mock := &mockUploader{platform: "youtube"}

	tool := NewUploadTool(mediaDir, map[string]Uploader{"youtube": mock})

	// Create a file outside the media directory
	outsideDir := t.TempDir()
	resolvedOutside, err := filepath.EvalSymlinks(outsideDir)
	if err != nil {
		t.Fatalf("resolve outside dir: %v", err)
	}
	outsidePath := filepath.Join(resolvedOutside, "secret.mp4")
	if err := os.WriteFile(outsidePath, testMP4Header, 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	// Create a symlink inside mediaDir pointing outside
	symlinkPath := filepath.Join(mediaDir.String(), "escape.mp4")
	if err := os.Symlink(outsidePath, symlinkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	args := json.RawMessage(fmt.Sprintf(`{"file": %q, "platform": "youtube", "title": "test"}`, symlinkPath))
	_, err = tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for symlink escape, got nil")
	}
}

// Task 7.7: UploadTool rate limit per interaction.
func TestUploadTool_RateLimitPerInteraction(t *testing.T) {
	mediaDir, mp4Path := createMediaDir(t)
	mock := &mockUploader{platform: "youtube"}

	tool := NewUploadTool(mediaDir, map[string]Uploader{"youtube": mock})

	args := json.RawMessage(fmt.Sprintf(`{"file": %q, "platform": "youtube", "title": "test"}`, mp4Path))

	// First upload should succeed
	_, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("first upload should succeed: %v", err)
	}

	// Second upload should fail with rate limit error
	_, err = tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected rate limit error on second upload, got nil")
	}
}

// Task 7.8: UploadTool requires confirmation.
func TestUploadTool_RequiresConfirmation(t *testing.T) {
	mediaDir, _ := createMediaDir(t)
	tool := NewUploadTool(mediaDir, nil)

	if !tool.RequiresConfirmation() {
		t.Error("UploadTool.RequiresConfirmation() = false, want true")
	}
}

// Task 7.9: UploadTool execute with mock uploader.
func TestUploadTool_Execute_MockUploader(t *testing.T) {
	mediaDir, mp4Path := createMediaDir(t)
	mock := &mockUploader{platform: "youtube"}

	tool := NewUploadTool(mediaDir, map[string]Uploader{"youtube": mock})

	args := json.RawMessage(fmt.Sprintf(`{
		"file": %q,
		"platform": "youtube",
		"title": "My Video",
		"description": "A test video",
		"tags": ["test", "upload"],
		"privacy": "unlisted"
	}`, mp4Path))

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !mock.called {
		t.Fatal("mock uploader was not called")
	}

	if mock.metadata.Title != "My Video" {
		t.Errorf("title=%q, want 'My Video'", mock.metadata.Title)
	}
	if mock.metadata.Description != "A test video" {
		t.Errorf("description=%q, want 'A test video'", mock.metadata.Description)
	}
	if mock.metadata.Privacy != "unlisted" {
		t.Errorf("privacy=%q, want 'unlisted'", mock.metadata.Privacy)
	}
	if len(mock.metadata.Tags) != 2 {
		t.Errorf("tags=%v, want 2 tags", mock.metadata.Tags)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["status"] != "uploaded" {
		t.Errorf("status=%v, want uploaded", m["status"])
	}
	if m["platform"] != "youtube" {
		t.Errorf("platform=%v, want youtube", m["platform"])
	}
}

// Task 7.10: UploadTool returns error for unknown platform.
func TestUploadTool_Execute_UnknownPlatform(t *testing.T) {
	mediaDir, mp4Path := createMediaDir(t)
	tool := NewUploadTool(mediaDir, map[string]Uploader{})

	args := json.RawMessage(fmt.Sprintf(`{"file": %q, "platform": "twitch", "title": "test"}`, mp4Path))
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for unknown platform, got nil")
	}
}

// Task 7.11: UploadTool timeout on upload (S10).
func TestUploadTool_Execute_Timeout(t *testing.T) {
	mediaDir, mp4Path := createMediaDir(t)
	mock := &mockUploader{
		platform: "youtube",
		delay:    5 * time.Second, // longer than context timeout
	}

	tool := NewUploadTool(mediaDir, map[string]Uploader{"youtube": mock})

	// Create a context that times out quickly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	args := json.RawMessage(fmt.Sprintf(`{"file": %q, "platform": "youtube", "title": "test"}`, mp4Path))
	_, err := tool.Execute(ctx, args)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}
