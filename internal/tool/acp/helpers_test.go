package acp

import (
	"errors"
	"testing"

	acpruntime "github.com/uvwt/agentdock/internal/acp"
	toolcore "github.com/uvwt/agentdock/internal/tool/core"
)

func TestACPToolErrorClassifiesPromptSizeAsValidation(t *testing.T) {
	err := acpToolError(&acpruntime.Error{Code: "ACP_PROMPT_TOO_LARGE", Message: "prompt too large"})
	var toolErr *toolcore.ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("tool error type = %T", err)
	}
	if toolErr.Category != "validation" {
		t.Fatalf("tool error category = %q", toolErr.Category)
	}
}

func TestACPToolErrorClassifiesCursorAheadAsValidation(t *testing.T) {
	err := acpToolError(&acpruntime.Error{Code: "ACP_CURSOR_AHEAD", Message: "cursor ahead"})
	var toolErr *toolcore.ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("tool error type = %T", err)
	}
	if toolErr.Category != "validation" {
		t.Fatalf("tool error category = %q", toolErr.Category)
	}
}
