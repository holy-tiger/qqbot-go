package httpapi

import (
	"net/http"
)

// handleListAccounts handles GET /api/v1/accounts
func (s *APIServer) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	statuses := s.manager.GetAllStatuses()
	result := make([]struct {
		ID        string `json:"id"`
		Connected bool   `json:"connected"`
	}, len(statuses))
	for i, st := range statuses {
		result[i] = struct {
			ID        string `json:"id"`
			Connected bool   `json:"connected"`
		}{ID: st.GetID(), Connected: st.IsConnected()}
	}
	writeOK(w, result)
}

// handleGetAccount handles GET /api/v1/accounts/{id}
func (s *APIServer) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	acct := s.manager.GetAccount(id)
	if acct == nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	writeOK(w, struct {
		ID        string `json:"id"`
		Connected bool   `json:"connected"`
	}{
		ID:        acct.GetID(),
		Connected: acct.IsConnected(),
	})
}
