package image

import (
	"encoding/binary"
	"testing"
)

// Helper to build a minimal PNG with given width/height
func makePNG(w, h uint32) []byte {
	buf := make([]byte, 24)
	// PNG signature
	copy(buf[0:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	// IHDR chunk: length(4) + "IHDR"(4) + width(4) + height(4)
	binary.BigEndian.PutUint32(buf[8:12], 13) // chunk data length
	copy(buf[12:16], []byte("IHDR"))
	binary.BigEndian.PutUint32(buf[16:20], w)
	binary.BigEndian.PutUint32(buf[20:24], h)
	return buf
}

// Helper to build a minimal JPEG with SOF0 marker
func makeJPEG(w, h uint16) []byte {
	buf := make([]byte, 20)
	buf[0] = 0xFF
	buf[1] = 0xD8 // SOI
	// SOF0 marker at offset 2
	buf[2] = 0xFF
	buf[3] = 0xC0
	binary.BigEndian.PutUint16(buf[4:6], 11) // length
	buf[6] = 8                                // precision
	binary.BigEndian.PutUint16(buf[7:9], h)   // height
	binary.BigEndian.PutUint16(buf[9:11], w)  // width
	return buf
}

// Helper to build a minimal JPEG with SOF2 marker
func makeJPEGWithSOF2(w, h uint16) []byte {
	buf := make([]byte, 20)
	buf[0] = 0xFF
	buf[1] = 0xD8
	buf[2] = 0xFF
	buf[3] = 0xC2 // SOF2
	binary.BigEndian.PutUint16(buf[4:6], 11)
	buf[6] = 8
	binary.BigEndian.PutUint16(buf[7:9], h)
	binary.BigEndian.PutUint16(buf[9:11], w)
	return buf
}

// Helper to build a minimal GIF
func makeGIF(w, h uint16) []byte {
	buf := make([]byte, 10)
	copy(buf[0:6], []byte("GIF89a"))
	binary.LittleEndian.PutUint16(buf[6:8], w)
	binary.LittleEndian.PutUint16(buf[8:10], h)
	return buf
}

// Helper to build a minimal WebP (VP8 lossy)
func makeWebPVP8(w, h uint16) []byte {
	buf := make([]byte, 30)
	copy(buf[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(buf[4:8], 30) // file size
	copy(buf[8:12], []byte("WEBP"))
	copy(buf[12:16], []byte("VP8 "))
	// VP8 bitstream starts at 20
	// 3-byte signature: 9D 01 2A
	buf[20] = 0x9D
	buf[21] = 0x01
	buf[22] = 0x2A
	// width and height at offset 26, 28 (little-endian with 14-bit mask)
	binary.LittleEndian.PutUint16(buf[26:28], w&0x3FFF)
	binary.LittleEndian.PutUint16(buf[28:30], h&0x3FFF)
	return buf
}

// Helper to build a minimal WebP (VP8L lossless)
func makeWebPVP8L(w, h uint16) []byte {
	buf := make([]byte, 25)
	copy(buf[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(buf[4:8], 25)
	copy(buf[8:12], []byte("WEBP"))
	copy(buf[12:16], []byte("VP8L"))
	buf[20] = 0x2F // VP8L signature byte
	// bits encode width-1 and height-1 in 14-bit fields
	bits := uint32(w-1) | uint32(h-1)<<14
	binary.LittleEndian.PutUint32(buf[21:25], bits)
	return buf
}

func TestParseImageSize_PNG(t *testing.T) {
	data := makePNG(800, 600)
	size, err := ParseImageSize(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size.Width != 800 || size.Height != 600 {
		t.Errorf("got %dx%d, want 800x600", size.Width, size.Height)
	}
}

func TestParseImageSize_JPEG_SOF0(t *testing.T) {
	data := makeJPEG(1920, 1080)
	size, err := ParseImageSize(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size.Width != 1920 || size.Height != 1080 {
		t.Errorf("got %dx%d, want 1920x1080", size.Width, size.Height)
	}
}

func TestParseImageSize_JPEG_SOF2(t *testing.T) {
	data := makeJPEGWithSOF2(640, 480)
	size, err := ParseImageSize(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size.Width != 640 || size.Height != 480 {
		t.Errorf("got %dx%d, want 640x480", size.Width, size.Height)
	}
}

func TestParseImageSize_GIF(t *testing.T) {
	data := makeGIF(100, 200)
	size, err := ParseImageSize(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size.Width != 100 || size.Height != 200 {
		t.Errorf("got %dx%d, want 100x200", size.Width, size.Height)
	}
}

func TestParseImageSize_WebP_VP8(t *testing.T) {
	data := makeWebPVP8(400, 300)
	size, err := ParseImageSize(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size.Width != 400 || size.Height != 300 {
		t.Errorf("got %dx%d, want 400x300", size.Width, size.Height)
	}
}

func TestParseImageSize_WebP_VP8L(t *testing.T) {
	data := makeWebPVP8L(500, 400)
	size, err := ParseImageSize(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size.Width != 500 || size.Height != 400 {
		t.Errorf("got %dx%d, want 500x400", size.Width, size.Height)
	}
}

func TestParseImageSize_UnknownFormat(t *testing.T) {
	data := []byte("this is not an image")
	_, err := ParseImageSize(data)
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestParseImageSize_TooShort(t *testing.T) {
	_, err := ParseImageSize([]byte{0x89, 0x50})
	if err == nil {
		t.Fatal("expected error for too-short data")
	}
}

func TestHasQQBotImageSize(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"![#100px #200px](url)", true},
		{"![#0px #0px](url)", true},
		{"![image](url)", false},
		{"![#abcpx #200px](url)", false},
		{"no image here", false},
	}
	for _, tt := range tests {
		got := HasQQBotImageSize([]byte(tt.input))
		if got != tt.want {
			t.Errorf("HasQQBotImageSize(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestFormatQQBotMarkdownImage(t *testing.T) {
	got := FormatQQBotMarkdownImage("http://example.com/img.png", 320, 240)
	want := "![#320px #240px](http://example.com/img.png)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseImageSizeFile(t *testing.T) {
	f := createTempFile(t, makePNG(640, 480))
	defer f.remove()

	size, err := ParseImageSizeFile(f.path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size.Width != 640 || size.Height != 480 {
		t.Errorf("got %dx%d, want 640x480", size.Width, size.Height)
	}
}

func TestParseImageSizeFile_NotFound(t *testing.T) {
	_, err := ParseImageSizeFile("/nonexistent/path/image.png")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
