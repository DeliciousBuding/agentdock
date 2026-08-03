package acp

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestConnectionRecoversInboundHandlerPanic(t *testing.T) {
	reader, peerWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	peerReader, writer, err := os.Pipe()
	if err != nil {
		_ = reader.Close()
		_ = peerWriter.Close()
		t.Fatal(err)
	}
	defer func() { _ = peerWriter.Close() }()
	defer func() { _ = peerReader.Close() }()

	connection := NewConnection(reader, writer, func(_ context.Context, method string, _ json.RawMessage) (any, *rpcError) {
		if method == "panic" {
			panic("test panic")
		}
		return map[string]any{"ok": true}, nil
	}, nil)
	defer func() { _ = connection.Close() }()

	encoder := json.NewEncoder(peerWriter)
	decoder := json.NewDecoder(peerReader)
	decode := func() rpcMessage {
		t.Helper()
		result := make(chan rpcMessage, 1)
		errCh := make(chan error, 1)
		go func() {
			var response rpcMessage
			if err := decoder.Decode(&response); err != nil {
				errCh <- err
				return
			}
			result <- response
		}()
		select {
		case response := <-result:
			return response
		case err := <-errCh:
			t.Fatal(err)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for ACP response")
		}
		return rpcMessage{}
	}

	if err := encoder.Encode(rpcMessage{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "panic", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	panicResponse := decode()
	if panicResponse.Error == nil || panicResponse.Error.Code != -32603 {
		t.Fatalf("panic response = %#v", panicResponse)
	}

	if err := encoder.Encode(rpcMessage{JSONRPC: "2.0", ID: json.RawMessage("2"), Method: "ok", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	normalResponse := decode()
	if normalResponse.Error != nil || string(normalResponse.Result) != `{"ok":true}` {
		t.Fatalf("connection was unusable after panic: %#v", normalResponse)
	}
}
