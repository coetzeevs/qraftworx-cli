package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/coetzeevs/qraftworx-cli/internal/safepath"
)

// Minimal valid MP4 file header (ftyp box).
// This is a minimal ftyp atom: size (8 bytes) + "ftyp" + brand "isom" + version + compatible brands.
var testMP4Header = []byte{
	0x00, 0x00, 0x00, 0x1C, // box size = 28 bytes
	0x66, 0x74, 0x79, 0x70, // "ftyp"
	0x69, 0x73, 0x6F, 0x6D, // major brand "isom"
	0x00, 0x00, 0x02, 0x00, // minor version
	0x69, 0x73, 0x6F, 0x6D, // compatible brand "isom"
	0x69, 0x73, 0x6F, 0x32, // compatible brand "iso2"
	0x6D, 0x70, 0x34, 0x31, // compatible brand "mp41"
}

// Minimal valid JPEG file header (SOI marker + JFIF APP0).
var testJPEGHeader = []byte{
	0xFF, 0xD8, 0xFF, 0xE0, // SOI + APP0 marker
	0x00, 0x10, // APP0 length
	0x4A, 0x46, 0x49, 0x46, 0x00, // "JFIF\0"
	0x01, 0x01, // version
	0x00,       // aspect ratio units
	0x00, 0x01, // X density
	0x00, 0x01, // Y density
	0x00, 0x00, // thumbnail dimensions
}

func writeTestFile(t *testing.T, dir, name string, data []byte) safepath.SafePath {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write test file %s: %v", name, err)
	}
	sp, err := safepath.New(path, []string{dir})
	if err != nil {
		t.Fatalf("safepath for %s: %v", name, err)
	}
	return sp
}

// Task 7.1: ValidateMediaFile accepts video/mp4.
func TestValidateMediaFile_AcceptsMP4(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFile(t, dir, "test.mp4", testMP4Header)

	mimeType, err := ValidateMediaFile(sp)
	if err != nil {
		t.Fatalf("expected valid MP4, got error: %v", err)
	}
	if mimeType != "video/mp4" {
		t.Errorf("mime=%q, want video/mp4", mimeType)
	}
}

// Task 7.2: ValidateMediaFile accepts image/jpeg.
func TestValidateMediaFile_AcceptsJPEG(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFile(t, dir, "test.jpg", testJPEGHeader)

	mimeType, err := ValidateMediaFile(sp)
	if err != nil {
		t.Fatalf("expected valid JPEG, got error: %v", err)
	}
	if mimeType != "image/jpeg" {
		t.Errorf("mime=%q, want image/jpeg", mimeType)
	}
}

// Task 7.3: ValidateMediaFile rejects text/plain.
func TestValidateMediaFile_RejectsText(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFile(t, dir, "readme.txt", []byte("This is plain text content"))

	_, err := ValidateMediaFile(sp)
	if err == nil {
		t.Fatal("expected error for text file, got nil")
	}
}

// Task 7.4: ValidateMediaFile rejects file with .mp4 extension but text content.
func TestValidateMediaFile_RejectsRenamedText(t *testing.T) {
	dir := t.TempDir()
	// Text content disguised with .mp4 extension
	sp := writeTestFile(t, dir, "fake.mp4", []byte("This is not an MP4 file at all"))

	_, err := ValidateMediaFile(sp)
	if err == nil {
		t.Fatal("expected error for renamed text file with .mp4 extension, got nil")
	}
}
