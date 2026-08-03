package acp

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

func (m *Manager) ListInteractions(sessionID string, pendingOnly bool) []Interaction {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Interaction, 0)
	for _, interaction := range m.interactions {
		if sessionID != "" && interaction.SessionID != sessionID {
			continue
		}
		if pendingOnly && interaction.Status != InteractionPending {
			continue
		}
		result = append(result, interaction.public())
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}

func (m *Manager) InspectInteraction(id string) (Interaction, error) {
	m.mu.RLock()
	interaction := m.interactions[id]
	if interaction == nil {
		m.mu.RUnlock()
		return Interaction{}, newError("ACP_INTERACTION_NOT_FOUND", "ACP interaction was not found", false, map[string]any{"interaction_id": id}, nil)
	}
	result := interaction.public()
	m.mu.RUnlock()
	return result, nil
}

func (m *Manager) RespondInteraction(id, optionID string, cancelled bool) (Interaction, error) {
	m.mu.Lock()
	interaction := m.interactions[id]
	if interaction == nil {
		m.mu.Unlock()
		return Interaction{}, newError("ACP_INTERACTION_NOT_FOUND", "ACP interaction was not found", false, map[string]any{"interaction_id": id}, nil)
	}
	if interaction.Status != InteractionPending {
		result := interaction.public()
		m.mu.Unlock()
		return result, newError("ACP_INTERACTION_SETTLED", "ACP interaction is no longer pending", false, map[string]any{"interaction_id": id, "status": interaction.Status}, nil)
	}
	if !cancelled {
		valid := false
		for _, option := range interaction.Options {
			if option.OptionID == optionID {
				valid = true
				break
			}
		}
		if !valid {
			m.mu.Unlock()
			return Interaction{}, newError("ACP_PERMISSION_OPTION_INVALID", "permission option is not offered by the agent policy", false, map[string]any{"interaction_id": id, "option_id": optionID}, nil)
		}
	}
	response := interactionResponse{optionID: optionID, cancelled: cancelled}
	select {
	case interaction.respond <- response:
		if cancelled {
			interaction.Status = InteractionCancelled
		} else {
			interaction.Status = InteractionResponded
		}
		result := interaction.public()
		m.mu.Unlock()
		return result, nil
	default:
		result := interaction.public()
		m.mu.Unlock()
		return result, newError("ACP_INTERACTION_SETTLED", "ACP interaction response channel is already settled", false, map[string]any{"interaction_id": id}, nil)
	}
}

func (m *Manager) handleRequest(ctx context.Context, method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "session/request_permission":
		result, err := m.handlePermission(ctx, params)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}
		return result, nil
	default:
		return nil, &rpcError{Code: -32601, Message: "Method not found"}
	}
}

