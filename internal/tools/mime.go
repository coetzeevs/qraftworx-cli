package tools

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// allowedMIMETypes is the allowlist of MIME types accepted for upload (S7).
var allowedMIMETypes = map[string]bool{
	"video/mp4":  true,
	"image/jpeg": true,
}

// ValidateMediaFile checks that a file is a valid media file for upload (S7).
// Reads file header bytes, validates MIME type against allowlist.
// Rejects based on actual content, not file extension.
func ValidateMediaFile(path safepath.SafePath) (string, error) {
	f, err := os.Open(path.String())
	if err != nil {
		return "", fmt.Errorf("validate media: open: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Read the first 512 bytes for MIME detection (http.DetectContentType uses up to 512)
	buf := make([]byte, 512)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", fmt.Errorf("validate media: read header: %w", err)
	}
	buf = buf[:n]

	mimeType := http.DetectContentType(buf)

	if !allowedMIMETypes[mimeType] {
		return "", fmt.Errorf("validate media: MIME type %q not allowed", mimeType)
	}

	return mimeType, nil
}
