package outbound

import (
	"testing"
)

func TestParseMediaTags_ImageTag(t *testing.T) {
	text := "check this out <qqimg>/path/to/image.png</qqimg> nice right?"
	tags := ParseMediaTags(text)

	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].Type != "image" {
		t.Errorf("expected type 'image', got '%s'", tags[0].Type)
	}
	if tags[0].Path != "/path/to/image.png" {
		t.Errorf("expected path '/path/to/image.png', got '%s'", tags[0].Path)
	}
	if tags[0].RawMatch != "<qqimg>/path/to/image.png</qqimg>" {
		t.Errorf("raw match mismatch: '%s'", tags[0].RawMatch)
	}
}

func TestParseMediaTags_VoiceTag(t *testing.T) {
	text := "<qqvoice>/tmp/voice.silk</qqvoice>"
	tags := ParseMediaTags(text)

	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].Type != "voice" {
		t.Errorf("expected type 'voice', got '%s'", tags[0].Type)
	}
	if tags[0].Path != "/tmp/voice.silk" {
		t.Errorf("expected path '/tmp/voice.silk', got '%s'", tags[0].Path)
	}
}

func TestParseMediaTags_VideoTag(t *testing.T) {
	text := "<qqvideo>https://example.com/video.mp4</qqvideo>"
	tags := ParseMediaTags(text)

	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].Type != "video" {
		t.Errorf("expected type 'video', got '%s'", tags[0].Type)
	}
	if tags[0].SrcType != "url" {
		t.Errorf("expected srcType 'url', got '%s'", tags[0].SrcType)
	}
}

func TestParseMediaTags_FileTag(t *testing.T) {
	text := "<qqfile>/data/document.pdf</qqfile>"
	tags := ParseMediaTags(text)

	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].Type != "file" {
		t.Errorf("expected type 'file', got '%s'", tags[0].Type)
	}
	if tags[0].Path != "/data/document.pdf" {
		t.Errorf("expected path '/data/document.pdf', got '%s'", tags[0].Path)
	}
}

func TestParseMediaTags_MixedTextAndTags(t *testing.T) {
	text := "Hello! Here is a photo <qqimg>/img/cat.jpg</qqimg> and a voice <qqvoice>/audio/hello.silk</qqvoice> enjoy!"
	tags := ParseMediaTags(text)

	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0].Type != "image" {
		t.Errorf("expected first tag type 'image', got '%s'", tags[0].Type)
	}
	if tags[1].Type != "voice" {
		t.Errorf("expected second tag type 'voice', got '%s'", tags[1].Type)
	}
}

func TestParseMediaTags_MultipleImageTags(t *testing.T) {
	text := "<qqimg>/a.png</qqimg><qqimg>/b.png</qqimg><qqimg>/c.png</qqimg>"
	tags := ParseMediaTags(text)

	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}
	for i, tag := range tags {
		if tag.Type != "image" {
			t.Errorf("tag[%d]: expected type 'image', got '%s'", i, tag.Type)
		}
	}
	expectedPaths := []string{"/a.png", "/b.png", "/c.png"}
	for i, tag := range tags {
		if tag.Path != expectedPaths[i] {
			t.Errorf("tag[%d]: expected path '%s', got '%s'", i, expectedPaths[i], tag.Path)
		}
	}
}

func TestParseMediaTags_NoTags(t *testing.T) {
	tags := ParseMediaTags("just plain text here")
	if len(tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(tags))
	}
}

func TestParseMediaTags_EmptyText(t *testing.T) {
	tags := ParseMediaTags("")
	if len(tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(tags))
	}
}

func TestParseMediaTags_NormalizedTags(t *testing.T) {
	// Test that NormalizeMediaTags is called and malformed tags are fixed
	text := "look at this <img>/photo.png</img>"
	tags := ParseMediaTags(text)

	if len(tags) != 1 {
		t.Fatalf("expected 1 tag after normalization, got %d", len(tags))
	}
	if tags[0].Type != "image" {
		t.Errorf("expected type 'image' after normalization, got '%s'", tags[0].Type)
	}
}

func TestStripMediaTags_Basic(t *testing.T) {
	text := "before <qqimg>/img.png</qqimg> after"
	stripped := StripMediaTags(text)

	if stripped != "before  after" {
		t.Errorf("expected 'before  after', got '%s'", stripped)
	}
}

func TestStripMediaTags_MultipleTags(t *testing.T) {
	text := "a<qqimg>/1.png</qqimg>b<qqvoice>/v.silk</qqvoice>c"
	stripped := StripMediaTags(text)

	if stripped != "abc" {
		t.Errorf("expected 'abc', got '%s'", stripped)
	}
}

func TestStripMediaTags_NoTags(t *testing.T) {
	text := "just text here"
	stripped := StripMediaTags(text)

	if stripped != "just text here" {
		t.Errorf("expected 'just text here', got '%s'", stripped)
	}
}

func TestStripMediaTags_Empty(t *testing.T) {
	stripped := StripMediaTags("")
	if stripped != "" {
		t.Errorf("expected empty string, got '%s'", stripped)
	}
}

func TestParseMediaTags_URLEntry(t *testing.T) {
	text := "<qqimg>https://example.com/photo.jpg</qqimg>"
	tags := ParseMediaTags(text)

	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].SrcType != "url" {
		t.Errorf("expected srcType 'url', got '%s'", tags[0].SrcType)
	}
	if tags[0].Path != "https://example.com/photo.jpg" {
		t.Errorf("expected URL path, got '%s'", tags[0].Path)
	}
}

func TestParseMediaTags_PathSrcType(t *testing.T) {
	text := "<qqfile>/local/doc.pdf</qqfile>"
	tags := ParseMediaTags(text)

	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].SrcType != "file" {
		t.Errorf("expected srcType 'file', got '%s'", tags[0].SrcType)
	}
}
