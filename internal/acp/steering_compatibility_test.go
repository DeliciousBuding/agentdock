package acp

import (
	"context"
	"testing"
)

func TestClaudeSteeringCompatibilityVersionBoundary(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{name: "affected release", version: "0.64.2", want: true},
		{name: "affected older release", version: "v0.63.9", want: true},
		{name: "fixed boundary", version: "0.64.3", want: false},
		{name: "future release", version: "1.0.0", want: false},
		{name: "unknown version", version: "development", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := requiresHostSteeringFallback(AgentInfo{Name: claudeAgentACPName, Version: test.version})
			if got != test.want {
				t.Fatalf("fallback(%q) = %v, want %v", test.version, got, test.want)
			}
		})
	}
	if requiresHostSteeringFallback(AgentInfo{Name: "other", Version: "0.64.2"}) {
		t.Fatal("non-Claude adapter selected Claude compatibility fallback")
	}
}

func TestClaudeSteeringFallbackCancelsOldRunAndStartsObservableTurn(t *testing.T) {
	workspace := t.TempDir()
	manager, err := newTestManagerWithAgent(t.TempDir(), workspace, claudeAgentACPName, "0.64.2", "claude_steer_fallback")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = manager.Close() }()

	created, err := manager.NewSession(context.Background(), workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	original, err := manager.StartPrompt(context.Background(), created.Session.ID, "original")
	if err != nil {
		t.Fatal(err)
	}
	steering, err := manager.Steer(context.Background(), created.Session.ID, "STEERED")
	if err != nil {
		t.Fatal(err)
	}
	if steering["outcome"] != "startedNewTurn" || steering["reason"] != "claude_ede_compatibility" || steering["cancelledRunId"] != original.RunID {
		t.Fatalf("steering result = %#v", steering)
	}
	newRunID, _ := steering["runId"].(string)
	if newRunID == "" || newRunID == original.RunID {
		t.Fatalf("new steering run id = %q", newRunID)
	}
	cancelled := waitForSettledRun(t, manager, original.RunID)
	if cancelled.Status != RunCancelled || cancelled.Events[len(cancelled.Events)-1].Type != "cancelled" {
		t.Fatalf("original run = %#v", cancelled)
	}
	completed := waitForSettledRun(t, manager, newRunID)
	if completed.Status != RunCompleted || completed.StopReason != "end_turn" {
		t.Fatalf("fallback run = %#v", completed)
	}
	assertEventTypes(t, completed.Events, "agent_message_chunk", "completed")
}

func TestSteeringPromptRequiredStartsObservableRun(t *testing.T) {
	workspace := t.TempDir()
	manager, err := newTestManagerWithAgent(t.TempDir(), workspace, "generic-agent", "1.0.0", "steering_prompt_required")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = manager.Close() }()

	created, err := manager.NewSession(context.Background(), workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	steering, err := manager.Steer(context.Background(), created.Session.ID, "STEERED")
	if err != nil {
		t.Fatal(err)
	}
	if steering["outcome"] != "startedNewTurn" || steering["reason"] != "adapter_prompt_required" {
		t.Fatalf("steering result = %#v", steering)
	}
	runID, _ := steering["runId"].(string)
	completed := waitForSettledRun(t, manager, runID)
	if completed.Status != RunCompleted || completed.StopReason != "end_turn" {
		t.Fatalf("prompt-required run = %#v", completed)
	}
	assertEventTypes(t, completed.Events, "agent_message_chunk", "completed")
}

func TestClaudeSteeringResetFailureInterruptsSession(t *testing.T) {
	workspace := t.TempDir()
	manager, err := newTestManagerWithAgent(t.TempDir(), workspace, claudeAgentACPName, "0.64.2", "steering_reset_failure")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = manager.Close() }()

	created, err := manager.NewSession(context.Background(), workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	original, err := manager.StartPrompt(context.Background(), created.Session.ID, "original")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Steer(context.Background(), created.Session.ID, "replacement"); errorCode(err) != "ACP_REMOTE_ERROR" {
		t.Fatalf("steering reset error = %v", err)
	}
	cancelled := waitForSettledRun(t, manager, original.RunID)
	if cancelled.Status != RunCancelled {
		t.Fatalf("original run = %#v", cancelled)
	}
	record, err := manager.InspectSession(created.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != SessionInterrupted || record.LastStopReason != "steering_reset_failed" {
		t.Fatalf("session after reset failure = %#v", record)
	}
	manager.mu.RLock()
	_, loaded := manager.loaded[created.Session.ID]
	manager.mu.RUnlock()
	if loaded {
		t.Fatal("failed steering reset retained loaded-session cache")
	}
}
