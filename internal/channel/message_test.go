package channel

import (
	"testing"

	"github.com/openclaw/qqbot/internal/types"
)

func strPtr(s string) *string { return &s }

func TestAppendAttachmentInfo_NoAttachments(t *testing.T) {
	got := appendAttachmentInfo("hello", nil)
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}

	got = appendAttachmentInfo("hello", []types.MessageAttachment{})
	if got != "hello" {
		t.Errorf("expected %q with empty slice, got %q", "hello", got)
	}
}

func TestAppendAttachmentInfo_Image(t *testing.T) {
	got := appendAttachmentInfo("看看这个", []types.MessageAttachment{
		{ContentType: "image", URL: "https://example.com/img.png"},
	})
	want := "看看这个\n[图片: https://example.com/img.png]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAppendAttachmentInfo_Voice(t *testing.T) {
	tests := []struct {
		name string
		att  types.MessageAttachment
		want string
	}{
		{
			name: "voice with wav and asr",
			att: types.MessageAttachment{
				ContentType:  "voice",
				URL:          "https://example.com/voice.silk",
				VoiceWavURL:  strPtr("https://example.com/voice.wav"),
				ASRReferText: strPtr("你好"),
			},
			want: "msg\n[语音: https://example.com/voice.wav] (识别: 你好)",
		},
		{
			name: "voice with wav but no asr",
			att: types.MessageAttachment{
				ContentType: "voice",
				URL:          "https://example.com/voice.silk",
				VoiceWavURL:  strPtr("https://example.com/voice.wav"),
			},
			want: "msg\n[语音: https://example.com/voice.wav]",
		},
		{
			name: "voice no wav url",
			att: types.MessageAttachment{
				ContentType: "voice",
				URL:         "https://example.com/voice.silk",
			},
			want: "msg\n[语音: https://example.com/voice.silk]",
		},
		{
			name: "voice with empty wav url falls back to silk",
			att: types.MessageAttachment{
				ContentType: "voice",
				URL:         "https://example.com/voice.silk",
				VoiceWavURL: strPtr(""),
			},
			want: "msg\n[语音: https://example.com/voice.silk]",
		},
		{
			name: "voice with empty asr text",
			att: types.MessageAttachment{
				ContentType:  "voice",
				URL:          "https://example.com/voice.silk",
				VoiceWavURL:  strPtr("https://example.com/voice.wav"),
				ASRReferText: strPtr(""),
			},
			want: "msg\n[语音: https://example.com/voice.wav]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendAttachmentInfo("msg", []types.MessageAttachment{tt.att})
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppendAttachmentInfo_Video(t *testing.T) {
	got := appendAttachmentInfo("看看", []types.MessageAttachment{
		{ContentType: "video", URL: "https://example.com/vid.mp4"},
	})
	want := "看看\n[视频: https://example.com/vid.mp4]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAppendAttachmentInfo_File(t *testing.T) {
	tests := []struct {
		name string
		att  types.MessageAttachment
		want string
	}{
		{
			name: "file with filename",
			att: types.MessageAttachment{
				ContentType: "file",
				URL:         "https://example.com/doc.pdf",
				Filename:    strPtr("report.pdf"),
			},
			want: "msg\n[文件: report.pdf — https://example.com/doc.pdf]",
		},
		{
			name: "file without filename",
			att: types.MessageAttachment{
				ContentType: "file",
				URL:         "https://example.com/doc.pdf",
			},
			want: "msg\n[文件: 未知文件 — https://example.com/doc.pdf]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendAttachmentInfo("msg", []types.MessageAttachment{tt.att})
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppendAttachmentInfo_UnknownType(t *testing.T) {
	got := appendAttachmentInfo("msg", []types.MessageAttachment{
		{ContentType: "unknown_type", URL: "https://example.com/x"},
	})
	if got != "msg" {
		t.Errorf("unknown content_type should be ignored, got %q", got)
	}
}

func TestAppendAttachmentInfo_Multiple(t *testing.T) {
	got := appendAttachmentInfo("看看这个", []types.MessageAttachment{
		{ContentType: "image", URL: "https://example.com/img.png"},
		{ContentType: "voice", URL: "https://example.com/voice.silk",
			VoiceWavURL:  strPtr("https://example.com/voice.wav"),
			ASRReferText: strPtr("你好")},
		{ContentType: "video", URL: "https://example.com/vid.mp4"},
	})
	want := "看看这个\n[图片: https://example.com/img.png]\n[语音: https://example.com/voice.wav] (识别: 你好)\n[视频: https://example.com/vid.mp4]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