func (m *Manager) handlePermission(ctx context.Context, params json.RawMessage) (any, error) {
	var request struct {
		SessionID string             `json:"sessionId"`
		ToolCall  map[string]any     `json:"toolCall"`
		Options   []PermissionOption `json:"options"`
	}
	if err := json.Unmarshal(params, &request); err != nil {
		return nil, newError("ACP_PERMISSION_INVALID", "decode ACP permission request", false, nil, err)
	}
	m.mu.RLock()
	localID := m.remoteToLocal[request.SessionID]
	m.mu.RUnlock()
	if localID == "" {
		return cancelledPermissionOutcome(), nil
	}
	options := permittedPermissionOptions(request.Options)
	if len(options) == 0 {
		return cancelledPermissionOutcome(), nil
	}
	id, err := newID("acpi")
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	interaction := &Interaction{
		ID:        id,
		SessionID: localID,
		Kind:      "permission",
		Status:    InteractionPending,
		ToolCall:  request.ToolCall,
		Options:   options,
		CreatedAt: now,
		ExpiresAt: now.Add(m.opts.InteractionTimeout),
		respond:   make(chan interactionResponse, 1),
	}
	m.mu.Lock()
	m.interactions[id] = interaction
	runID := m.activeRunBySession[localID]
	run := m.runs[runID]
	m.pruneInteractionsLocked(now)
	publicInteraction := interaction.public()
	m.mu.Unlock()
	if run != nil {
		visible, _ := json.Marshal(publicInteraction)
		run.appendEvent(Event{Type: "permission_request", Update: visible})
	}

	timer := time.NewTimer(m.opts.InteractionTimeout)
	defer timer.Stop()
	select {
	case response := <-interaction.respond:
		return permissionResponseOutcome(response), nil
	case <-timer.C:
		if response, ok := pendingInteractionResponse(interaction); ok {
			return permissionResponseOutcome(response), nil
		}
		m.mu.Lock()
		status := interaction.Status
		if interaction.Status == InteractionPending {
			interaction.Status = InteractionExpired
		}
		m.mu.Unlock()
		if status == InteractionResponded || status == InteractionCancelled {
			if response, ok := pendingInteractionResponse(interaction); ok {
				return permissionResponseOutcome(response), nil
			}
		}
		return cancelledPermissionOutcome(), nil
	case <-ctx.Done():
		if response, ok := pendingInteractionResponse(interaction); ok {
			return permissionResponseOutcome(response), nil
		}
		m.mu.Lock()
		status := interaction.Status
		if interaction.Status == InteractionPending {
			interaction.Status = InteractionCancelled
		}
		m.mu.Unlock()
		if status == InteractionResponded || status == InteractionCancelled {
			if response, ok := pendingInteractionResponse(interaction); ok {
				return permissionResponseOutcome(response), nil
			}
		}
		return cancelledPermissionOutcome(), nil
	case <-m.closedCh:
		if response, ok := pendingInteractionResponse(interaction); ok {
			return permissionResponseOutcome(response), nil
		}
		return cancelledPermissionOutcome(), nil
	}
}

func permittedPermissionOptions(options []PermissionOption) []PermissionOption {
	capacity := len(options)
	if capacity > 32 {
		capacity = 32
	}
	result := make([]PermissionOption, 0, capacity)
	seen := make(map[string]struct{}, capacity)
	for _, option := range options {
		if strings.TrimSpace(option.OptionID) == "" {
			continue
		}
		if _, exists := seen[option.OptionID]; exists {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(option.Kind))
		if strings.Contains(kind, "always") {
			continue
		}
		seen[option.OptionID] = struct{}{}
		result = append(result, option)
		if len(result) == 32 {
			break
		}
	}
	return result
}

func cancelledPermissionOutcome() map[string]any {
	return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}
}

func permissionResponseOutcome(response interactionResponse) map[string]any {
	if response.cancelled {
		return cancelledPermissionOutcome()
	}
	return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": response.optionID}}
}

func pendingInteractionResponse(interaction *Interaction) (interactionResponse, bool) {
	select {
	case response := <-interaction.respond:
		return response, true
	default:
		return interactionResponse{}, false
	}
}

func (m *Manager) cancelPendingInteractions(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, interaction := range m.interactions {
		if interaction.SessionID != sessionID || interaction.Status != InteractionPending {
			continue
		}
		interaction.Status = InteractionCancelled
		select {
		case interaction.respond <- interactionResponse{cancelled: true}:
		default:
		}
	}
}

func (m *Manager) pruneInteractionsLocked(now time.Time) {
	if len(m.interactions) <= 256 {
		return
	}
	for id, interaction := range m.interactions {
		if interaction.Status == InteractionPending || now.Sub(interaction.CreatedAt) < time.Hour {
			continue
		}
		delete(m.interactions, id)
		if len(m.interactions) <= 192 {
			break
		}
	}
	if len(m.interactions) <= 256 {
		return
	}
	for id, interaction := range m.interactions {
		if interaction.Status == InteractionPending {
			continue
		}
		delete(m.interactions, id)
		if len(m.interactions) <= 192 {
			break
		}
	}
}
