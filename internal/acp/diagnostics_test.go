package acp

import (
	"errors"
	"strings"
	"testing"
)

func TestProcessWrapErrorRedactsRemoteDiagnostics(t *testing.T) {
	secret := "test-secret-value"
	process := &agentProcess{spec: AgentSpec{
		Name:        "test-agent",
		Environment: map[string]string{"API_TOKEN": secret},
	}}
	remote := newError("ACP_REMOTE_ERROR", "remote echoed "+secret, false, map[string]any{
		"rpc_data": secret + strings.Repeat("x", 70000),
	}, nil)
	wrapped := process.wrapError("request", remote)
	var acpErr *Error
	if !errors.As(wrapped, &acpErr) {
		t.Fatalf("wrapped error type = %T", wrapped)
	}
	if strings.Contains(acpErr.Message, secret) {
		t.Fatal("remote error message leaked an inherited environment value")
	}
	diagnostic, _ := acpErr.Details["rpc_data"].(string)
	if strings.Contains(diagnostic, secret) || len(diagnostic) > 65560 || !strings.HasSuffix(diagnostic, "...[truncated]") {
		t.Fatalf("sanitized rpc_data length=%d suffix=%v", len(diagnostic), strings.HasSuffix(diagnostic, "...[truncated]"))
	}
}
