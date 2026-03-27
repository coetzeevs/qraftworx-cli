package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// YouTubeUploader uploads media to YouTube via the resumable upload API.
type YouTubeUploader struct {
	apiURL     string // YouTube upload endpoint (overridable for testing)
	oauthToken string // OAuth2 bearer token
	client     *http.Client
}

// NewYouTubeUploader creates a YouTubeUploader.
func NewYouTubeUploader(apiURL, oauthToken string, client *http.Client) *YouTubeUploader {
	if client == nil {
		client = http.DefaultClient
	}
	return &YouTubeUploader{
		apiURL:     apiURL,
		oauthToken: oauthToken,
		client:     client,
	}
}

// Platform returns "youtube".
func (u *YouTubeUploader) Platform() string { return "youtube" }

// youtubeSnippet is the metadata sent in the multipart request.
type youtubeSnippet struct {
	Snippet youtubeSnippetInner `json:"snippet"`
	Status  youtubeStatus       `json:"status"`
}

type youtubeSnippetInner struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type youtubeStatus struct {
	PrivacyStatus string `json:"privacyStatus"`
}

// Upload sends a media file to YouTube using multipart upload with OAuth.
func (u *YouTubeUploader) Upload(ctx context.Context, file safepath.SafePath, metadata UploadMetadata) (*UploadResult, error) {
	// Build multipart body
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Part 1: JSON metadata
	metadataHeader := make(map[string][]string)
	metadataHeader["Content-Type"] = []string{"application/json"}
	metaPart, err := writer.CreatePart(metadataHeader)
	if err != nil {
		return nil, fmt.Errorf("youtube: create metadata part: %w", err)
	}

	privacy := metadata.Privacy
	if privacy == "" {
		privacy = "private"
	}

	snippet := youtubeSnippet{
		Snippet: youtubeSnippetInner{
			Title:       metadata.Title,
			Description: metadata.Description,
			Tags:        metadata.Tags,
		},
		Status: youtubeStatus{
			PrivacyStatus: privacy,
		},
	}
	if err := json.NewEncoder(metaPart).Encode(snippet); err != nil {
		return nil, fmt.Errorf("youtube: encode metadata: %w", err)
	}

	// Part 2: File content
	filePart, err := writer.CreateFormFile("file", file.String())
	if err != nil {
		return nil, fmt.Errorf("youtube: create file part: %w", err)
	}

	f, err := os.Open(file.String())
	if err != nil {
		return nil, fmt.Errorf("youtube: open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(filePart, f); err != nil {
		return nil, fmt.Errorf("youtube: copy file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("youtube: close multipart: %w", err)
	}

	// Build request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.apiURL, &body)
	if err != nil {
		return nil, fmt.Errorf("youtube: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+u.oauthToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("youtube: upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("youtube: decode response: %w", err)
	}

	return &UploadResult{
		Platform: "youtube",
		URL:      fmt.Sprintf("https://youtube.com/watch?v=%s", result.ID),
		ID:       result.ID,
	}, nil
}
