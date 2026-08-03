package acp

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	ProtocolVersion = 1
	maxEventCount   = 512
	maxEventBytes   = 4 << 20
)

type AgentSpec struct {
	Name         string
	Command      string
	Args         []string
	AllowedRoots []string
	Environment  map[string]string
}

type Options struct {
	Home               string
	Agent              AgentSpec
	MaxConcurrentRuns  int
	InteractionTimeout time.Duration
}

type Error struct {
	Code      string
	Message   string
	Retryable bool
	Details   map[string]any
	Cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

func newError(code, message string, retryable bool, details map[string]any, cause error) *Error {
	return &Error{Code: code, Message: message, Retryable: retryable, Details: details, Cause: cause}
}

func errorCode(err error) string {
	var acpErr *Error
	if errors.As(err, &acpErr) {
		return acpErr.Code
	}
	return "ACP_INTERNAL"
}

type AgentInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion   int            `json:"protocolVersion"`
	AgentCapabilities map[string]any `json:"agentCapabilities"`
	AgentInfo         AgentInfo      `json:"agentInfo"`
	AuthMethods       []any          `json:"authMethods,omitempty"`
	Meta              map[string]any `json:"_meta,omitempty"`
}

type SessionStatus string

const (
	SessionReady       SessionStatus = "ready"
	SessionRunning     SessionStatus = "running"
	SessionInterrupted SessionStatus = "interrupted"
	SessionClosed      SessionStatus = "closed"
	SessionError       SessionStatus = "error"
	sessionDeleted     SessionStatus = "deleted"
)

type SessionRecord struct {
	SchemaVersion         int           `json:"schema_version"`
	ID                    string        `json:"id"`
	Agent                 string        `json:"agent"`
	RemoteSessionID       string        `json:"remote_session_id"`
	CWD                   string        `json:"cwd"`
	AdditionalDirectories []string      `json:"additional_directories,omitempty"`
	Status                SessionStatus `json:"status"`
	LastStopReason        string        `json:"last_stop_reason,omitempty"`
	CreatedAt             time.Time     `json:"created_at"`
	UpdatedAt             time.Time     `json:"updated_at"`
	ClosedAt              *time.Time    `json:"closed_at,omitempty"`
}

type RunStatus string

const (
	RunRunning     RunStatus = "running"
	RunCompleted   RunStatus = "completed"
	RunCancelled   RunStatus = "cancelled"
	RunInterrupted RunStatus = "interrupted"
	RunFailed      RunStatus = "failed"
)

type Event struct {
	Seq        uint64          `json:"seq"`
	Type       string          `json:"type"`
	SessionID  string          `json:"session_id"`
	RunID      string          `json:"run_id,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	Update     json.RawMessage `json:"update,omitempty"`
	StopReason string          `json:"stop_reason,omitempty"`
	ErrorCode  string          `json:"error_code,omitempty"`
	Message    string          `json:"message,omitempty"`
}

type Run struct {
	ID         string
	SessionID  string
	Status     RunStatus
	StartedAt  time.Time
	EndedAt    *time.Time
	StopReason string
	Err        error
	cancel     func()
	eventsMu   sync.Mutex
	events     []Event
	eventBytes int
	nextSeq    uint64
	notify     chan struct{}
}

func newRun(id, sessionID string) *Run {
	return &Run{
		ID: id, SessionID: sessionID, Status: RunRunning, StartedAt: time.Now().UTC(),
		nextSeq: 1, notify: make(chan struct{}, 1),
	}
}

func (r *Run) appendEvent(event Event) {
	if r == nil {
		return
	}
	r.eventsMu.Lock()
	event.Seq = r.nextSeq
	r.nextSeq++
	event.SessionID = r.SessionID
	event.RunID = r.ID
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	size := len(event.Update) + len(event.Message) + len(event.StopReason) + 128
	r.events = append(r.events, event)
	r.eventBytes += size
	for len(r.events) > 1 && (len(r.events) > maxEventCount || r.eventBytes > maxEventBytes) {
		removed := r.events[0]
		r.eventBytes -= len(removed.Update) + len(removed.Message) + len(removed.StopReason) + 128
		r.events = r.events[1:]
	}
	r.eventsMu.Unlock()
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

func (r *Run) eventsAfter(after uint64, limit int) ([]Event, uint64, uint64, bool) {
	r.eventsMu.Lock()
	defer r.eventsMu.Unlock()
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	out := make([]Event, 0, limit)
	cursor := after
	for _, event := range r.events {
		if event.Seq <= after {
			continue
		}
		out = append(out, event)
		cursor = event.Seq
		if len(out) == limit {
			break
		}
	}
	firstSeq := r.nextSeq
	if len(r.events) > 0 {
		firstSeq = r.events[0].Seq
	}
	truncated := after < firstSeq-1
	return out, cursor, firstSeq, truncated
}

type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

type InteractionStatus string

const (
	InteractionPending   InteractionStatus = "pending"
	InteractionResponded InteractionStatus = "responded"
	InteractionCancelled InteractionStatus = "cancelled"
	InteractionExpired   InteractionStatus = "expired"
)

type Interaction struct {
	ID        string             `json:"id"`
	SessionID string             `json:"session_id"`
	Kind      string             `json:"kind"`
	Status    InteractionStatus  `json:"status"`
	ToolCall  map[string]any     `json:"tool_call,omitempty"`
	Options   []PermissionOption `json:"options,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	ExpiresAt time.Time          `json:"expires_at"`
	respond   chan interactionResponse
}

type interactionResponse struct {
	optionID  string
	cancelled bool
}

func (i *Interaction) public() Interaction {
	if i == nil {
		return Interaction{}
	}
	return Interaction{
		ID: i.ID, SessionID: i.SessionID, Kind: i.Kind, Status: i.Status,
		ToolCall: cloneMap(i.ToolCall), Options: append([]PermissionOption(nil), i.Options...),
		CreatedAt: i.CreatedAt, ExpiresAt: i.ExpiresAt,
	}
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}
