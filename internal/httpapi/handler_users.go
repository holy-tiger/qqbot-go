package httpapi

import (
	"net/http"
	"strconv"

	"github.com/openclaw/qqbot/internal/store"
)

// userListRequest is the query parameters for listing users.
type userListRequest struct {
	Type         string `json:"type,omitempty"`
	ActiveWithin int64  `json:"active_within,omitempty"`
	Limit        int    `json:"limit,omitempty"`
	SortBy       string `json:"sort_by,omitempty"`
	SortOrder    string `json:"sort_order,omitempty"`
}

// handleListUsers handles GET /api/v1/accounts/{id}/users
func (s *APIServer) handleListUsers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	opts := store.ListOptions{
		Type:      r.URL.Query().Get("type"),
		SortBy:    r.URL.Query().Get("sort_by"),
		SortOrder: r.URL.Query().Get("sort_order"),
	}

	if v := r.URL.Query().Get("active_within"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			opts.ActiveWithin = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			opts.Limit = n
		}
	}

	users := s.manager.ListUsers(id, opts)
	if users == nil {
		users = []store.KnownUser{}
	}
	writeOK(w, users)
}

// handleUserStats handles GET /api/v1/accounts/{id}/users/stats
func (s *APIServer) handleUserStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stats := s.manager.GetUserStats(id)
	writeOK(w, stats)
}

// handleClearUsers handles DELETE /api/v1/accounts/{id}/users
func (s *APIServer) handleClearUsers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	n := s.manager.ClearUsers(id)
	writeOK(w, map[string]int{"removed": n})
}
