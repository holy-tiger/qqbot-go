package image

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DefaultTTL is the default time-to-live for stored images.
const DefaultTTL = 24 * time.Hour

// ImageServerConfig holds configuration for the image server.
type ImageServerConfig struct {
	Port int
	Dir  string
	TTL  time.Duration // default 24h if zero
}

type imageEntry struct {
	filename  string
	mimeType  string
	createdAt time.Time
}

// ImageServer serves images from a local directory over HTTP.
type ImageServer struct {
	server        *http.Server
	dir           string
	port          int
	index         map[string]*imageEntry
	mu            sync.RWMutex
	ttl           time.Duration
	running       bool
	cleanupCancel context.CancelFunc // P2-12: cancel cleanup goroutine
	cleanupDone   chan struct{}
}

// imagePathRegex matches /images/{uuid}.{ext}
var imagePathRegex = regexp.MustCompile(`^/images/([a-f0-9]{32})\.(\w+)$`)

// NewImageServer creates a new ImageServer with the given config.
func NewImageServer(cfg ImageServerConfig) *ImageServer {
	ttl := cfg.TTL
	if ttl == 0 {
		ttl = DefaultTTL
	}
	if cfg.Dir == "" {
		cfg.Dir = "./qqbot-images"
	}
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil
	}
	return &ImageServer{
		dir:         cfg.Dir,
		port:        cfg.Port,
		index:       make(map[string]*imageEntry),
		ttl:         ttl,
		cleanupDone: make(chan struct{}),
	}
}

// Start begins listening and serving HTTP requests.
func (s *ImageServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create image directory: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	s.server = &http.Server{Handler: mux}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	// Get actual port (important if port was 0)
	addr := listener.Addr().(*net.TCPAddr)
	s.port = addr.Port

	s.running = true
	// P2-12: start periodic cleanup
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	s.cleanupCancel = cleanupCancel
	go s.cleanupLoop(cleanupCtx)
	go func() {
		s.server.Serve(listener)
	}()

	return nil
}

// Stop shuts down the HTTP server.
func (s *ImageServer) Stop() error {
	s.mu.Lock()
	wasRunning := s.running
	s.running = false
	s.mu.Unlock()

	if wasRunning {
		if s.cleanupCancel != nil {
			s.cleanupCancel()
		}
		<-s.cleanupDone // wait for cleanupLoop to exit
		if s.server != nil {
			return s.server.Close()
		}
	}
	return nil
}

// StoreImage saves image data and returns the relative URL path.
func (s *ImageServer) StoreImage(uuid string, data []byte) (string, error) {
	// Validate UUID is 32 hex chars
	if len(uuid) != 32 {
		return "", fmt.Errorf("invalid uuid length: %d", len(uuid))
	}
	if _, err := hex.DecodeString(uuid); err != nil {
		return "", fmt.Errorf("invalid uuid hex: %w", err)
	}

	ext := detectExtension(data)
	mimeType := detectMIMEType(ext)
	filename := uuid + "." + ext

	filePath := filepath.Join(s.dir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("write image: %w", err)
	}

	s.mu.Lock()
	s.index[uuid] = &imageEntry{
		filename:  filename,
		mimeType:  mimeType,
		createdAt: time.Now(),
	}
	s.mu.Unlock()

	return fmt.Sprintf("/images/%s.%s", uuid, ext), nil
}

// GetImageURL returns the URL path for a stored image, or empty string if not found.
func (s *ImageServer) GetImageURL(uuid string) string {
	s.mu.RLock()
	entry, ok := s.index[uuid]
	s.mu.RUnlock()

	if !ok {
		return ""
	}
	return "/images/" + entry.filename
}

// IsRunning returns true if the server is currently running.
func (s *ImageServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Handler returns the HTTP handler for testing purposes.
func (s *ImageServer) Handler() http.Handler {
	return http.HandlerFunc(s.handleRequest)
}

// cleanupExpired removes expired images from disk and index.
func (s *ImageServer) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for uuid, entry := range s.index {
		if now.Sub(entry.createdAt) > s.ttl {
			os.Remove(filepath.Join(s.dir, entry.filename))
			delete(s.index, uuid)
		}
	}
}

// P2-12: cleanupLoop runs periodic cleanup of expired images.
func (s *ImageServer) cleanupLoop(ctx context.Context) {
	defer close(s.cleanupDone)
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupExpired()
		case <-ctx.Done():
			return
		}
	}
}

func (s *ImageServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")

	if r.Method == "OPTIONS" {
		w.WriteHeader(204)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}

	// Path pattern: /images/{uuid}.{ext}
	match := imagePathRegex.FindStringSubmatch(r.URL.Path)
	if match == nil {
		w.WriteHeader(404)
		return
	}

	uuid := match[1]
	_ = match[2] // ext from URL

	s.mu.RLock()
	entry, ok := s.index[uuid]
	s.mu.RUnlock()

	if !ok {
		w.WriteHeader(404)
		return
	}

	// Check TTL
	if s.ttl > 0 && time.Since(entry.createdAt) > s.ttl {
		w.WriteHeader(410)
		return
	}

	// Path safety check
	filePath := filepath.Join(s.dir, entry.filename)
	absDir, err := filepath.Abs(s.dir)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !strings.HasPrefix(absFile, absDir+string(filepath.Separator)) && absFile != absDir {
		w.WriteHeader(403)
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		w.WriteHeader(404)
		return
	}

	w.Header().Set("Content-Type", entry.mimeType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(200)
	w.Write(data)
}

func detectExtension(data []byte) string {
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "png"
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "jpg"
	}
	if len(data) >= 6 && (string(data[0:6]) == "GIF87a" || string(data[0:6]) == "GIF89a") {
		return "gif"
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "webp"
	}
	return "png"
}

func detectMIMEType(ext string) string {
	switch ext {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
