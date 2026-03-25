package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/openclaw/qqbot/internal/proactive"
)

// handleCreateReminder handles POST /api/v1/accounts/{id}/reminders
func (s *APIServer) handleCreateReminder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req reminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TargetType != "c2c" && req.TargetType != "group" {
		writeError(w, http.StatusBadRequest, "target_type must be 'c2c' or 'group'")
		return
	}
	if req.TargetAddress == "" {
		writeError(w, http.StatusBadRequest, "target_address is required")
		return
	}

	now := time.Now()
	nextRun := now
	if req.Schedule != "" {
		nextRun = proactive.CalculateNextRun(req.Schedule, now)
		if nextRun.IsZero() {
			nextRun = now
		}
	}

	job := proactive.ReminderJob{
		Content:       req.Content,
		TargetType:    req.TargetType,
		TargetAddress: req.TargetAddress,
		AccountID:     id,
		Schedule:      req.Schedule,
		NextRun:       nextRun,
		CreatedAt:     now,
	}

	jobID, err := s.manager.AddReminder(job)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{
		"job_id":    jobID,
		"next_run":  nextRun,
		"schedule":  req.Schedule,
	})
}

// handleCancelReminder handles DELETE /api/v1/accounts/{id}/reminders/{remID}
func (s *APIServer) handleCancelReminder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	remID := r.PathValue("remID")

	if s.manager.CancelReminder(id, remID) {
		writeOK(w, map[string]string{"status": "cancelled"})
	} else {
		writeError(w, http.StatusNotFound, "reminder not found")
	}
}

// handleListReminders handles GET /api/v1/accounts/{id}/reminders
func (s *APIServer) handleListReminders(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	reminders := s.manager.GetReminders(id)
	if reminders == nil {
		reminders = []proactive.ReminderJob{}
	}
	writeOK(w, reminders)
}
