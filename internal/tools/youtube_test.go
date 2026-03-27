package tools

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// Task 7.12: YouTubeUploader request format verified via httptest.Server.
func TestYouTubeUploader_RequestFormat(t *testing.T) {
	var (
		gotAuth        string
		gotContentType string
		gotMetadata    youtubeSnippet
		gotFileContent []byte
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")

		// Parse multipart
		mediaType, params, err := mime.ParseMediaType(gotContentType)
		if err != nil {
			t.Errorf("parse content type: %v", err)
			http.Error(w, "bad content type", http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(mediaType, "multipart/") {
			t.Errorf("content-type=%q, want multipart/*", mediaType)
			http.Error(w, "not multipart", http.StatusBadRequest)
			return
		}

		reader := multipart.NewReader(r.Body, params["boundary"])

		// Part 1: metadata (JSON)
		part1, err := reader.NextPart()
		if err != nil {
			t.Errorf("read metadata part: %v", err)
			http.Error(w, "no metadata part", http.StatusBadRequest)
			return
		}
		metaBytes, err := io.ReadAll(part1)
		if err != nil {
			t.Errorf("read metadata: %v", err)
			http.Error(w, "bad metadata", http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(metaBytes, &gotMetadata); err != nil {
			t.Errorf("unmarshal metadata: %v", err)
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		// Part 2: file
		part2, err := reader.NextPart()
		if err != nil {
			t.Errorf("read file part: %v", err)
			http.Error(w, "no file part", http.StatusBadRequest)
			return
		}
		gotFileContent, err = io.ReadAll(part2)
		if err != nil {
			t.Errorf("read file content: %v", err)
			http.Error(w, "bad file", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "yt_video_123"}`))
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

	uploader := NewYouTubeUploader(server.URL, "test-oauth-token", server.Client())

	metadata := UploadMetadata{
		Title:       "Test Video",
		Description: "A test upload",
		Tags:        []string{"golang", "test"},
		Privacy:     "unlisted",
	}

	result, err := uploader.Upload(context.Background(), sp, metadata)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// Verify OAuth token
	if gotAuth != "Bearer test-oauth-token" {
		t.Errorf("auth=%q, want 'Bearer test-oauth-token'", gotAuth)
	}

	// Verify multipart content type
	if !strings.Contains(gotContentType, "multipart/form-data") {
		t.Errorf("content-type=%q, want multipart/form-data", gotContentType)
	}

	// Verify metadata
	if gotMetadata.Snippet.Title != "Test Video" {
		t.Errorf("title=%q, want 'Test Video'", gotMetadata.Snippet.Title)
	}
	if gotMetadata.Snippet.Description != "A test upload" {
		t.Errorf("description=%q, want 'A test upload'", gotMetadata.Snippet.Description)
	}
	if len(gotMetadata.Snippet.Tags) != 2 {
		t.Errorf("tags=%v, want 2 tags", gotMetadata.Snippet.Tags)
	}
	if gotMetadata.Status.PrivacyStatus != "unlisted" {
		t.Errorf("privacy=%q, want 'unlisted'", gotMetadata.Status.PrivacyStatus)
	}

	// Verify file content matches
	if len(gotFileContent) != len(testMP4Header) {
		t.Errorf("file content length=%d, want %d", len(gotFileContent), len(testMP4Header))
	}

	// Verify result
	if result.Platform != "youtube" {
		t.Errorf("platform=%q, want youtube", result.Platform)
	}
	if result.ID != "yt_video_123" {
		t.Errorf("id=%q, want yt_video_123", result.ID)
	}
	if !strings.Contains(result.URL, "yt_video_123") {
		t.Errorf("url=%q, want to contain video ID", result.URL)
	}
}
