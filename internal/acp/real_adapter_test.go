package acp

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// TestRealAdapterInitialize is an opt-in smoke test for a host-installed ACP
// adapter. It is skipped in normal CI because the executable is deliberately a
// host-level dependency rather than a Go module or bundled AgentDock asset.
func TestRealAdapterInitialize(t *testing.T) {
	command := strings.TrimSpace(os.Getenv("AGENTDOCK_TEST_ACP_COMMAND"))
	if command == "" {
		t.Skip("AGENTDOCK_TEST_ACP_COMMAND is not configured")
	}
	var args []string
	if raw := strings.TrimSpace(os.Getenv("AGENTDOCK_TEST_ACP_ARGS_JSON")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			t.Fatalf("decode AGENTDOCK_TEST_ACP_ARGS_JSON: %v", err)
		}
	}
	root := strings.TrimSpace(os.Getenv("AGENTDOCK_TEST_ACP_ROOT"))
	if root == "" {
		root = t.TempDir()
	}
	manager, err := NewManager(Options{
		Home:              t.TempDir(),
		Agent:             AgentSpec{Name: "real-smoke", Command: command, Args: args, AllowedRoots: []string{root}},
		MaxConcurrentRuns: 1, InteractionTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = manager.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	initialized, err := manager.AgentInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if initialized.ProtocolVersion != ProtocolVersion || initialized.AgentInfo.Name == "" {
		t.Fatalf("unexpected initialize response: %#v", initialized)
	}
}
