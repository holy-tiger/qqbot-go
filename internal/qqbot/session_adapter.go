package qqbot

import (
	"github.com/openclaw/qqbot/internal/gateway"
	"github.com/openclaw/qqbot/internal/store"
)

// gatewaySessionAdapter adapts store.SessionStore to implement gateway.SessionPersister.
type gatewaySessionAdapter struct {
	store *store.SessionStore
}

func newGatewaySessionAdapter(s *store.SessionStore) *gatewaySessionAdapter {
	return &gatewaySessionAdapter{store: s}
}

func (a *gatewaySessionAdapter) Load(accountID, expectedAppID string) *gateway.SessionData {
	state := a.store.Load(accountID, expectedAppID)
	if state == nil {
		return nil
	}
	return &gateway.SessionData{
		SessionID:        state.SessionID,
		LastSeq:          state.LastSeq,
		IntentLevelIndex: state.IntentLevelIndex,
		AccountID:        state.AccountID,
		AppID:            state.AppID,
	}
}

func (a *gatewaySessionAdapter) Save(data gateway.SessionData) {
	a.store.Save(store.SessionState{
		SessionID:        data.SessionID,
		LastSeq:          data.LastSeq,
		IntentLevelIndex: data.IntentLevelIndex,
		AccountID:        data.AccountID,
		AppID:            data.AppID,
	})
}

func (a *gatewaySessionAdapter) UpdateLastSeq(accountID string, lastSeq int) {
	a.store.UpdateLastSeq(accountID, lastSeq)
}
