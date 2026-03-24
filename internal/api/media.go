package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/openclaw/qqbot/internal/types"
)

// UploadC2CMedia uploads media for C2C (private) messages.
// If fileData is provided, the upload cache is checked first.
func (c *APIClient) UploadC2CMedia(ctx context.Context, openid string, fileType types.MediaFileType, fileURL, fileData string, fileName string) (types.UploadMediaResponse, error) {
	if fileURL == "" && fileData == "" {
		return types.UploadMediaResponse{}, fmt.Errorf("uploadC2CMedia: fileURL or fileData is required")
	}

	// Check upload cache for fileData
	if fileData != "" {
		hash := c.uploadCache.ComputeFileHash(fileData)
		cached := c.uploadCache.Get(hash, "c2c", openid, fileType)
		if cached != "" {
			return types.UploadMediaResponse{
				FileInfo: cached,
			}, nil
		}
	}

	body := map[string]interface{}{
		"file_type":    int(fileType),
		"srv_send_msg": false,
	}
	if fileURL != "" {
		body["url"] = fileURL
	} else {
		body["file_data"] = fileData
	}
	if fileType == types.MediaFileTypeFile && fileName != "" {
		body["file_name"] = fileName
	}

	respBody, err := c.apiRequestWithRetry(ctx, "POST", fmt.Sprintf("/v2/users/%s/files", openid), body)
	if err != nil {
		return types.UploadMediaResponse{}, err
	}

	var result types.UploadMediaResponse
	if err := jsonUnmarshal(respBody, &result); err != nil {
		return types.UploadMediaResponse{}, fmt.Errorf("parse upload response: %w", err)
	}

	// Cache the result
	if fileData != "" && result.FileInfo != "" && result.TTL > 0 {
		hash := c.uploadCache.ComputeFileHash(fileData)
		c.uploadCache.Set(hash, "c2c", openid, fileType, result.FileInfo, result.FileUUID, result.TTL)
	}

	return result, nil
}

// UploadGroupMedia uploads media for group messages.
func (c *APIClient) UploadGroupMedia(ctx context.Context, groupOpenID string, fileType types.MediaFileType, fileURL, fileData string, fileName string) (types.UploadMediaResponse, error) {
	if fileURL == "" && fileData == "" {
		return types.UploadMediaResponse{}, fmt.Errorf("uploadGroupMedia: fileURL or fileData is required")
	}

	// Check upload cache for fileData
	if fileData != "" {
		hash := c.uploadCache.ComputeFileHash(fileData)
		cached := c.uploadCache.Get(hash, "group", groupOpenID, fileType)
		if cached != "" {
			return types.UploadMediaResponse{
				FileInfo: cached,
			}, nil
		}
	}

	body := map[string]interface{}{
		"file_type":    int(fileType),
		"srv_send_msg": false,
	}
	if fileURL != "" {
		body["url"] = fileURL
	} else {
		body["file_data"] = fileData
	}
	if fileType == types.MediaFileTypeFile && fileName != "" {
		body["file_name"] = fileName
	}

	respBody, err := c.apiRequestWithRetry(ctx, "POST", fmt.Sprintf("/v2/groups/%s/files", groupOpenID), body)
	if err != nil {
		return types.UploadMediaResponse{}, err
	}

	var result types.UploadMediaResponse
	if err := jsonUnmarshal(respBody, &result); err != nil {
		return types.UploadMediaResponse{}, fmt.Errorf("parse upload response: %w", err)
	}

	// Cache the result
	if fileData != "" && result.FileInfo != "" && result.TTL > 0 {
		hash := c.uploadCache.ComputeFileHash(fileData)
		c.uploadCache.Set(hash, "group", groupOpenID, fileType, result.FileInfo, result.FileUUID, result.TTL)
	}

	return result, nil
}

