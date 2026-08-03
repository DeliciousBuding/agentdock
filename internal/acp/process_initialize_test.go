package acp

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestInitializeAcceptsOmittedAgentInfo(t *testing.T) {
	workspace := t.TempDir()
	manager, err := NewManager(Options{
		Home: t.TempDir(),
		Agent: AgentSpec{
			Name:         "helper",
			Command:      os.Args[0],
			Args:         []string{"-test.run=TestACPHelperProcess"},
			AllowedRoots: []string{workspace},
			Environment: map[string]string{
				"GO_WANT_ACP_HELPER":            "1",
				"GO_ACP_HELPER_OMIT_AGENT_INFO": "1",
			},
		},
		MaxConcurrentRuns:  1,
		InteractionTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = manager.Close() }()

	initialized, err := manager.AgentInfo(context.Background())
	if err != nil {
		t.Fatalf("initialize without agentInfo failed: %v", err)
	}
	if initialized.ProtocolVersion != ProtocolVersion {
		t.Fatalf("protocol version = %d", initialized.ProtocolVersion)
	}
	if initialized.AgentInfo != (AgentInfo{}) {
		t.Fatalf("omitted agentInfo decoded as %#v", initialized.AgentInfo)
	}
}
