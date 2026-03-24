package outbound

import (
	"regexp"
	"strings"

	"github.com/openclaw/qqbot/internal/utils"
)

// Media tag constants for tag name mapping.
const (
	MediaTagImage = "qqimg"
	MediaTagVoice = "qqvoice"
	MediaTagVideo = "qqvideo"
	MediaTagFile  = "qqfile"
)

// ParsedMediaTag represents a parsed media tag from AI output.
type ParsedMediaTag struct {
	Type     string // "image", "voice", "video", "file"
	SrcType  string // "url" or "file"
	Path     string // URL or file path
	Caption  string // optional text content
	RawMatch string // original matched text (for replacement)
}

var mediaTagRegex = regexp.MustCompile(`<(qqimg|qqvoice|qqvideo|qqfile)>([^<>]+)</(?:qqimg|qqvoice|qqvideo|qqfile|img)>`)
var mediaTagStripRegex = regexp.MustCompile(`<(?:qqimg|qqvoice|qqvideo|qqfile)[^>]*>[^<>]*</(?:qqimg|qqvoice|qqvideo|qqfile|img)>`)

var tagTypeMap = map[string]string{
	"qqimg":   "image",
	"qqvoice": "voice",
	"qqvideo": "video",
	"qqfile":  "file",
}

// ParseMediaTags extracts all media tags from AI output text.
// It uses NormalizeMediaTags from utils to fix malformed tags first.
func ParseMediaTags(text string) []ParsedMediaTag {
	normalized := utils.NormalizeMediaTags(text)

	matches := mediaTagRegex.FindAllStringSubmatch(normalized, -1)
	if len(matches) == 0 {
		return nil
	}

	var tags []ParsedMediaTag
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}

		tagName := strings.ToLower(m[1])
		content := strings.TrimSpace(m[2])
		if content == "" {
			continue
		}

		tagType := tagTypeMap[tagName]
		if tagType == "" {
			tagType = "image"
		}

		srcType := "file"
		if strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://") {
			srcType = "url"
		}

		tags = append(tags, ParsedMediaTag{
			Type:     tagType,
			SrcType:  srcType,
			Path:     content,
			RawMatch: m[0],
		})
	}

	return tags
}

// StripMediaTags removes all media tags from text, returning clean text.
func StripMediaTags(text string) string {
	normalized := utils.NormalizeMediaTags(text)
	return mediaTagStripRegex.ReplaceAllString(normalized, "")
}
