package image

import (
	"os"
	"testing"
)

type tempFile struct {
	path string
}

func createTempFile(t *testing.T, data []byte) *tempFile {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "img-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	_, err = f.Write(data)
	if err != nil {
		f.Close()
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return &tempFile{path: f.Name()}
}

func (f *tempFile) remove() {
	os.Remove(f.path)
}
