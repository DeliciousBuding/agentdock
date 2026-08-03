package acp

import (
	"fmt"
	"testing"
	"time"
)

func TestSettleInteractionWaitPreservesReadyResponse(t *testing.T) {
	manager := &Manager{}
	interaction := &Interaction{
		Status:  InteractionResponded,
		respond: make(chan interactionResponse, 1),
	}
	interaction.respond <- interactionResponse{optionID: "allow-once"}

	response, ok := manager.settleInteractionWait(interaction, InteractionExpired)
	if !ok || response.optionID != "allow-once" || response.cancelled {
		t.Fatalf("settled response = %#v, ok=%v", response, ok)
	}
	if interaction.Status != InteractionResponded {
		t.Fatalf("interaction status = %s", interaction.Status)
	}
}

func TestSettleInteractionWaitAdvancesPendingState(t *testing.T) {
	manager := &Manager{}
	interaction := &Interaction{
		Status:  InteractionPending,
		respond: make(chan interactionResponse, 1),
	}
	if _, ok := manager.settleInteractionWait(interaction, InteractionExpired); ok {
		t.Fatal("unexpected interaction response")
	}
	if interaction.Status != InteractionExpired {
		t.Fatalf("interaction status = %s", interaction.Status)
	}
}

func TestPruneInteractionsRemovesOldestSettledFirst(t *testing.T) {
	now := time.Now().UTC()
	manager := &Manager{interactions: make(map[string]*Interaction)}
	for index := 0; index < 300; index++ {
		status := InteractionResponded
		if index >= 290 {
			status = InteractionPending
		}
		id := fmt.Sprintf("acpi_%03d", index)
		manager.interactions[id] = &Interaction{
			ID: id, Status: status, CreatedAt: now.Add(time.Duration(index-400) * time.Hour),
		}
	}

	manager.pruneInteractionsLocked(now)
	if len(manager.interactions) != targetRetainedInteractions {
		t.Fatalf("retained interactions = %d", len(manager.interactions))
	}
	boundary := 300 - targetRetainedInteractions
	for index := 0; index < boundary; index++ {
		id := fmt.Sprintf("acpi_%03d", index)
		if _, exists := manager.interactions[id]; exists {
			t.Fatalf("old interaction %s was retained", id)
		}
	}
	for index := boundary; index < 300; index++ {
		id := fmt.Sprintf("acpi_%03d", index)
		if _, exists := manager.interactions[id]; !exists {
			t.Fatalf("newer or pending interaction %s was removed", id)
		}
	}
}
