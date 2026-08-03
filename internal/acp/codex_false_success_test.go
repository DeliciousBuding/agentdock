package acp

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCodexFalseSuccessIsPromotedToFailure(t *testing.T) {
	manager, err := newTestManagerWithPromptMode(t.TempDir(), t.TempDir(), "@agentclientprotocol/codex-acp", "codex_false_success")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = manager.Close() }()

	created, err := manager.NewSession(context.Background(), manager.opts.Agent.AllowedRoots[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	started, err := manager.StartPrompt(context.Background(), created.Session.ID, "trigger remote failure")
	if err != nil {
		t.Fatal(err)
	}
	result := waitForSettledRun(t, manager, started.RunID)
	if result.Status != RunFailed || result.ErrorCode != "ACP_REMOTE_ERROR" {
		t.Fatalf("run result = status %s code %q message %q", result.Status, result.ErrorCode, result.Message)
	}
	assertEventTypes(t, result.Events, "session_info_update", "agent_message_chunk", "error")
	if result.Events[len(result.Events)-1].Type != "error" {
		t.Fatalf("last event = %#v", result.Events[len(result.Events)-1])
	}
	inspected, err := manager.InspectSession(created.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Status != SessionError || inspected.LastStopReason != "ACP_REMOTE_ERROR" {
		t.Fatalf("session after false success = %#v", inspected)
	}
}

func TestCodexRetryThenRecoveryRemainsCompleted(t *testing.T) {
	manager, err := newTestManagerWithPromptMode(t.TempDir(), t.TempDir(), "@agentclientprotocol/codex-acp", "codex_recovered")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = manager.Close() }()

	created, err := manager.NewSession(context.Background(), manager.opts.Agent.AllowedRoots[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	started, err := manager.StartPrompt(context.Background(), created.Session.ID, "recover after retry")
	if err != nil {
		t.Fatal(err)
	}
	result := waitForSettledRun(t, manager, started.RunID)
	if result.Status != RunCompleted || result.ErrorCode != "" || result.StopReason != "end_turn" {
		t.Fatalf("recovered run = %#v", result)
	}
	assertEventTypes(t, result.Events, "session_info_update", "agent_message_chunk", "completed")
	if result.Events[len(result.Events)-1].Type != "completed" {
		t.Fatalf("last event = %#v", result.Events[len(result.Events)-1])
	}
}

func TestNonCodexAdapterDoesNotApplyCodexCompensation(t *testing.T) {
	manager, err := newTestManagerWithPromptMode(t.TempDir(), t.TempDir(), "different-adapter", "codex_false_success")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = manager.Close() }()

	created, err := manager.NewSession(context.Background(), manager.opts.Agent.AllowedRoots[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	started, err := manager.StartPrompt(context.Background(), created.Session.ID, "adapter-specific behavior")
	if err != nil {
		t.Fatal(err)
	}
	result := waitForSettledRun(t, manager, started.RunID)
	if result.Status != RunCompleted || result.StopReason != "end_turn" {
		t.Fatalf("non-Codex run = %#v", result)
	}
}

func waitForSettledRun(t *testing.T, manager *Manager, runID string) PromptEventsResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	after := uint64(0)
	allEvents := make([]Event, 0, 16)
	for time.Now().Before(deadline) {
		result, err := manager.PromptEvents(context.Background(), runID, after, 200, 250*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		allEvents = append(allEvents, result.Events...)
		after = result.NextSeq
		if result.Status != RunRunning && !result.HasMore {
			result.Events = allEvents
			return result
		}
	}
	t.Fatalf("run %s did not settle", runID)
	return PromptEventsResult{}
}

func TestCodexRemoteErrorTrackingIsBounded(t *testing.T) {
	run := newRun("acpr_bounded", "acps_bounded")
	for index := 0; index < maxTrackedRemoteErrors+10; index++ {
		update := testMarshalRaw(map[string]any{
			"sessionUpdate": "session_info_update",
			"_meta": map[string]any{"codex": map[string]any{"error": map[string]any{
				"message": "retry", "additionalDetails": "error-" + strconv.Itoa(index), "willRetry": true,
			}}},
		})
		run.noteCodexUpdateLocked("session_info_update", update)
	}
	if len(run.remoteErrorCandidates) != maxTrackedRemoteErrors {
		t.Fatalf("tracked remote errors = %d", len(run.remoteErrorCandidates))
	}
	oversized := strings.Repeat("x", maxTrackedRemoteErrorBytes+1)
	run.noteCodexUpdateLocked("session_info_update", testMarshalRaw(map[string]any{
		"sessionUpdate": "session_info_update",
		"_meta": map[string]any{"codex": map[string]any{"error": map[string]any{
			"message": oversized, "additionalDetails": oversized, "willRetry": true,
		}}},
	}))
	if len(run.remoteErrorCandidates) != maxTrackedRemoteErrors {
		t.Fatalf("oversized remote error changed candidate count: %d", len(run.remoteErrorCandidates))
	}
	run.noteCodexUpdateLocked("agent_message_chunk", testMarshalRaw(map[string]any{
		"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": oversized},
	}))
	if run.lastAssistantChunk != "" {
		t.Fatalf("oversized assistant chunk was retained: %d bytes", len(run.lastAssistantChunk))
	}
}
