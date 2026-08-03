package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestManagerPromptEventsPermissionAndPersistence(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	manager, err := newTestManager(home, workspace)
	if err != nil {
		t.Fatal(err)
	}

	created, err := manager.NewSession(context.Background(), workspace, nil)
	if err != nil {
		manager.Close()
		t.Fatal(err)
	}
	if created.Session.ID == "" || created.Session.RemoteSessionID != "remote-1" {
		manager.Close()
		t.Fatalf("unexpected session: %#v", created)
	}

	started, err := manager.StartPrompt(context.Background(), created.Session.ID, "exercise permission")
	if err != nil {
		manager.Close()
		t.Fatal(err)
	}
	if started.Status != RunRunning {
		manager.Close()
		t.Fatalf("start status = %s", started.Status)
	}

	var allEvents []Event
	var permission Interaction
	after := uint64(0)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		events, err := manager.PromptEvents(context.Background(), started.RunID, after, 100, 250*time.Millisecond)
		if err != nil {
			manager.Close()
			t.Fatal(err)
		}
		allEvents = append(allEvents, events.Events...)
		if len(events.Events) > 0 {
			after = events.Events[len(events.Events)-1].Seq
		}
		interactions := manager.ListInteractions(created.Session.ID, true)
		if len(interactions) > 0 {
			permission = interactions[0]
			break
		}
	}
	if permission.ID == "" {
		manager.Close()
		t.Fatalf("permission interaction was not emitted; events=%#v", allEvents)
	}
	if len(permission.Options) != 1 || permission.Options[0].OptionID != "allow-once" {
		manager.Close()
		t.Fatalf("policy did not filter always option: %#v", permission.Options)
	}
	if _, err := manager.RespondInteraction(permission.ID, "allow-always", false); err == nil {
		manager.Close()
		t.Fatal("always option was accepted")
	}
	if _, err := manager.RespondInteraction(permission.ID, "allow-once", false); err != nil {
		manager.Close()
		t.Fatal(err)
	}

	completed := false
	for time.Now().Before(deadline) {
		events, err := manager.PromptEvents(context.Background(), started.RunID, after, 100, 250*time.Millisecond)
		if err != nil {
			manager.Close()
			t.Fatal(err)
		}
		allEvents = append(allEvents, events.Events...)
		if len(events.Events) > 0 {
			after = events.Events[len(events.Events)-1].Seq
		}
		if events.Status == RunCompleted {
			completed = true
			break
		}
	}
	if !completed {
		manager.Close()
		t.Fatalf("prompt did not complete; events=%#v", allEvents)
	}
	assertEventTypes(t, allEvents, "agent_message_chunk", "permission_request", "completed")

	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := newTestManager(home, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	sessions, err := reloaded.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].ID != created.Session.ID {
		t.Fatalf("persisted sessions = %#v", sessions)
	}
	loaded, err := reloaded.LoadSession(context.Background(), created.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Session.Status != SessionReady {
		t.Fatalf("loaded status = %s", loaded.Session.Status)
	}
}

