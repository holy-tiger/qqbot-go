package utils

import (
	"os"
	"testing"
)

func TestCheckFileSize_OK(t *testing.T) {
	f := createTestFile(t, 1024)
	defer os.Remove(f)

	result := CheckFileSize(f, 10*1024)
	if !result.OK {
		t.Errorf("expected OK, got error: %s", result.Error)
	}
	if result.Size != 1024 {
		t.Errorf("got size %d, want 1024", result.Size)
	}
}

func TestCheckFileSize_TooLarge(t *testing.T) {
	f := createTestFile(t, 30*1024*1024)
	defer os.Remove(f)

	result := CheckFileSize(f, 20*1024*1024)
	if result.OK {
		t.Error("expected OK=false for oversized file")
	}
	if result.Error == "" {
		t.Error("expected error message for oversized file")
	}
}

func TestCheckFileSize_NotFound(t *testing.T) {
	result := CheckFileSize("/nonexistent/file.txt", 1024)
	if result.OK {
		t.Error("expected OK=false for nonexistent file")
	}
	if result.Error == "" {
		t.Error("expected error message for nonexistent file")
	}
}

func TestFileExists_True(t *testing.T) {
	f := createTestFile(t, 100)
	defer os.Remove(f)

	if !FileExists(f) {
		t.Error("expected FileExists=true for existing file")
	}
}

func TestFileExists_False(t *testing.T) {
	if FileExists("/nonexistent/file.txt") {
		t.Error("expected FileExists=false for nonexistent file")
	}
}

func TestGetFileSize(t *testing.T) {
	f := createTestFile(t, 4096)
	defer os.Remove(f)

	size, err := GetFileSize(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 4096 {
		t.Errorf("got %d, want 4096", size)
	}
}

func TestGetFileSize_NotFound(t *testing.T) {
	_, err := GetFileSize("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestIsLargeFile(t *testing.T) {
	tests := []struct {
		size int64
		want bool
	}{
		{0, false},
		{1024, false},
		{5 * 1024 * 1024 - 1, false},
		{5 * 1024 * 1024, true},
		{10 * 1024 * 1024, true},
	}
	for _, tt := range tests {
		got := IsLargeFile(tt.size)
		if got != tt.want {
			t.Errorf("IsLargeFile(%d) = %v, want %v", tt.size, got, tt.want)
		}
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0B"},
		{512, "512B"},
		{1023, "1023B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1024 * 1024, "1.0MB"},
		{5 * 1024 * 1024 * 10, "50.0MB"},
	}
	for _, tt := range tests {
		got := FormatFileSize(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatFileSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestGetMimeType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"photo.jpg", "image/jpeg"},
		{"photo.jpeg", "image/jpeg"},
		{"photo.png", "image/png"},
		{"photo.gif", "image/gif"},
		{"photo.webp", "image/webp"},
		{"photo.bmp", "image/bmp"},
		{"video.mp4", "video/mp4"},
		{"video.mov", "video/quicktime"},
		{"video.avi", "video/x-msvideo"},
		{"video.mkv", "video/x-matroska"},
		{"video.webm", "video/webm"},
		{"doc.pdf", "application/pdf"},
		{"doc.doc", "application/msword"},
		{"doc.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"data.xls", "application/vnd.ms-excel"},
		{"data.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"archive.zip", "application/zip"},
		{"archive.tar", "application/x-tar"},
		{"archive.gz", "application/gzip"},
		{"notes.txt", "text/plain"},
		{"unknown.xyz", "application/octet-stream"},
	}
	for _, tt := range tests {
		got := GetMimeType(tt.path)
		if got != tt.want {
			t.Errorf("GetMimeType(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestReadFileAsync(t *testing.T) {
	content := []byte("hello async read")
	f := createTestFileWithData(t, content)
	defer os.Remove(f)

	data, err := ReadFileAsync(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("got %q, want %q", string(data), string(content))
	}
}

func TestReadFileAsync_NotFound(t *testing.T) {
	_, err := ReadFileAsync("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func createTestFile(t *testing.T, size int64) string {
	return createTestFileWithData(t, make([]byte, size))
}

func createTestFileWithData(t *testing.T, data []byte) string {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "test-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	_, err = f.Write(data)
	if err != nil {
		f.Close()
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}