// sendC2CMediaMessage sends a media message (image, voice, video, file) to a C2C user.
func (c *APIClient) sendC2CMediaMessage(ctx context.Context, openid string, fileInfo string, msgID string, content string, meta types.OutboundMeta) (types.MessageResponse, error) {
	var msgSeq int
	if msgID != "" {
		msgSeq = getNextMsgSeq(msgID)
	} else {
		msgSeq = 1
	}

	body := map[string]interface{}{
		"msg_type": 7,
		"media":    map[string]interface{}{"file_info": fileInfo},
		"msg_seq":  msgSeq,
	}
	if msgID != "" {
		body["msg_id"] = msgID
	}
	if content != "" {
		body["content"] = content
	}

	resp, err := c.sendAndNotify(ctx, "POST", fmt.Sprintf("/v2/users/%s/messages", openid), body, meta)
	if err != nil {
		return types.MessageResponse{}, err
	}
	return *resp, nil
}

// sendGroupMediaMessage sends a media message to a group.
func (c *APIClient) sendGroupMediaMessage(ctx context.Context, groupOpenID string, fileInfo string, msgID string, content string) (types.MessageResponse, error) {
	var msgSeq int
	if msgID != "" {
		msgSeq = getNextMsgSeq(msgID)
	} else {
		msgSeq = 1
	}

	body := map[string]interface{}{
		"msg_type": 7,
		"media":    map[string]interface{}{"file_info": fileInfo},
		"msg_seq":  msgSeq,
	}
	if msgID != "" {
		body["msg_id"] = msgID
	}
	if content != "" {
		body["content"] = content
	}

	resp, err := c.sendAndNotify(ctx, "POST", fmt.Sprintf("/v2/groups/%s/messages", groupOpenID), body, types.OutboundMeta{Text: content})
	if err != nil {
		return types.MessageResponse{}, err
	}
	return *resp, nil
}

// SendC2CImageMessage uploads and sends an image to a C2C user.
func (c *APIClient) SendC2CImageMessage(ctx context.Context, openid, imageURL string, msgID string, content string) (types.MessageResponse, error) {
	var uploadResult types.UploadMediaResponse
	var isBase64 bool
	var err error

	if strings.HasPrefix(imageURL, "data:") {
		isBase64 = true
		// Parse data URL: data:<mime>;base64,<data>
		parts := strings.SplitN(imageURL, ",", 2)
		if len(parts) != 2 {
			return types.MessageResponse{}, fmt.Errorf("invalid base64 data URL format")
		}
		uploadResult, err = c.UploadC2CMedia(ctx, openid, types.MediaFileTypeImage, "", parts[1], "")
	} else {
		uploadResult, err = c.UploadC2CMedia(ctx, openid, types.MediaFileTypeImage, imageURL, "", "")
	}
	if err != nil {
		return types.MessageResponse{}, err
	}

	meta := types.OutboundMeta{
		Text:      content,
		MediaType: "image",
	}
	if !isBase64 {
		meta.MediaURL = imageURL
	}

	return c.sendC2CMediaMessage(ctx, openid, uploadResult.FileInfo, msgID, content, meta)
}

// SendGroupImageMessage uploads and sends an image to a group.
func (c *APIClient) SendGroupImageMessage(ctx context.Context, groupOpenID, imageURL string, msgID string, content string) (types.MessageResponse, error) {
	var uploadResult types.UploadMediaResponse
	var err error

	if strings.HasPrefix(imageURL, "data:") {
		parts := strings.SplitN(imageURL, ",", 2)
		if len(parts) != 2 {
			return types.MessageResponse{}, fmt.Errorf("invalid base64 data URL format")
		}
		uploadResult, err = c.UploadGroupMedia(ctx, groupOpenID, types.MediaFileTypeImage, "", parts[1], "")
	} else {
		uploadResult, err = c.UploadGroupMedia(ctx, groupOpenID, types.MediaFileTypeImage, imageURL, "", "")
	}
	if err != nil {
		return types.MessageResponse{}, err
	}

	return c.sendGroupMediaMessage(ctx, groupOpenID, uploadResult.FileInfo, msgID, content)
}

