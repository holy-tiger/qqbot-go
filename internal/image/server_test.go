package image

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func testUUID(s string) string {
	// Convert any string to a 32-char hex string by padding/truncating
	h := s
	for len(h) < 32 {
		h += h
	}
	if len(h) > 32 {
		h = h[:32]
	}
	// Ensure all chars are valid hex
	valid := "0123456789abcdef"
	result := make([]byte, 32)
	for i := range result {
		result[i] = valid[int(h[i])%16]
	}
	return string(result)
}

func TestNewImageServer(t *testing.T) {
	dir := t.TempDir()
	cfg := ImageServerConfig{Port: 0, Dir: dir}
	s := NewImageServer(cfg)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.IsRunning() {
		t.Fatal("new server should not be running")
	}
}

func TestImageServer_StartStop(t *testing.T) {
	dir := t.TempDir()
	cfg := ImageServerConfig{Port: 0, Dir: dir, TTL: time.Hour}
	s := NewImageServer(cfg)

	err := s.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !s.IsRunning() {
		t.Fatal("should be running after Start")
	}

	err = s.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if s.IsRunning() {
		t.Fatal("should not be running after Stop")
	}
}

func TestImageServer_StoreAndGetURL(t *testing.T) {
	dir := t.TempDir()
	cfg := ImageServerConfig{Port: 0, Dir: dir, TTL: time.Hour}
	s := NewImageServer(cfg)
	defer s.Stop()

	if err := s.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	uuid := "abc12345678901234567890123456789"
	data := []byte("fake-image-data-png")

	url, err := s.StoreImage(uuid, data)
	if err != nil {
		t.Fatalf("StoreImage failed: %v", err)
	}

	if !strings.Contains(url, "/images/"+uuid) {
		t.Errorf("URL %q should contain /images/%s", url, uuid)
	}

	files, _ := os.ReadDir(dir)
	found := false
	for _, f := range files {
		if strings.HasPrefix(f.Name(), uuid) {
			found = true
			break
		}
	}
	if !found {
		t.Error("no file written to disk for stored image")
	}

	gotURL := s.GetImageURL(uuid)
	if gotURL != url {
		t.Errorf("GetImageURL = %q, want %q", gotURL, url)
	}
}

func TestImageServer_GetImageURL_NotFound(t *testing.T) {
	dir := t.TempDir()
	cfg := ImageServerConfig{Port: 0, Dir: dir}
	s := NewImageServer(cfg)
	defer s.Stop()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	url := s.GetImageURL("aabbccdd11223344aabbccdd11223344")
	if url != "" {
		t.Errorf("expected empty URL for nonexistent uuid, got %q", url)
	}
}

func TestImageServer_CORSHeaders(t *testing.T) {
	dir := t.TempDir()
	cfg := ImageServerConfig{Port: 0, Dir: dir, TTL: time.Hour}
	s := NewImageServer(cfg)
	defer s.Stop()

	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	uuid := testUUID("cors-test-uuid-1")
	s.StoreImage(uuid, []byte("data"))
	urlPath := s.GetImageURL(uuid)
	idx := strings.Index(urlPath, "/images/")
	if idx < 0 {
		t.Fatal("URL should contain /images/")
	}
	path := urlPath[idx:]

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", path, nil)
	s.Handler().ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS Access-Control-Allow-Origin: *")
	}
	if w.Header().Get("Access-Control-Allow-Methods") != "GET, OPTIONS" {
		t.Error("missing CORS Access-Control-Allow-Methods")
	}
}

func TestImageServer_OptionsMethod(t *testing.T) {
	dir := t.TempDir()
	cfg := ImageServerConfig{Port: 0, Dir: dir, TTL: time.Hour}
	s := NewImageServer(cfg)
	defer s.Stop()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("OPTIONS", "/images/test.png", nil)
	s.Handler().ServeHTTP(w, r)

	if w.Code != 204 {
		t.Errorf("OPTIONS status = %d, want 204", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header on OPTIONS")
	}
}

func TestImageServer_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	cfg := ImageServerConfig{Port: 0, Dir: dir, TTL: time.Hour}
	s := NewImageServer(cfg)
	defer s.Stop()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/images/../etc/passwd", nil)
	s.Handler().ServeHTTP(w, r)

	if w.Code != 404 {
		t.Errorf("traversal request status = %d, want 404", w.Code)
	}
}

func TestImageServer_TTLExpiry(t *testing.T) {
	dir := t.TempDir()
	cfg := ImageServerConfig{Port: 0, Dir: dir, TTL: time.Nanosecond}
	s := NewImageServer(cfg)
	defer s.Stop()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	uuid := testUUID("ttl-expiry-test-uuid")
	s.StoreImage(uuid, []byte("data"))

	url := s.GetImageURL(uuid)
	if url == "" {
		t.Fatal("expected URL")
	}

	time.Sleep(10 * time.Millisecond)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", url, nil)
	s.Handler().ServeHTTP(w, r)

	if w.Code != 410 {
		t.Errorf("expired image status = %d, want 410", w.Code)
	}
}

func TestImageServer_ServeImage(t *testing.T) {
	dir := t.TempDir()
	cfg := ImageServerConfig{Port: 0, Dir: dir, TTL: time.Hour}
	s := NewImageServer(cfg)
	defer s.Stop()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	uuid := testUUID("serve-image-test")
	imageData := []byte("PNGDATAFAKEIMAGE")
	s.StoreImage(uuid, imageData)

	url := s.GetImageURL(uuid)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", url, nil)
	s.Handler().ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if string(w.Body.Bytes()) != string(imageData) {
		t.Errorf("body = %q, want %q", w.Body.String(), string(imageData))
	}
}

func TestImageServer_NotFound(t *testing.T) {
	dir := t.TempDir()
	cfg := ImageServerConfig{Port: 0, Dir: dir, TTL: time.Hour}
	s := NewImageServer(cfg)
	defer s.Stop()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/images/aabbccdd11223344aabbccdd11223344.png", nil)
	s.Handler().ServeHTTP(w, r)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestImageServer_DuplicateStore(t *testing.T) {
	dir := t.TempDir()
	cfg := ImageServerConfig{Port: 0, Dir: dir, TTL: time.Hour}
	s := NewImageServer(cfg)
	defer s.Stop()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	uuid := testUUID("duplicate-store-test")
	url1, _ := s.StoreImage(uuid, []byte("first"))
	url2, _ := s.StoreImage(uuid, []byte("second"))
	if url1 != url2 {
		t.Errorf("storing same UUID should return same URL, got %q and %q", url1, url2)
	}
}
