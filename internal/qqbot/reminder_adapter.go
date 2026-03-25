package qqbot

import (
	"github.com/openclaw/qqbot/internal/proactive"
	"github.com/openclaw/qqbot/internal/store"
)

// reminderStoreAdapter adapts store.ReminderStore to implement proactive.ReminderPersister.
type reminderStoreAdapter struct {
	store *store.ReminderStore
}

func newReminderStoreAdapter(s *store.ReminderStore) *reminderStoreAdapter {
	return &reminderStoreAdapter{store: s}
}

func (a *reminderStoreAdapter) Save(job proactive.ReminderJob) {
	a.store.Save(store.ReminderRecord{
		ID:            job.ID,
		Content:       job.Content,
		TargetType:    job.TargetType,
		TargetAddress: job.TargetAddress,
		AccountID:     job.AccountID,
		Schedule:      job.Schedule,
		NextRun:       job.NextRun,
		CreatedAt:     job.CreatedAt,
	})
}

func (a *reminderStoreAdapter) Get(id string) *proactive.ReminderJob {
	r := a.store.Get(id)
	if r == nil {
		return nil
	}
	return &proactive.ReminderJob{
		ID:            r.ID,
		Content:       r.Content,
		TargetType:    r.TargetType,
		TargetAddress: r.TargetAddress,
		AccountID:     r.AccountID,
		Schedule:      r.Schedule,
		NextRun:       r.NextRun,
		CreatedAt:     r.CreatedAt,
	}
}

func (a *reminderStoreAdapter) Delete(id string) bool {
	return a.store.Delete(id)
}

func (a *reminderStoreAdapter) ListByAccount(accountID string) []proactive.ReminderJob {
	records := a.store.ListByAccount(accountID)
	result := make([]proactive.ReminderJob, len(records))
	for i, r := range records {
		result[i] = proactive.ReminderJob{
			ID:            r.ID,
			Content:       r.Content,
			TargetType:    r.TargetType,
			TargetAddress: r.TargetAddress,
			AccountID:     r.AccountID,
			Schedule:      r.Schedule,
			NextRun:       r.NextRun,
			CreatedAt:     r.CreatedAt,
		}
	}
	return result
}

func (a *reminderStoreAdapter) ListAll() []proactive.ReminderJob {
	records := a.store.ListAll()
	result := make([]proactive.ReminderJob, len(records))
	for i, r := range records {
		result[i] = proactive.ReminderJob{
			ID:            r.ID,
			Content:       r.Content,
			TargetType:    r.TargetType,
			TargetAddress: r.TargetAddress,
			AccountID:     r.AccountID,
			Schedule:      r.Schedule,
			NextRun:       r.NextRun,
			CreatedAt:     r.CreatedAt,
		}
	}
	return result
}
