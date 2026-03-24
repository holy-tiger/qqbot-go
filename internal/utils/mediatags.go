package utils

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// validTags are the standard media tag names.
var validTags = []string{"qqimg", "qqvoice", "qqvideo", "qqfile"}

// tagAliases maps variant tag names (lowercase) to standard names.
var tagAliases = map[string]string{
	// qqimg variants
	"qq_img":     "qqimg",
	"qqimage":    "qqimg",
	"qq_image":   "qqimg",
	"qqpic":      "qqimg",
	"qq_pic":     "qqimg",
	"qqpicture":  "qqimg",
	"qq_picture": "qqimg",
	"qqphoto":    "qqimg",
	"qq_photo":   "qqimg",
	"img":        "qqimg",
	"image":      "qqimg",
	"pic":        "qqimg",
	"picture":    "qqimg",
	"photo":      "qqimg",
	// qqvoice variants
	"qq_voice": "qqvoice",
	"qqaudio":  "qqvoice",
	"qq_audio": "qqvoice",
	"voice":    "qqvoice",
	"audio":    "qqvoice",
	// qqvideo variants
	"qq_video": "qqvideo",
	"video":    "qqvideo",
	// qqfile variants
	"qq_file":  "qqfile",
	"qqdoc":    "qqfile",
	"qq_doc":   "qqfile",
	"file":     "qqfile",
	"doc":      "qqfile",
	"document": "qqfile",
}

var tagNamePattern string
var multilineTagCleanup *regexp.Regexp
var fuzzyMediaTagRegex *regexp.Regexp

// Unicode characters for full-width brackets
const (
	fullWidthLessThan  = "\uFF1C" // ＜
	fullWidthGreaterThan = "\uFF1E" // ＞
)

func init() {
	set := make(map[string]bool)
	for _, tag := range validTags {
		set[tag] = true
	}
	for alias := range tagAliases {
		set[alias] = true
	}
	allTagNames := make([]string, 0, len(set))
	for name := range set {
		allTagNames = append(allTagNames, name)
	}
	sort.Slice(allTagNames, func(i, j int) bool {
		return len(allTagNames[i]) > len(allTagNames[j])
	})
	tagNamePattern = strings.Join(allTagNames, "|")

	// Build bracket patterns with both ASCII and full-width chars
	ob := "[" + "<" + fullWidthLessThan + "]"
	cb := "[" + ">" + fullWidthGreaterThan + "]"

	multilineTagCleanup = regexp.MustCompile(
		fmt.Sprintf("(%s\\s*(?:%s)\\s*%s)([\\s\\S]*?)(%s\\s*/?\\s*(?:%s)\\s*%s)",
			ob, tagNamePattern, cb, ob, tagNamePattern, cb),
	)

	fuzzyMediaTagRegex = regexp.MustCompile(
		fmt.Sprintf("`?%s\\s*(%s)\\s*%s[\"']?\\s*([^%s\"'`]+?)\\s*[\"']?%s\\s*/?\\s*(?:%s)\\s*%s`?",
			ob, tagNamePattern, cb, "<"+fullWidthLessThan+">"+fullWidthGreaterThan, ob, tagNamePattern, cb),
	)
}

// resolveTagName maps a raw tag name to its standard form.
func resolveTagName(raw string) string {
	lower := strings.ToLower(raw)
	for _, tag := range validTags {
		if lower == tag {
			return tag
		}
	}
	if canonical, ok := tagAliases[lower]; ok {
		return canonical
	}
	return "qqimg"
}

// NormalizeMediaTags preprocesses LLM output to fix malformed media tags.
// Standard format: <qqimg>/path/to/file</qqimg>
func NormalizeMediaTags(text string) string {
	// Pre-cleanup: compress newlines/tabs inside tags to spaces
	cleaned := multilineTagCleanup.ReplaceAllStringFunc(text, func(match string) string {
		sub := multilineTagCleanup.FindStringSubmatch(match)
		if len(sub) < 4 {
			return match
		}
		open := sub[1]
		body := sub[2]
		close := sub[3]
		flat := strings.ReplaceAll(body, "\r", " ")
		flat = strings.ReplaceAll(flat, "\n", " ")
		flat = strings.ReplaceAll(flat, "\t", " ")
		for strings.Contains(flat, "  ") {
			flat = strings.ReplaceAll(flat, "  ", " ")
		}
		return open + flat + close
	})

	return fuzzyMediaTagRegex.ReplaceAllStringFunc(cleaned, func(match string) string {
		sub := fuzzyMediaTagRegex.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		rawTag := sub[1]
		content := strings.TrimSpace(sub[2])
		if content == "" {
			return match
		}
		tag := resolveTagName(rawTag)
		return "<" + tag + ">" + content + "</" + tag + ">"
	})
}