// SendC2CVoiceMessage uploads and sends a voice message to a C2C user.
func (c *APIClient) SendC2CVoiceMessage(ctx context.Context, openid, voiceBase64 string, msgID string, ttsText string) (types.MessageResponse, error) {
	uploadResult, err := c.UploadC2CMedia(ctx, openid, types.MediaFileTypeVoice, "", voiceBase64, "")
	if err != nil {
		return types.MessageResponse{}, err
	}

	meta := types.OutboundMeta{
		MediaType: "voice",
		TTSText:   ttsText,
	}

	return c.sendC2CMediaMessage(ctx, openid, uploadResult.FileInfo, msgID, "", meta)
}

// SendGroupVoiceMessage uploads and sends a voice message to a group.
func (c *APIClient) SendGroupVoiceMessage(ctx context.Context, groupOpenID, voiceBase64 string, msgID string) (types.MessageResponse, error) {
	uploadResult, err := c.UploadGroupMedia(ctx, groupOpenID, types.MediaFileTypeVoice, "", voiceBase64, "")
	if err != nil {
		return types.MessageResponse{}, err
	}

	return c.sendGroupMediaMessage(ctx, groupOpenID, uploadResult.FileInfo, msgID, "")
}

// SendC2CFileMessage uploads and sends a file to a C2C user.
func (c *APIClient) SendC2CFileMessage(ctx context.Context, openid string, fileBase64, fileURL string, msgID, fileName string) (types.MessageResponse, error) {
	uploadResult, err := c.UploadC2CMedia(ctx, openid, types.MediaFileTypeFile, fileURL, fileBase64, fileName)
	if err != nil {
		return types.MessageResponse{}, err
	}

	meta := types.OutboundMeta{
		MediaType:      "file",
		MediaURL:       fileURL,
		MediaLocalPath: fileName,
	}

	return c.sendC2CMediaMessage(ctx, openid, uploadResult.FileInfo, msgID, "", meta)
}

// SendGroupFileMessage uploads and sends a file to a group.
func (c *APIClient) SendGroupFileMessage(ctx context.Context, groupOpenID string, fileBase64, fileURL string, msgID, fileName string) (types.MessageResponse, error) {
	uploadResult, err := c.UploadGroupMedia(ctx, groupOpenID, types.MediaFileTypeFile, fileURL, fileBase64, fileName)
	if err != nil {
		return types.MessageResponse{}, err
	}

	return c.sendGroupMediaMessage(ctx, groupOpenID, uploadResult.FileInfo, msgID, "")
}

// SendC2CVideoMessage uploads and sends a video to a C2C user.
func (c *APIClient) SendC2CVideoMessage(ctx context.Context, openid string, videoURL, videoBase64 string, msgID, content string) (types.MessageResponse, error) {
	uploadResult, err := c.UploadC2CMedia(ctx, openid, types.MediaFileTypeVideo, videoURL, videoBase64, "")
	if err != nil {
		return types.MessageResponse{}, err
	}

	meta := types.OutboundMeta{
		Text:      content,
		MediaType: "video",
		MediaURL:  videoURL,
	}

	return c.sendC2CMediaMessage(ctx, openid, uploadResult.FileInfo, msgID, content, meta)
}

// SendGroupVideoMessage uploads and sends a video to a group.
func (c *APIClient) SendGroupVideoMessage(ctx context.Context, groupOpenID string, videoURL, videoBase64 string, msgID, content string) (types.MessageResponse, error) {
	uploadResult, err := c.UploadGroupMedia(ctx, groupOpenID, types.MediaFileTypeVideo, videoURL, videoBase64, "")
	if err != nil {
		return types.MessageResponse{}, err
	}

	return c.sendGroupMediaMessage(ctx, groupOpenID, uploadResult.FileInfo, msgID, content)
}