func TestManagerSessionLifecycleCapabilities(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	additional := filepath.Join(workspace, "secondary")
	if err := os.MkdirAll(additional, 0o700); err != nil {
		t.Fatal(err)
	}
	manager, err := newTestManager(home, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	if err := manager.Authenticate(context.Background(), "missing-auth"); err == nil {
		t.Fatal("unadvertised authentication method was accepted")
	}
	if err := manager.Authenticate(context.Background(), "test-auth"); err != nil {
		t.Fatal(err)
	}
	created, err := manager.NewSession(context.Background(), workspace, []string{additional})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Session.AdditionalDirectories) != 1 || created.Session.AdditionalDirectories[0] != additional {
		t.Fatalf("additional directories = %#v", created.Session.AdditionalDirectories)
	}
	if err := manager.SetSessionMode(context.Background(), created.Session.ID, "code"); err != nil {
		t.Fatal(err)
	}
	options, err := manager.SetSessionConfigOption(context.Background(), created.Session.ID, "safe", false)
	if err != nil {
		t.Fatal(err)
	}
	if options == nil {
		t.Fatal("set_config_option omitted config options")
	}
	steering, err := manager.Steer(context.Background(), created.Session.ID, "adjust")
	if err != nil {
		t.Fatal(err)
	}
	if steering["outcome"] != "injected" {
		t.Fatalf("steering outcome = %#v", steering)
	}
	forked, err := manager.ForkSession(context.Background(), created.Session.ID, "", []string{})
	if err != nil {
		t.Fatal(err)
	}
	if forked.Session.RemoteSessionID == created.Session.RemoteSessionID || len(forked.Session.AdditionalDirectories) != 0 {
		t.Fatalf("forked session = %#v", forked.Session)
	}
	resumed, err := manager.ResumeSession(context.Background(), created.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Session.Status != SessionReady {
		t.Fatalf("resumed status = %s", resumed.Session.Status)
	}
	closed, err := manager.CloseSession(context.Background(), forked.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != SessionClosed {
		t.Fatalf("closed status = %s", closed.Status)
	}
	if err := manager.DeleteSession(context.Background(), forked.Session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.InspectSession(forked.Session.ID); err == nil {
		t.Fatal("deleted session remained inspectable")
	}
}

func TestCapabilityInspectionUsesInitializeContract(t *testing.T) {
	process := &agentProcess{initialize: InitializeResult{
		AgentCapabilities: map[string]any{
			"loadSession":         true,
			"sessionCapabilities": map[string]any{"resume": map[string]any{}},
		},
		Meta: map[string]any{"steering": map[string]any{"supported": true}},
	}}
	if !process.supportsLoadSession() || !process.supportsSessionCapability("resume") || process.supportsSessionCapability("fork") || !process.supportsSteering() {
		t.Fatalf("capability inspection failed: %#v", process.initialize)
	}
}

func TestManagerRecoveryMarksRunningSessionInterrupted(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	store, err := newSessionStore(home)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	record := SessionRecord{
		ID: "acps_recovery", Agent: "helper", RemoteSessionID: "remote-recovery", CWD: workspace,
		Status: SessionRunning, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.Save(record); err != nil {
		t.Fatal(err)
	}
	manager, err := newTestManager(home, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	loaded, err := manager.InspectSession(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != SessionInterrupted || loaded.LastStopReason != "agentdock_restart" {
		t.Fatalf("recovered session = %#v", loaded)
	}
}

func TestManagerCloseMarksActivePromptInterrupted(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	manager, err := newTestManager(home, workspace)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.NewSession(context.Background(), workspace, nil)
	if err != nil {
		manager.Close()
		t.Fatal(err)
	}
	started, err := manager.StartPrompt(context.Background(), created.Session.ID, "block for shutdown")
	if err != nil {
		manager.Close()
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(manager.ListInteractions(created.Session.ID, true)) > 0 {
			break
		}
		if _, err := manager.PromptEvents(context.Background(), started.RunID, 0, 10, 50*time.Millisecond); err != nil {
			manager.Close()
			t.Fatal(err)
		}
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := newTestManager(home, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	record, err := reloaded.InspectSession(created.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != SessionInterrupted || record.LastStopReason != "agentdock_shutdown" {
		t.Fatalf("shutdown record = %#v", record)
	}
}

func TestResolveCWDEnforcesAllowedRootsAndSymlinks(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside")
	if err := os.MkdirAll(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	manager, err := newTestManager(t.TempDir(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	resolved, err := manager.resolveCWD("inside")
	if err != nil || resolved != inside {
		t.Fatalf("inside resolve = %q, %v", resolved, err)
	}
	if _, err := manager.resolveCWD(outside); err == nil {
		t.Fatal("outside cwd was accepted")
	}
	link := filepath.Join(root, "escape-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Logf("symlink test skipped on this host: %v", err)
		return
	}
	if _, err := manager.resolveCWD(link); err == nil {
		t.Fatal("symlink escape was accepted")
	}
}

func TestConnectionCancelsInboundRequest(t *testing.T) {
	agentReader, agentWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	clientReader, clientWriter, err := os.Pipe()
	if err != nil {
		agentReader.Close()
		agentWriter.Close()
		t.Fatal(err)
	}
	defer agentWriter.Close()
	defer clientReader.Close()

	started := make(chan struct{})
	cancelled := make(chan struct{})
	connection := NewConnection(agentReader, clientWriter, func(ctx context.Context, _ string, _ json.RawMessage) (any, *rpcError) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return cancelledPermissionOutcome(), nil
	}, nil)
	defer connection.Close()

	encoder := json.NewEncoder(agentWriter)
	if err := encoder.Encode(rpcMessage{JSONRPC: "2.0", ID: json.RawMessage("7"), Method: "session/request_permission", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("inbound request did not start")
	}
	if err := encoder.Encode(rpcMessage{JSONRPC: "2.0", Method: "$/cancel_request", Params: json.RawMessage(`{"requestId":7}`)}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("inbound request was not cancelled")
	}
}

func TestCancelThenDeleteDoesNotRecreateSessionState(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	manager, err := newTestManager(home, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	created, err := manager.NewSession(context.Background(), workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := manager.NewSession(context.Background(), workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	started, err := manager.StartPrompt(context.Background(), created.Session.ID, "wait for permission")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(manager.ListInteractions(created.Session.ID, true)) > 0 {
			break
		}
		_, _ = manager.PromptEvents(context.Background(), started.RunID, 0, 10, 50*time.Millisecond)
	}
	if err := manager.CancelPrompt(context.Background(), other.Session.ID, started.RunID); err == nil {
		t.Fatal("cancel accepted a run from another session")
	}
	if err := manager.CancelPrompt(context.Background(), created.Session.ID, started.RunID); err != nil {
		t.Fatal(err)
	}
	events, err := manager.PromptEvents(context.Background(), started.RunID, 0, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if events.Status != RunCancelled {
		t.Fatalf("cancelled run status = %s", events.Status)
	}

	manager.mu.Lock()
	if manager.process == nil {
		manager.mu.Unlock()
		t.Fatal("helper process is not initialized")
	}
	sessionCapabilities := manager.process.initialize.AgentCapabilities["sessionCapabilities"].(map[string]any)
	delete(sessionCapabilities, "delete")
	manager.mu.Unlock()
	if err := manager.DeleteSession(context.Background(), created.Session.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := manager.store.Get(created.Session.ID); err == nil {
		t.Fatal("cancelled prompt recreated deleted session state")
	}
}

func TestRunEventRingReportsTruncation(t *testing.T) {
	run := newRun("acpr_ring", "acps_ring")
	for index := 0; index < maxEventCount+10; index++ {
		run.appendEvent(Event{Type: "chunk", Message: strconv.Itoa(index)})
	}
	events, next, first, truncated := run.eventsAfter(0, 200)
	if !truncated || first <= 1 || next != first+199 || len(events) != 200 {
		t.Fatalf("event ring = events %d next %d first %d truncated %v", len(events), next, first, truncated)
	}
	second, secondNext, _, secondTruncated := run.eventsAfter(next, 200)
	if secondTruncated || len(second) != 200 || second[0].Seq != next+1 || secondNext != second[len(second)-1].Seq {
		t.Fatalf("second event page = events %d next %d truncated %v", len(second), secondNext, secondTruncated)
	}
}

func TestRedactEnvironmentValues(t *testing.T) {
	redacted := redactEnvironmentValues("failed with secret-token and abc", map[string]string{"TOKEN": "secret-token", "SHORT": "abc"})
	if strings.Contains(redacted, "secret-token") || !strings.Contains(redacted, "[REDACTED]") || !strings.Contains(redacted, "abc") {
		t.Fatalf("redacted text = %q", redacted)
	}
}

func TestConnectionRejectsUnencodablePayloadWithoutPanic(t *testing.T) {
	reader, writer, cleanup := pipeConnectionPair(t)
	defer cleanup()
	connection := NewConnection(reader, writer, nil, nil)
	defer connection.Close()
	err := connection.Notify("test", map[string]any{"value": math.NaN()})
	var acpErr *Error
	if !errors.As(err, &acpErr) || acpErr.Code != "ACP_PROTOCOL_ERROR" {
		t.Fatalf("unencodable payload error = %#v", err)
	}
}

func TestConnectionRejectsOversizedMessage(t *testing.T) {
	reader, writer, cleanup := pipeConnectionPair(t)
	defer cleanup()
	connection := NewConnection(reader, writer, nil, nil)
	defer connection.Close()
	err := connection.Notify("test", map[string]any{"value": strings.Repeat("x", maxRPCLineBytes)})
	var acpErr *Error
	if !errors.As(err, &acpErr) || acpErr.Code != "ACP_MESSAGE_TOO_LARGE" {
		t.Fatalf("oversized error = %#v", err)
	}
}

func newTestManager(home, workspace string) (*Manager, error) {
	return NewManager(Options{
		Home: home,
		Agent: AgentSpec{
			Name: "helper", Command: os.Args[0], Args: []string{"-test.run=TestACPHelperProcess"},
			AllowedRoots: []string{workspace}, Environment: map[string]string{"GO_WANT_ACP_HELPER": "1"},
		},
		MaxConcurrentRuns: 2, InteractionTimeout: 3 * time.Second,
	})
}

func assertEventTypes(t *testing.T, events []Event, expected ...string) {
	t.Helper()
	seen := map[string]bool{}
	for _, event := range events {
		seen[event.Type] = true
	}
	for _, eventType := range expected {
		if !seen[eventType] {
			t.Fatalf("event type %q missing from %#v", eventType, events)
		}
	}
}

func pipeConnectionPair(t *testing.T) (*os.File, *os.File, func()) {
	t.Helper()
	reader, peerWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	peerReader, writer, err := os.Pipe()
	if err != nil {
		reader.Close()
		peerWriter.Close()
		t.Fatal(err)
	}
	cleanup := func() {
		reader.Close()
		writer.Close()
		peerReader.Close()
		peerWriter.Close()
	}
	return reader, writer, cleanup
}

func TestACPHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_ACP_HELPER") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64<<10), maxRPCLineBytes)
	encoder := json.NewEncoder(os.Stdout)
	remoteCount := 0
	for scanner.Scan() {
		var message rpcMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			os.Exit(2)
		}
		switch message.Method {
		case "initialize":
			writeHelperResult(encoder, message.ID, map[string]any{
				"protocolVersion": ProtocolVersion,
				"agentCapabilities": map[string]any{
					"loadSession": true,
					"sessionCapabilities": map[string]any{
						"close": map[string]any{}, "delete": map[string]any{}, "resume": map[string]any{},
						"fork": map[string]any{}, "additionalDirectories": map[string]any{},
					},
				},
				"agentInfo":   map[string]any{"name": "helper-acp", "title": "Helper ACP", "version": "1.0.0"},
				"authMethods": []map[string]any{{"id": "test-auth", "name": "Test auth"}},
				"_meta":       map[string]any{"steering": map[string]any{"supported": true}},
			})
		case "authenticate":
			writeHelperResult(encoder, message.ID, map[string]any{})
		case "session/new":
			remoteCount++
			writeHelperResult(encoder, message.ID, map[string]any{
				"sessionId":     "remote-" + strconv.Itoa(remoteCount),
				"modes":         map[string]any{"currentModeId": "code", "availableModes": []any{}},
				"configOptions": []map[string]any{{"id": "safe", "name": "Safe", "type": "boolean", "currentValue": true}},
			})
		case "session/load", "session/resume":
			writeHelperResult(encoder, message.ID, map[string]any{
				"modes":         map[string]any{"currentModeId": "code", "availableModes": []any{}},
				"configOptions": []map[string]any{{"id": "safe", "name": "Safe", "type": "boolean", "currentValue": true}},
			})
		case "session/fork":
			remoteCount++
			writeHelperResult(encoder, message.ID, map[string]any{"sessionId": "remote-" + strconv.Itoa(remoteCount)})
		case "session/set_mode":
			writeHelperResult(encoder, message.ID, map[string]any{})
		case "session/set_config_option":
			writeHelperResult(encoder, message.ID, map[string]any{
				"configOptions": []map[string]any{{"id": "safe", "name": "Safe", "type": "boolean", "currentValue": false}},
			})
		case "session/close", "session/delete":
			writeHelperResult(encoder, message.ID, map[string]any{})
		case "_session/steering":
			writeHelperResult(encoder, message.ID, map[string]any{"outcome": "injected"})
		case "session/prompt":
			handleHelperPrompt(scanner, encoder, message)
		case "session/cancel", "$/cancel_request":
			// Notifications are accepted; prompt cancellation is exercised by manager context.
		default:
			if len(message.ID) > 0 {
				_ = encoder.Encode(rpcMessage{JSONRPC: "2.0", ID: message.ID, Error: &rpcError{Code: -32601, Message: "Method not found"}})
			}
		}
	}
	os.Exit(0)
}

func handleHelperPrompt(scanner *bufio.Scanner, encoder *json.Encoder, prompt rpcMessage) {
	_ = encoder.Encode(rpcMessage{
		JSONRPC: "2.0", Method: "session/update",
		Params: testMarshalRaw(map[string]any{
			"sessionId": "remote-1",
			"update":    map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "working"}},
		}),
	})
	permissionID := json.RawMessage("900")
	_ = encoder.Encode(rpcMessage{
		JSONRPC: "2.0", ID: permissionID, Method: "session/request_permission",
		Params: testMarshalRaw(map[string]any{
			"sessionId": "remote-1",
			"toolCall":  map[string]any{"toolCallId": "tool-1", "title": "write file", "kind": "edit"},
			"options": []map[string]any{
				{"optionId": "allow-once", "name": "Allow once", "kind": "allow_once"},
				{"optionId": "allow-always", "name": "Always allow", "kind": "allow_always"},
			},
		}),
	})
	for scanner.Scan() {
		var response rpcMessage
		if json.Unmarshal(scanner.Bytes(), &response) != nil {
			os.Exit(3)
		}
		if string(response.ID) != string(permissionID) {
			continue
		}
		var result struct {
			Outcome struct {
				Outcome  string `json:"outcome"`
				OptionID string `json:"optionId"`
			} `json:"outcome"`
		}
		if json.Unmarshal(response.Result, &result) != nil || result.Outcome.Outcome != "selected" || result.Outcome.OptionID != "allow-once" {
			os.Exit(4)
		}
		writeHelperResult(encoder, prompt.ID, map[string]any{"stopReason": "end_turn"})
		return
	}
	os.Exit(5)
}

func writeHelperResult(encoder *json.Encoder, id json.RawMessage, result any) {
	_ = encoder.Encode(rpcMessage{JSONRPC: "2.0", ID: id, Result: testMarshalRaw(result)})
}

func testMarshalRaw(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
