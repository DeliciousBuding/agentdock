package acp

import "encoding/json"

func (m *Manager) handleNotification(method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	var notification struct {
		SessionID string          `json:"sessionId"`
		Update    json.RawMessage `json:"update"`
	}
	if err := json.Unmarshal(params, &notification); err != nil || notification.SessionID == "" {
		return
	}
	m.mu.RLock()
	localID := m.remoteToLocal[notification.SessionID]
	if localID == "" {
		m.mu.RUnlock()
		return
	}
	runID := m.activeRunBySession[localID]
	run := m.runs[runID]
	m.mu.RUnlock()
	if run == nil {
		return
	}
	eventType := "session_update"
	var updateType struct {
		SessionUpdate string `json:"sessionUpdate"`
	}
	if json.Unmarshal(notification.Update, &updateType) == nil && updateType.SessionUpdate != "" {
		eventType = updateType.SessionUpdate
	}
	run.appendSessionUpdate(Event{Type: eventType, Update: append(json.RawMessage(nil), notification.Update...)})
}
