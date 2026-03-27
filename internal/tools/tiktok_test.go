package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// Task 7.13: TikTokUploader request format verified via httptest.Server.
func TestTikTokUploader_RequestFormat(t *testing.T) {
	var (
		gotAuth        string
		gotContentType string
		gotMetaHeader  string
		gotBody        []byte
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotMetaHeader = r.Header.Get("X-TikTok-Metadata")

		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"video_id": "tt_video_456"}}`))
	}))
	defer server.Close()

	// Create test media file
	dir := t.TempDir()
	mp4Path := filepath.Join(dir, "test.mp4")
	if err := os.WriteFile(mp4Path, testMP4Header, 0o600); err != nil {
		t.Fatal(err)
	}
	sp, err := safepath.New(mp4Path, []string{dir})
	if err != nil {
		t.Fatal(err)
	}

	uploader := NewTikTokUploader(server.URL, "tiktok-access-token", server.Client())

	metadata := UploadMetadata{
		Title:       "TikTok Video",
		Description: "A test TikTok upload",
		Tags:        []string{"viral", "test"},
		Privacy:     "public",
	}

	result, err := uploader.Upload(context.Background(), sp, metadata)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// Verify access token
	if gotAuth != "Bearer tiktok-access-token" {
		t.Errorf("auth=%q, want 'Bearer tiktok-access-token'", gotAuth)
	}

	// Verify content type is octet-stream (file body)
	if gotContentType != "application/octet-stream" {
		t.Errorf("content-type=%q, want 'application/octet-stream'", gotContentType)
	}

	// Verify metadata header
	if gotMetaHeader == "" {
		t.Fatal("X-TikTok-Metadata header is empty")
	}
	var metaFromHeader tiktokUploadRequest
	if err := json.Unmarshal([]byte(gotMetaHeader), &metaFromHeader); err != nil {
		t.Fatalf("unmarshal metadata header: %v", err)
	}
	if metaFromHeader.Title != "TikTok Video" {
		t.Errorf("title=%q, want 'TikTok Video'", metaFromHeader.Title)
	}
	if metaFromHeader.Description != "A test TikTok upload" {
		t.Errorf("description=%q, want 'A test TikTok upload'", metaFromHeader.Description)
	}
	if metaFromHeader.Privacy != "public" {
		t.Errorf("privacy=%q, want 'public'", metaFromHeader.Privacy)
	}
	if len(metaFromHeader.Tags) != 2 {
		t.Errorf("tags=%v, want 2 tags", metaFromHeader.Tags)
	}

	// Verify body contains the file content
	if len(gotBody) != len(testMP4Header) {
		t.Errorf("body length=%d, want %d", len(gotBody), len(testMP4Header))
	}

	// Verify result
	if result.Platform != "tiktok" {
		t.Errorf("platform=%q, want tiktok", result.Platform)
	}
	if result.ID != "tt_video_456" {
		t.Errorf("id=%q, want tt_video_456", result.ID)
	}
	if !strings.Contains(result.URL, "tt_video_456") {
		t.Errorf("url=%q, want to contain video ID", result.URL)
	}
}
