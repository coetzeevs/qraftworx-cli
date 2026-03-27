package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// TikTokUploader uploads media to TikTok via the Content Posting API.
type TikTokUploader struct {
	apiURL      string // TikTok API endpoint (overridable for testing)
	accessToken string // OAuth2 access token
	client      *http.Client
}

// NewTikTokUploader creates a TikTokUploader.
func NewTikTokUploader(apiURL, accessToken string, client *http.Client) *TikTokUploader {
	if client == nil {
		client = http.DefaultClient
	}
	return &TikTokUploader{
		apiURL:      apiURL,
		accessToken: accessToken,
		client:      client,
	}
}

// Platform returns "tiktok".
func (u *TikTokUploader) Platform() string { return "tiktok" }

// tiktokUploadRequest is the JSON body sent to the TikTok API.
type tiktokUploadRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Privacy     string   `json:"privacy_level"`
	VideoData   string   `json:"video_data"` // base64 encoded in real API; raw bytes via file upload here
}

// Upload sends a media file to TikTok using the Content Posting API format.
func (u *TikTokUploader) Upload(ctx context.Context, file safepath.SafePath, metadata UploadMetadata) (*UploadResult, error) {
	// Read the file content
	fileData, err := os.ReadFile(file.String())
	if err != nil {
		return nil, fmt.Errorf("tiktok: read file: %w", err)
	}

	privacy := metadata.Privacy
	if privacy == "" {
		privacy = "private"
	}

	// Build the JSON request with file data
	reqBody := tiktokUploadRequest{
		Title:       metadata.Title,
		Description: metadata.Description,
		Tags:        metadata.Tags,
		Privacy:     privacy,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("tiktok: marshal request: %w", err)
	}

	// For the TikTok API, we send metadata as JSON and file as the body
	// The actual API uses a two-step process; we simulate the single-step version
	// by sending metadata in a header and file as body
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.apiURL, bytes.NewReader(fileData))
	if err != nil {
		return nil, fmt.Errorf("tiktok: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+u.accessToken)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-TikTok-Metadata", string(jsonBody))

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tiktok: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tiktok: upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			VideoID string `json:"video_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("tiktok: decode response: %w", err)
	}

	return &UploadResult{
		Platform: "tiktok",
		URL:      fmt.Sprintf("https://www.tiktok.com/@user/video/%s", result.Data.VideoID),
		ID:       result.Data.VideoID,
	}, nil
}
