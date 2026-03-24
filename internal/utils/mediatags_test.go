package utils

import (
	"testing"
)

func TestNormalizeMediaTags_StandardTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "qqimg standard",
			input: "Here is an image: <qqimg>/path/to/file.png</qqimg>",
			want:  "Here is an image: <qqimg>/path/to/file.png</qqimg>",
		},
		{
			name:  "qqvoice standard",
			input: "<qqvoice>/path/to/voice.mp3</qqvoice>",
			want:  "<qqvoice>/path/to/voice.mp3</qqvoice>",
		},
		{
			name:  "qqvideo standard",
			input: "<qqvideo>/path/to/video.mp4</qqvideo>",
			want:  "<qqvideo>/path/to/video.mp4</qqvideo>",
		},
		{
			name:  "qqfile standard",
			input: "<qqfile>/path/to/doc.pdf</qqfile>",
			want:  "<qqfile>/path/to/doc.pdf</qqfile>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeMediaTags(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeMediaTags_Aliases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "qq_img to qqimg",
			input: "<qq_img>/path/to/img.png</qq_img>",
			want:  "<qqimg>/path/to/img.png</qqimg>",
		},
		{
			name:  "qqimage to qqimg",
			input: "<qqimage>/path/to/img.png</qqimage>",
			want:  "<qqimg>/path/to/img.png</qqimg>",
		},
		{
			name:  "image to qqimg",
			input: "<image>/path/to/img.png</image>",
			want:  "<qqimg>/path/to/img.png</qqimg>",
		},
		{
			name:  "img to qqimg",
			input: "<img>/path/to/img.png</img>",
			want:  "<qqimg>/path/to/img.png</qqimg>",
		},
		{
			name:  "pic to qqimg",
			input: "<pic>/path/to/img.png</pic>",
			want:  "<qqimg>/path/to/img.png</qqimg>",
		},
		{
			name:  "photo to qqimg",
			input: "<photo>/path/to/img.png</photo>",
			want:  "<qqimg>/path/to/img.png</qqimg>",
		},
		{
			name:  "picture to qqimg",
			input: "<picture>/path/to/img.png</picture>",
			want:  "<qqimg>/path/to/img.png</qqimg>",
		},
		{
			name:  "qqvoice alias voice",
			input: "<voice>/path/to/voice.mp3</voice>",
			want:  "<qqvoice>/path/to/voice.mp3</qqvoice>",
		},
		{
			name:  "qqvoice alias audio",
			input: "<audio>/path/to/voice.mp3</audio>",
			want:  "<qqvoice>/path/to/voice.mp3</qqvoice>",
		},
		{
			name:  "qqvideo alias video",
			input: "<video>/path/to/video.mp4</video>",
			want:  "<qqvideo>/path/to/video.mp4</qqvideo>",
		},
		{
			name:  "qqfile alias file",
			input: "<file>/path/to/doc.pdf</file>",
			want:  "<qqfile>/path/to/doc.pdf</qqfile>",
		},
		{
			name:  "qqfile alias doc",
			input: "<doc>/path/to/doc.pdf</doc>",
			want:  "<qqfile>/path/to/doc.pdf</qqfile>",
		},
		{
			name:  "qqfile alias document",
			input: "<document>/path/to/doc.pdf</document>",
			want:  "<qqfile>/path/to/doc.pdf</qqfile>",
		},
		{
			name:  "qqpic to qqimg",
			input: "<qqpic>/path/to/img.png</qqpic>",
			want:  "<qqimg>/path/to/img.png</qqimg>",
		},
		{
			name:  "qqphoto to qqimg",
			input: "<qqphoto>/path/to/img.png</qqphoto>",
			want:  "<qqimg>/path/to/img.png</qqimg>",
		},
		{
			name:  "qqaudio to qqvoice",
			input: "<qqaudio>/path/to/voice.mp3</qqaudio>",
			want:  "<qqvoice>/path/to/voice.mp3</qqvoice>",
		},
		{
			name:  "qqdoc to qqfile",
			input: "<qqdoc>/path/to/doc.pdf</qqdoc>",
			want:  "<qqfile>/path/to/doc.pdf</qqfile>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeMediaTags(tt.input)
			if got != tt.want {
				t.Errorf("\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeMediaTags_Malformed(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "extra spaces in tag",
			input: "< qqimg > /path/to/file.png </ qqimg >",
			want:  "<qqimg>/path/to/file.png</qqimg>",
		},
		{
			name:  "backtick wrapped",
			input: "`<qqimg>/path/to/file.png</qqimg>`",
			want:  "<qqimg>/path/to/file.png</qqimg>",
		},
		{
			name:  "quoted path",
			input: `<qqimg>"/path/to/file.png"</qqimg>`,
			want:  "<qqimg>/path/to/file.png</qqimg>",
		},
		{
			name:  "missing slash in close tag",
			input: "<qqimg>/path/to/file.png<qqimg>",
			want:  "<qqimg>/path/to/file.png</qqimg>",
		},
		{
			name:  "mismatched close tag",
			input: "<qqimg>/path/to/file.png</image>",
			want:  "<qqimg>/path/to/file.png</qqimg>",
		},
		{
			name:  "multiline content",
			input: "<qqimg>\n/path/to/\nfile.png\n</qqimg>",
			want:  "<qqimg>/path/to/ file.png</qqimg>",
		},
		{
			name:  "no change for plain text",
			input: "Hello, this is just text",
			want:  "Hello, this is just text",
		},
		{
			name:  "empty tag content unchanged",
			input: "before<qqimg></qqimg>after",
			want:  "before<qqimg></qqimg>after",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeMediaTags(tt.input)
			if got != tt.want {
				t.Errorf("\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}
