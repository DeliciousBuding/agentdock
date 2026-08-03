package acp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionStoreFailsClosedOnCorruptState(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "acp", "sessions")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "acps_corrupt.json"), []byte(`{"schema_version":1`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newSessionStore(home); err == nil {
		t.Fatal("corrupt ACP state was silently ignored")
	}
}

func TestSessionStoreRejectsSymlinkState(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "acp", "sessions")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "acps_link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	if _, err := newSessionStore(home); err == nil {
		t.Fatal("symlink ACP state was silently followed")
	}
}
