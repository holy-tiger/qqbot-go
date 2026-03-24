package types

import "encoding/json"

// MediaFileType represents a media file type for upload.
type MediaFileType int

// MediaFileType constants
const (
	MediaFileTypeImage MediaFileType = 1
	MediaFileTypeVideo MediaFileType = 2
	MediaFileTypeVoice MediaFileType = 3
	MediaFileTypeFile  MediaFileType = 4
)

// Intent constants (bitmask values matching QQ Bot API)
const (
	IntentGuilds              = 1 << 0  // 1
	IntentGuildMembers        = 1 << 1  // 2
	IntentDirectMessage       = 1 << 12 // 4096
	IntentGroupAndC2C         = 1 << 30 // 1073741824
	IntentPublicGuildMessages = 1 << 25 // 33554432
)

// IntentLevel defines a level of intent permissions with fallback behavior.
type IntentLevel struct {
	Name     string
	Priority int
	Intents  int
}

// DefaultIntentLevels are the three fallback levels for intent negotiation.
var DefaultIntentLevels = []IntentLevel{
	{
		Name:     "full",
		Priority: 0,
		Intents: IntentGuilds | IntentGuildMembers | IntentDirectMessage |
			IntentGroupAndC2C | IntentPublicGuildMessages,
	},
	{
		Name:     "group_and_guild",
		Priority: 1,
		Intents: IntentGuilds | IntentPublicGuildMessages | IntentGroupAndC2C,
	},
	{
		Name:     "guild_only",
		Priority: 2,
		Intents: IntentGuilds | IntentPublicGuildMessages,
	},
}

// WSPayload represents a WebSocket event payload from the QQ Bot gateway.
type WSPayload struct {
	Op    int             `json:"op"`
	Data  json.RawMessage `json:"d,omitempty"`
	Seq   *int            `json:"s,omitempty"`
	Event *string         `json:"t,omitempty"`
}

// C2CAuthor represents the author of a C2C (private) message.
type C2CAuthor struct {
	ID          string `json:"id"`
	UnionOpenID string `json:"union_openid"`
	UserOpenID  string `json:"user_openid"`
}

// C2CMessageEvent represents a C2C (private) message event.
type C2CMessageEvent struct {
	Author       C2CAuthor           `json:"author"`
	Content      string              `json:"content"`
	ID           string              `json:"id"`
	Timestamp    string              `json:"timestamp"`
	MessageScene *MessageScene       `json:"message_scene,omitempty"`
	Attachments  []MessageAttachment `json:"attachments,omitempty"`
}

// MessageScene contains metadata about where a message was sent.
type MessageScene struct {
	Source string   `json:"source"`
	Ext    []string `json:"ext,omitempty"`
}

// MessageAttachment represents a rich media attachment in a message.
type MessageAttachment struct {
	ContentType  string `json:"content_type"`
	Filename     *string `json:"filename,omitempty"`
	Height       *int    `json:"height,omitempty"`
	Width        *int    `json:"width,omitempty"`
	Size         *int    `json:"size,omitempty"`
	URL          string  `json:"url"`
	VoiceWavURL  *string `json:"voice_wav_url,omitempty"`
	ASRReferText *string `json:"asr_refer_text,omitempty"`
}

// GuildAuthor represents the author of a guild message.
type GuildAuthor struct {
	ID       string  `json:"id"`
	Username *string `json:"username,omitempty"`
	Bot      *bool   `json:"bot,omitempty"`
}

// GuildMember represents guild member info.
type GuildMember struct {
	Nick     *string `json:"nick,omitempty"`
	JoinedAt *string `json:"joined_at,omitempty"`
}

// GuildMessageEvent represents a guild channel @mention message event.
type GuildMessageEvent struct {
	ID          string              `json:"id"`
	ChannelID   string              `json:"channel_id"`
	GuildID     string              `json:"guild_id"`
	Content     string              `json:"content"`
	Timestamp   string              `json:"timestamp"`
	Author      GuildAuthor         `json:"author"`
	Member      *GuildMember        `json:"member,omitempty"`
	Attachments []MessageAttachment `json:"attachments,omitempty"`
}

// GroupAuthor represents the author of a group message.
type GroupAuthor struct {
	ID           string `json:"id"`
	MemberOpenID string `json:"member_openid"`
}

