package acp

import (
	"testing"
	"time"
)

func TestConnectionCloseDoesNotWaitForPeerEOF(t *testing.T) {
	reader, writer, cleanup := pipeConnectionPair(t)
	connection := NewConnection(reader, writer, nil, nil)

	closed := make(chan error, 1)
	go func() { closed <- connection.Close() }()

	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(time.Second):
		cleanup()
		t.Fatal("Close() waited for the peer writer to close")
	}
	cleanup()
}
