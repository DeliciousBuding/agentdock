package acp

import (
	"errors"
	"strings"

	acpruntime "github.com/uvwt/agentdock/internal/acp"
	toolcore "github.com/uvwt/agentdock/internal/tool/core"
)

type Result = toolcore.Result
type ToolError = toolcore.ToolError

func stringArg(args map[string]any, key, fallback string) string {
	return toolcore.StringArg(args, key, fallback)
}

func intArg(args map[string]any, key string, fallback int) int {
	return toolcore.IntArg(args, key, fallback)
}

func boolArg(args map[string]any, key string, fallback bool) bool {
	return toolcore.BoolArg(args, key, fallback)
}

func stringSliceArg(args map[string]any, key string) []string {
	return toolcore.StringSliceArg(args, key)
}

func actionArg(args map[string]any) string {
	return strings.ToLower(strings.TrimSpace(stringArg(args, "action", "")))
}

func acpToolError(err error) error {
	if err == nil {
		return nil
	}
	var acpErr *acpruntime.Error
	if errors.As(err, &acpErr) {
		category := "runtime"
		switch {
		case strings.Contains(acpErr.Code, "START"), strings.Contains(acpErr.Code, "CONNECTION"), strings.Contains(acpErr.Code, "PROTOCOL"), strings.Contains(acpErr.Code, "REMOTE"), strings.Contains(acpErr.Code, "CAPABILITY"):
			category = "integration"
		case acpErr.Code == "ACP_PROMPT_TOO_LARGE", strings.Contains(acpErr.Code, "CURSOR"), strings.Contains(acpErr.Code, "INVALID"), strings.Contains(acpErr.Code, "DENIED"), strings.Contains(acpErr.Code, "NOT_FOUND"), strings.Contains(acpErr.Code, "CLOSED"), strings.Contains(acpErr.Code, "SETTLED"), strings.Contains(acpErr.Code, "MISMATCH"):
			category = "validation"
		}
		return &toolcore.ToolError{
			Code: acpErr.Code, Message: acpErr.Message, Category: category,
			Retryable: acpErr.Retryable, Details: acpErr.Details,
		}
	}
	return toolcore.NewErrorCause("ACP_INTERNAL", "ACP operation failed", "runtime", nil, err)
}

func validationError(code, message string, details map[string]any) error {
	return toolcore.NewErrorDetails(code, message, "validation", details)
}
