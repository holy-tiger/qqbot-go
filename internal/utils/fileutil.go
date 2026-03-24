package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxUploadSize is the maximum file upload size (20MB).
	MaxUploadSize = 20 * 1024 * 1024
	// LargeFileThreshold is the threshold for "large" files (5MB).
	LargeFileThreshold = 5 * 1024 * 1024
)

// FileSizeCheckResult holds the result of a file size check.
type FileSizeCheckResult struct {
	OK    bool
	Size  int64
	Error string
}

// CheckFileSize checks if a file's size is within the given limit.
func CheckFileSize(path string, maxSize int64) FileSizeCheckResult {
	info, err := os.Stat(path)
	if err != nil {
		return FileSizeCheckResult{
			OK:    false,
			Size:  0,
			Error: fmt.Sprintf("cannot read file: %v", err),
		}
	}
	if info.Size() > maxSize {
		return FileSizeCheckResult{
			OK:    false,
			Size:  info.Size(),
			Error: fmt.Sprintf("file too large (%.1fMB), limit is %dMB", float64(info.Size())/(1024*1024), maxSize/(1024*1024)),
		}
	}
	return FileSizeCheckResult{OK: true, Size: info.Size()}
}

// ReadFileAsync reads a file asynchronously.
func ReadFileAsync(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// FileExists returns true if the file at path exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetFileSize returns the size of the file at path.
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// IsLargeFile returns true if the size exceeds the large file threshold.
func IsLargeFile(size int64) bool {
	return size >= LargeFileThreshold
}

// FormatFileSize formats bytes as a human-readable string.
func FormatFileSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
}

// GetMimeType returns the MIME type based on file extension.
func GetMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
