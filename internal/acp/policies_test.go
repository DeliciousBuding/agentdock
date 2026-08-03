package acp

import (
	"encoding/json"
	"testing"
)

func TestCurrentPoliciesMatchRuntimeBoundsAndJSONContract(t *testing.T) {
	policies := CurrentPolicies()
	if policies.Events.MaxRetainedEvents != maxEventCount ||
		policies.Events.MaxRetainedBytes != maxEventBytes ||
		policies.Events.MaxUpdateBytes != maxEventUpdateBytes ||
		policies.Events.MaxEventsPerPage != 200 {
		t.Fatalf("event policy = %#v", policies.Events)
	}
	if policies.Interactions.MaxOptions != maxPermissionOptions ||
		policies.Interactions.MaxToolCallBytes != maxPermissionToolCallBytes ||
		policies.Interactions.MaxPendingGlobal != maxPendingInteractions ||
		policies.Interactions.MaxPendingPerSession != maxPendingInteractionsPerSession {
		t.Fatalf("interaction policy = %#v", policies.Interactions)
	}
	encoded, err := json.Marshal(policies)
	if err != nil {
		t.Fatal(err)
	}
	var public map[string]any
	if err := json.Unmarshal(encoded, &public); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"context_policy", "event_policy", "interaction_policy", "steering_policy"} {
		if _, exists := public[key]; !exists {
			t.Fatalf("serialized policies missing %s: %s", key, encoded)
		}
	}
}
