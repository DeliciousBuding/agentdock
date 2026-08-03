package acp

import (
	"context"
	"errors"
	"strings"
	"time"
)

type PromptStartResult struct {
	RunID     string    `json:"run_id"`
	SessionID string    `json:"session_id"`
	Status    RunStatus `json:"status"`
	StartedAt time.Time `json:"started_at"`
}

type PromptEventsResult struct {
	RunID      string     `json:"run_id"`
	SessionID  string     `json:"session_id"`
	Status     RunStatus  `json:"status"`
	Events     []Event    `json:"events"`
	NextSeq    uint64     `json:"next_seq"`
	FirstSeq   uint64     `json:"first_seq"`
	Truncated  bool       `json:"truncated"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	StopReason string     `json:"stop_reason,omitempty"`
	ErrorCode  string     `json:"error_code,omitempty"`
	Message    string     `json:"message,omitempty"`
}

func (m *Manager) StartPrompt(ctx context.Context, sessionID, text string) (PromptStartResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return PromptStartResult{}, newError("ACP_PROMPT_INVALID", "ACP prompt text is required", false, nil, nil)
	}
	if len([]byte(text)) > 256<<10 {
		return PromptStartResult{}, newError("ACP_PROMPT_TOO_LARGE", "ACP prompt exceeds 256 KiB", false, map[string]any{"bytes": len([]byte(text))}, nil)
	}
	if _, err := m.LoadSession(ctx, sessionID); err != nil {
		return PromptStartResult{}, err
	}
	endOperation, err := m.beginSessionOperation(sessionID)
	if err != nil {
		return PromptStartResult{}, err
	}
	defer endOperation()
	select {
	case m.runSlots <- struct{}{}:
	default:
		return PromptStartResult{}, newError("ACP_BUSY", "ACP concurrent prompt limit reached", true, map[string]any{"limit": cap(m.runSlots)}, nil)
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		<-m.runSlots
		return PromptStartResult{}, newError("ACP_MANAGER_CLOSED", "ACP manager is closed", false, nil, nil)
	}
	if existing := m.activeRunBySession[sessionID]; existing != "" {
		m.mu.Unlock()
		<-m.runSlots
		return PromptStartResult{}, newError("ACP_SESSION_BUSY", "ACP session already has an active prompt", true, map[string]any{"session_id": sessionID, "run_id": existing}, nil)
	}
	runID, err := newID("acpr")
	if err != nil {
		m.mu.Unlock()
		<-m.runSlots
		return PromptStartResult{}, err
	}
	run := newRun(runID, sessionID)
	runCtx, cancel := context.WithCancel(context.Background())
	run.cancel = cancel
	m.pruneRunsLocked(time.Now().UTC())
	m.runs[runID] = run
	m.activeRunBySession[sessionID] = runID
	record := m.sessions[sessionID]
	record.Status = SessionRunning
	record.UpdatedAt = time.Now().UTC()
	m.sessions[sessionID] = record
	m.mu.Unlock()

	if err := m.store.Save(record); err != nil {
		m.finishRun(run, RunFailed, "", err)
		return PromptStartResult{}, err
	}
	go m.runPrompt(runCtx, run, record, text)
	return PromptStartResult{RunID: run.ID, SessionID: sessionID, Status: RunRunning, StartedAt: run.StartedAt}, nil
}

func (m *Manager) PromptEvents(ctx context.Context, runID string, after uint64, limit int, wait time.Duration) (PromptEventsResult, error) {
	run, err := m.run(runID)
	if err != nil {
		return PromptEventsResult{}, err
	}
	events, next, first, truncated := run.eventsAfter(after, limit)
	status := runStatus(run)
	if len(events) == 0 && status == RunRunning && wait > 0 {
		if wait > 25*time.Second {
			wait = 25 * time.Second
		}
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-run.notify:
		case <-timer.C:
		case <-ctx.Done():
			return PromptEventsResult{}, newError("ACP_EVENTS_CANCELLED", "ACP event wait was cancelled", true, map[string]any{"run_id": runID}, ctx.Err())
		case <-m.closedCh:
			return PromptEventsResult{}, newError("ACP_MANAGER_CLOSED", "ACP manager is closed", true, nil, nil)
		}
		events, next, first, truncated = run.eventsAfter(after, limit)
	}

	run.eventsMu.Lock()
	result := PromptEventsResult{
		RunID:      run.ID,
		SessionID:  run.SessionID,
		Status:     run.Status,
		Events:     events,
		NextSeq:    next,
		FirstSeq:   first,
		Truncated:  truncated,
		StartedAt:  run.StartedAt,
		EndedAt:    run.EndedAt,
		StopReason: run.StopReason,
	}
	if run.Err != nil {
		result.ErrorCode = errorCode(run.Err)
		result.Message = run.Err.Error()
	}
	run.eventsMu.Unlock()
	return result, nil
}

func (m *Manager) CancelPrompt(_ context.Context, sessionID, runID string) error {
	m.mu.RLock()
	explicitRunID := runID != ""
	if runID == "" {
		runID = m.activeRunBySession[sessionID]
	}
	run := m.runs[runID]
	var record SessionRecord
	if run != nil {
		if sessionID != "" && sessionID != run.SessionID {
			m.mu.RUnlock()
			return newError("ACP_CANCEL_TARGET_MISMATCH", "ACP run does not belong to the supplied session", false, map[string]any{"session_id": sessionID, "run_id": runID, "run_session_id": run.SessionID}, nil)
		}
		sessionID = run.SessionID
		record = m.sessions[sessionID]
	}
	process := m.process
	m.mu.RUnlock()
	if run == nil {
		if explicitRunID {
			return newError("ACP_RUN_NOT_FOUND", "ACP prompt run was not found", false, map[string]any{"run_id": runID}, nil)
		}
		return nil
	}
	if runStatus(run) != RunRunning {
		if explicitRunID {
			return newError("ACP_RUN_SETTLED", "ACP prompt run is no longer running", false, map[string]any{"run_id": runID}, nil)
		}
		return nil
	}
	if process != nil && record.RemoteSessionID != "" {
		_ = process.connection.Notify("session/cancel", map[string]any{"sessionId": record.RemoteSessionID})
	}
	if run.cancel != nil {
		run.cancel()
	}
	m.cancelPendingInteractions(sessionID)
	m.finishRun(run, RunCancelled, "cancelled", nil)
	return nil
}

func (m *Manager) Steer(ctx context.Context, sessionID, text string) (map[string]any, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, newError("ACP_PROMPT_INVALID", "ACP steering text is required", false, nil, nil)
	}
	loaded, err := m.LoadSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	record := loaded.Session
	endOperation, err := m.beginSessionOperation(sessionID)
	if err != nil {
		return nil, err
	}
	defer endOperation()
	process, err := m.ensureProcess(ctx)
	if err != nil {
		return nil, err
	}
	if !process.supportsSteering() {
		return nil, capabilityError("_meta.steering.supported")
	}
	var result map[string]any
	if err := process.connection.Request(ctx, "_session/steering", map[string]any{
		"sessionId": record.RemoteSessionID,
		"prompt":    []map[string]any{{"type": "text", "text": text}},
		"_meta":     map[string]any{"steering": map[string]any{"idleBehavior": "promptRequired"}},
	}, &result); err != nil {
		return nil, process.wrapError("steer ACP session", err)
	}
	return result, nil
}

func (m *Manager) runPrompt(ctx context.Context, run *Run, record SessionRecord, text string) {
	m.mu.RLock()
	process := m.process
	m.mu.RUnlock()
	if process == nil {
		m.finishRun(run, RunFailed, "", newError("ACP_CONNECTION_FAILED", "ACP process is not available", true, nil, nil))
		return
	}
	var response struct {
		StopReason string `json:"stopReason"`
	}
	err := process.connection.Request(ctx, "session/prompt", map[string]any{
		"sessionId": record.RemoteSessionID,
		"prompt":    []map[string]any{{"type": "text", "text": text}},
	}, &response)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			m.finishRun(run, RunCancelled, "cancelled", nil)
			return
		}
		status := RunFailed
		var acpErr *Error
		if errors.As(err, &acpErr) && acpErr.Code == "ACP_CONNECTION_CLOSED" {
			status = RunInterrupted
		}
		m.finishRun(run, status, "", process.wrapError("run ACP prompt", err))
		return
	}
	m.finishRun(run, RunCompleted, response.StopReason, nil)
}

func (m *Manager) finishRun(run *Run, status RunStatus, stopReason string, err error) {
	now := time.Now().UTC()
	run.eventsMu.Lock()
	if run.Status != RunRunning {
		run.eventsMu.Unlock()
		return
	}
	run.Status = status
	run.EndedAt = &now
	run.StopReason = stopReason
	run.Err = err
	run.eventsMu.Unlock()

	switch {
	case err != nil:
		run.appendEvent(Event{Type: "error", ErrorCode: errorCode(err), Message: err.Error()})
	case status == RunCompleted:
		run.appendEvent(Event{Type: "completed", StopReason: stopReason})
	case status == RunCancelled:
		run.appendEvent(Event{Type: "cancelled", StopReason: stopReason})
	case status == RunInterrupted:
		run.appendEvent(Event{Type: "interrupted", StopReason: stopReason})
	}

	m.mu.Lock()
	delete(m.activeRunBySession, run.SessionID)
	_, terminalTransition := m.terminalSessions[run.SessionID]
	var record SessionRecord
	if !terminalTransition {
		record = m.sessions[run.SessionID]
		switch status {
		case RunInterrupted:
			record.Status = SessionInterrupted
		case RunFailed:
			record.Status = SessionError
		default:
			record.Status = SessionReady
		}
		record.LastStopReason = stopReason
		if err != nil && record.LastStopReason == "" {
			record.LastStopReason = errorCode(err)
		}
		record.UpdatedAt = now
		m.sessions[run.SessionID] = record
	}
	m.mu.Unlock()
	if !terminalTransition {
		_ = m.store.Save(record)
	}
	select {
	case <-m.runSlots:
	default:
	}
}

func runStatus(run *Run) RunStatus {
	run.eventsMu.Lock()
	defer run.eventsMu.Unlock()
	return run.Status
}

func (m *Manager) pruneRunsLocked(now time.Time) {
	if len(m.runs) <= 256 {
		return
	}
	for id, run := range m.runs {
		run.eventsMu.Lock()
		settled := run.Status != RunRunning
		endedAt := run.EndedAt
		run.eventsMu.Unlock()
		if settled && endedAt != nil && now.Sub(*endedAt) >= time.Hour {
			delete(m.runs, id)
		}
	}
	if len(m.runs) <= 256 {
		return
	}
	for id, run := range m.runs {
		run.eventsMu.Lock()
		settled := run.Status != RunRunning
		run.eventsMu.Unlock()
		if settled {
			delete(m.runs, id)
		}
		if len(m.runs) <= 192 {
			break
		}
	}
}
