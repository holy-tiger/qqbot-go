package httpapi

import (
	"encoding/json"
	"net/http"
)

// handleProactiveC2C handles POST /api/v1/accounts/{id}/proactive/c2c/{openid}
func (s *APIServer) handleProactiveC2C(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	openid := r.PathValue("openid")

	var req proactiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendProactiveC2C(r.Context(), id, openid, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}

// handleProactiveGroup handles POST /api/v1/accounts/{id}/proactive/groups/{openid}
func (s *APIServer) handleProactiveGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	openID := r.PathValue("openid")

	var req proactiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.manager.SendProactiveGroup(r.Context(), id, openID, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "sent"})
}

// handleBroadcast handles POST /api/v1/accounts/{id}/broadcast
func (s *APIServer) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proactiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sent, errs := s.manager.Broadcast(r.Context(), id, req.Content)
	errMsgs := make([]string, len(errs))
	for i, e := range errs {
		errMsgs[i] = e.Error()
	}
	writeOK(w, map[string]interface{}{
		"sent":  sent,
		"errors": errMsgs,
	})
}

// handleBroadcastGroups handles POST /api/v1/accounts/{id}/broadcast/groups
func (s *APIServer) handleBroadcastGroups(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proactiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sent, errs := s.manager.BroadcastToGroups(r.Context(), id, req.Content)
	errMsgs := make([]string, len(errs))
	for i, e := range errs {
		errMsgs[i] = e.Error()
	}
	writeOK(w, map[string]interface{}{
		"sent":   sent,
		"errors": errMsgs,
	})
}
