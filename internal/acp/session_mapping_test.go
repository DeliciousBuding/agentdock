package acp

import (
	"context"
	"testing"
	"time"
)

func TestLazySessionLoadRejectsDuplicateRemoteID(t *testing.T) {
	workspace := t.TempDir()
	manager, err := newTestManager(t.TempDir(), workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = manager.Close() }()

	created, err := manager.NewSession(context.Background(), workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	duplicate := SessionRecord{
		SchemaVersion:   sessionSchemaVersion,
		ID:              "acps_duplicate_remote",
		Agent:           created.Session.Agent,
		RemoteSessionID: created.Session.RemoteSessionID,
		CWD:             created.Session.CWD,
		Status:          SessionReady,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := manager.store.Save(duplicate); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.session(duplicate.ID); err == nil || errorCode(err) != "ACP_SESSION_STATE_INVALID" {
		t.Fatalf("duplicate remote session mapping error = %#v", err)
	}
}
