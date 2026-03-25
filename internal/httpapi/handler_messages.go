package httpapi

import (
	"encoding/json"
	"net/http"
)

// handleSendC2CText handles POST /api/v1/accounts/{id}/c2c/{openid}/messages
func (s *APIServer) handleSendC2CText(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	openid := r.PathValue("openid")

	var req textRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendC2C(r.Context(), id, openid, req.Content, req.MsgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}

// handleSendGroupText handles POST /api/v1/accounts/{id}/groups/{openid}/messages
func (s *APIServer) handleSendGroupText(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	openID := r.PathValue("openid")

	var req textRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendGroup(r.Context(), id, openID, req.Content, req.MsgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}

// handleSendChannelText handles POST /api/v1/accounts/{id}/channels/{channelID}/messages
func (s *APIServer) handleSendChannelText(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	channelID := r.PathValue("channelID")

	var req textRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendChannel(r.Context(), id, channelID, req.Content, req.MsgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}

// handleSendC2CImage handles POST /api/v1/accounts/{id}/c2c/{openid}/images
func (s *APIServer) handleSendC2CImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	openid := r.PathValue("openid")

	var req imageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendImage(r.Context(), id, "c2c", openid, req.ImageURL, req.Content, req.MsgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}

// handleSendGroupImage handles POST /api/v1/accounts/{id}/groups/{openid}/images
func (s *APIServer) handleSendGroupImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	openID := r.PathValue("openid")

	var req imageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendImage(r.Context(), id, "group", openID, req.ImageURL, req.Content, req.MsgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}

// handleSendC2CVoice handles POST /api/v1/accounts/{id}/c2c/{openid}/voice
func (s *APIServer) handleSendC2CVoice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	openid := r.PathValue("openid")

	var req voiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendVoice(r.Context(), id, "c2c", openid, req.VoiceBase64, req.TTSText, req.MsgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}

// handleSendGroupVoice handles POST /api/v1/accounts/{id}/groups/{openid}/voice
func (s *APIServer) handleSendGroupVoice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	openID := r.PathValue("openid")

	var req voiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendVoice(r.Context(), id, "group", openID, req.VoiceBase64, "", req.MsgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}

// handleSendC2CVideo handles POST /api/v1/accounts/{id}/c2c/{openid}/videos
func (s *APIServer) handleSendC2CVideo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	openid := r.PathValue("openid")

	var req videoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendVideo(r.Context(), id, "c2c", openid, req.VideoURL, req.VideoBase64, req.Content, req.MsgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}

// handleSendGroupVideo handles POST /api/v1/accounts/{id}/groups/{openid}/videos
func (s *APIServer) handleSendGroupVideo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	openID := r.PathValue("openid")

	var req videoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendVideo(r.Context(), id, "group", openID, req.VideoURL, req.VideoBase64, req.Content, req.MsgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}

// handleSendC2CFile handles POST /api/v1/accounts/{id}/c2c/{openid}/files
func (s *APIServer) handleSendC2CFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	openid := r.PathValue("openid")

	var req fileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendFile(r.Context(), id, "c2c", openid, req.FileBase64, req.FileURL, req.FileName, req.MsgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}

// handleSendGroupFile handles POST /api/v1/accounts/{id}/groups/{openid}/files
func (s *APIServer) handleSendGroupFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	openID := r.PathValue("openid")

	var req fileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendFile(r.Context(), id, "group", openID, req.FileBase64, req.FileURL, req.FileName, req.MsgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}