// GroupMessageEvent represents a group @mention message event.
type GroupMessageEvent struct {
	Author       GroupAuthor         `json:"author"`
	Content      string              `json:"content"`
	ID           string              `json:"id"`
	Timestamp    string              `json:"timestamp"`
	GroupID      string              `json:"group_id"`
	GroupOpenID  string              `json:"group_openid"`
	MessageScene *MessageScene       `json:"message_scene,omitempty"`
	Attachments  []MessageAttachment `json:"attachments,omitempty"`
}

// AudioFormatPolicy controls which audio formats can skip conversion.
type AudioFormatPolicy struct {
	STTDirectFormats    []string `json:"stt_direct_formats,omitempty" yaml:"stt_direct_formats,omitempty"`
	UploadDirectFormats []string `json:"upload_direct_formats,omitempty" yaml:"upload_direct_formats,omitempty"`
}

// QQBotAccountConfig holds the configuration for a single QQ Bot account.
type QQBotAccountConfig struct {
	Enabled              *bool               `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Name                 *string             `json:"name,omitempty" yaml:"name,omitempty"`
	AppID                *string             `json:"appId,omitempty" yaml:"appId,omitempty"`
	ClientSecret         *string             `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	ClientSecretFile     *string             `json:"clientSecretFile,omitempty" yaml:"clientSecretFile,omitempty"`
	DMPolicy             *string             `json:"dmPolicy,omitempty" yaml:"dmPolicy,omitempty"`
	AllowFrom            []string            `json:"allowFrom,omitempty" yaml:"allowFrom,omitempty"`
	SystemPrompt         *string             `json:"systemPrompt,omitempty" yaml:"systemPrompt,omitempty"`
	ImageServerBaseUrl   *string             `json:"imageServerBaseUrl,omitempty" yaml:"imageServerBaseUrl,omitempty"`
	MarkdownSupport      *bool               `json:"markdownSupport,omitempty" yaml:"markdownSupport,omitempty"`
	VoiceDirectUploadFormats []string        `json:"voiceDirectUploadFormats,omitempty" yaml:"voiceDirectUploadFormats,omitempty"`
	AudioFormatPolicy    *AudioFormatPolicy  `json:"audioFormatPolicy,omitempty" yaml:"audioFormatPolicy,omitempty"`
}

// ResolvedQQBotAccount is a fully resolved account with all defaults applied.
type ResolvedQQBotAccount struct {
	AccountID         string              `json:"accountId"`
	Name              *string             `json:"name,omitempty"`
	Enabled           bool                `json:"enabled"`
	AppID             string              `json:"appId"`
	ClientSecret      string              `json:"clientSecret"`
	SecretSource      string              `json:"secretSource"`
	SystemPrompt      *string             `json:"systemPrompt,omitempty"`
	ImageServerBaseUrl *string            `json:"imageServerBaseUrl,omitempty"`
	MarkdownSupport   bool                `json:"markdownSupport"`
	Config            *QQBotAccountConfig `json:"config,omitempty"`
}

// MessageResponse is the API response when sending a message.
type MessageResponse struct {
	ID        string   `json:"id"`
	Timestamp string   `json:"timestamp"`
	ExtInfo   *ExtInfo `json:"ext_info,omitempty"`
}

// ExtInfo contains extension info returned with message responses.
type ExtInfo struct {
	RefIdx     string `json:"ref_idx,omitempty"`
	BusinessID int    `json:"business_id,omitempty"`
}

// UploadMediaResponse is the API response when uploading media.
type UploadMediaResponse struct {
	FileUUID string `json:"file_uuid"`
	FileInfo string `json:"file_info"`
	TTL      int    `json:"ttl"`
}


// OutboundMeta holds metadata for an outbound message.
type OutboundMeta struct {
	Text            string `json:"text,omitempty"`
	MediaType       string `json:"media_type,omitempty"`
	MediaURL        string `json:"media_url,omitempty"`
	MediaLocalPath  string `json:"media_local_path,omitempty"`
	TTSText         string `json:"tts_text,omitempty"`
	MessageID       string `json:"message_id,omitempty"`
	MessageScene    string `json:"message_scene,omitempty"`
}
