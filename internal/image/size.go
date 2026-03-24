package image

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"regexp"
)

var (
	ErrUnknownFormat = errors.New("unknown image format")
	ErrDataTooShort  = errors.New("image data too short")
)

// ImageSize holds width and height dimensions.
type ImageSize struct {
	Width  int
	Height int
}

// ParseImageSize reads PNG/JPEG/GIF/WebP headers to get dimensions without full decode.
func ParseImageSize(data []byte) (*ImageSize, error) {
	if len(data) < 4 {
		return nil, ErrDataTooShort
	}

	// Try PNG
	if s, ok := parsePNGSize(data); ok {
		return s, nil
	}
	// Try JPEG
	if s, ok := parseJPEGSize(data); ok {
		return s, nil
	}
	// Try GIF
	if s, ok := parseGIFSize(data); ok {
		return s, nil
	}
	// Try WebP
	if s, ok := parseWebPSize(data); ok {
		return s, nil
	}

	return nil, ErrUnknownFormat
}

// ParseImageSizeFile reads dimensions from a file.
func ParseImageSizeFile(path string) (*ImageSize, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read image file: %w", err)
	}
	return ParseImageSize(data)
}

// FormatQQBotMarkdownImage generates QQ markdown image syntax: ![#Wpx #Hpx](url).
func FormatQQBotMarkdownImage(url string, width, height int) string {
	return fmt.Sprintf("![#%dpx #%dpx](%s)", width, height, url)
}

// HasQQBotImageSize checks if data has parseable image dimensions in QQ Bot markdown format.
func HasQQBotImageSize(data []byte) bool {
	return qqBotImageSizeRegex.Match(data)
}

var qqBotImageSizeRegex = regexp.MustCompile(`!\[#\d+px\s+#\d+px\]`)

// parsePNGSize extracts dimensions from PNG header.
func parsePNGSize(data []byte) (*ImageSize, bool) {
	if len(data) < 24 {
		return nil, false
	}
	// PNG signature: 89 50 4E 47 0D 0A 1A 0A
	if data[0] != 0x89 || data[1] != 0x50 || data[2] != 0x4E || data[3] != 0x47 {
		return nil, false
	}
	width := binary.BigEndian.Uint32(data[16:20])
	height := binary.BigEndian.Uint32(data[20:24])
	return &ImageSize{Width: int(width), Height: int(height)}, true
}

// parseJPEGSize extracts dimensions from JPEG SOF markers.
func parseJPEGSize(data []byte) (*ImageSize, bool) {
	if len(data) < 4 {
		return nil, false
	}
	// JPEG signature: FF D8
	if data[0] != 0xFF || data[1] != 0xD8 {
		return nil, false
	}

	offset := 2
	for offset < len(data)-9 {
		if data[offset] != 0xFF {
			offset++
			continue
		}

		marker := data[offset+1]
		// SOFn markers: 0xC0-0xC3, 0xC5-0xC7, 0xC9-0xCB, 0xCD-0xCF
		if isSOFMarker(marker) {
			if offset+9 <= len(data) {
				height := binary.BigEndian.Uint16(data[offset+5 : offset+7])
				width := binary.BigEndian.Uint16(data[offset+7 : offset+9])
				return &ImageSize{Width: int(width), Height: int(height)}, true
			}
		}

		// Skip this marker block
		if offset+3 < len(data) {
			blockLen := binary.BigEndian.Uint16(data[offset+2 : offset+4])
			if blockLen < 2 {
				offset++
				continue
			}
			offset += 2 + int(blockLen)
		} else {
			break
		}
	}

	return nil, false
}

func isSOFMarker(b byte) bool {
	// SOF0=0xC0, SOF1=0xC1, SOF2=0xC2, SOF3=0xC3
	// SOF5=0xC5, SOF6=0xC6, SOF7=0xC7
	// SOF9=0xC9, SOF10=0xCA, SOF11=0xCB
	// SOF13=0xCD, SOF14=0xCE, SOF15=0xCF
	switch b {
	case 0xC0, 0xC1, 0xC2, 0xC3,
		0xC5, 0xC6, 0xC7,
		0xC9, 0xCA, 0xCB,
		0xCD, 0xCE, 0xCF:
		return true
	}
	return false
}

// parseGIFSize extracts dimensions from GIF header.
func parseGIFSize(data []byte) (*ImageSize, bool) {
	if len(data) < 10 {
		return nil, false
	}
	sig := string(data[0:6])
	if sig != "GIF87a" && sig != "GIF89a" {
		return nil, false
	}
	width := binary.LittleEndian.Uint16(data[6:8])
	height := binary.LittleEndian.Uint16(data[8:10])
	return &ImageSize{Width: int(width), Height: int(height)}, true
}

// parseWebPSize extracts dimensions from WebP header.
func parseWebPSize(data []byte) (*ImageSize, bool) {
	if len(data) < 25 {
		return nil, false
	}

	// Check RIFF and WEBP signatures
	riff := string(data[0:4])
	webp := string(data[8:12])
	if riff != "RIFF" || webp != "WEBP" {
		return nil, false
	}

	chunkType := string(data[12:16])

	switch chunkType {
	case "VP8 ":
		// VP8 lossy: check signature 9D 01 2A at offset 20
		if len(data) >= 30 && data[20] == 0x9D && data[21] == 0x01 && data[22] == 0x2A {
			width := binary.LittleEndian.Uint16(data[26:28]) & 0x3FFF
			height := binary.LittleEndian.Uint16(data[28:30]) & 0x3FFF
			return &ImageSize{Width: int(width), Height: int(height)}, true
		}
	case "VP8L":
		// VP8L lossless: signature byte 0x2F at offset 20
		if len(data) >= 25 && data[20] == 0x2F {
			bits := binary.LittleEndian.Uint32(data[21:25])
			width := int(bits&0x3FFF) + 1
			height := int((bits>>14)&0x3FFF) + 1
			return &ImageSize{Width: width, Height: height}, true
		}
	case "VP8X":
		// VP8X extended: 24-bit little-endian width/height + 1
		if len(data) >= 30 {
			width := int(uint(data[24]) | uint(data[25])<<8 | uint(data[26])<<16) + 1
			height := int(uint(data[27]) | uint(data[28])<<8 | uint(data[29])<<16) + 1
			return &ImageSize{Width: width, Height: height}, true
		}
	}

	return nil, false
}
