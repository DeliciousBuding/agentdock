package acp

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/uvwt/agentdock/internal/fs/atomicfile"
	"github.com/uvwt/agentdock/internal/fs/securepath"
)

const sessionSchemaVersion = 1

type sessionStore struct {
	root  string
	agent string
	mu    sync.Mutex
}

func newSessionStore(home, agent string) (*sessionStore, error) {
	if strings.TrimSpace(home) == "" {
		return nil, errors.New("ACP home is required")
	}
	agent = strings.TrimSpace(agent)
	if agent == "" {
		return nil, errors.New("ACP agent identity is required")
	}
	root := sessionStoreRoot(home, agent)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create ACP session store: %w", err)
	}
	if err := securepath.EnsurePrivate(root); err != nil {
		return nil, fmt.Errorf("secure ACP session store: %w", err)
	}
	store := &sessionStore{root: root, agent: agent}
	if err := store.markInterrupted(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *sessionStore) Save(record SessionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(record)
}

func (s *sessionStore) Get(id string) (SessionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked(id)
}

func (s *sessionStore) List() ([]SessionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("list ACP sessions: %w", err)
	}
	result := make([]SessionRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		record, err := s.loadLocked(id)
		if err != nil {
			return nil, fmt.Errorf("load ACP session %s: %w", id, err)
		}
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	return result, nil
}

func (s *sessionStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete ACP session: %w", err)
	}
	return nil
}

func (s *sessionStore) markInterrupted() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return fmt.Errorf("list ACP sessions during recovery: %w", err)
	}
	now := time.Now().UTC()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		record, err := s.loadLocked(id)
		if err != nil {
			return fmt.Errorf("load ACP session %s during recovery: %w", id, err)
		}
		if record.Status != SessionRunning {
			continue
		}
		record.Status = SessionInterrupted
		record.LastStopReason = "agentdock_restart"
		record.UpdatedAt = now
		if err := s.saveLocked(record); err != nil {
			return err
		}
	}
	return nil
}

func (s *sessionStore) saveLocked(record SessionRecord) error {
	if record.ID == "" || record.RemoteSessionID == "" || record.CWD == "" {
		return errors.New("ACP session id, remote session id, and cwd are required")
	}
	if record.Agent != s.agent {
		return fmt.Errorf("ACP session agent mismatch: expected %q, got %q", s.agent, record.Agent)
	}
	path, err := s.path(record.ID)
	if err != nil {
		return err
	}
	record.SchemaVersion = sessionSchemaVersion
	if err := validateSessionRecord(record.ID, record); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode ACP session: %w", err)
	}
	if len(data) > 1<<20 {
		return errors.New("ACP session state exceeds 1 MiB")
	}
	if err := atomicfile.Write(path, data, 0o600); err != nil {
		return fmt.Errorf("write ACP session: %w", err)
	}
	return nil
}

func (s *sessionStore) loadLocked(id string) (SessionRecord, error) {
	path, err := s.path(id)
	if err != nil {
		return SessionRecord{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SessionRecord{}, newError("ACP_SESSION_NOT_FOUND", "ACP session was not found", false, map[string]any{"session_id": id}, err)
		}
		return SessionRecord{}, fmt.Errorf("stat ACP session: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return SessionRecord{}, errors.New("ACP session state must be a regular non-symlink file")
	}
	if info.Size() > 1<<20 {
		return SessionRecord{}, errors.New("ACP session state exceeds 1 MiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SessionRecord{}, fmt.Errorf("read ACP session: %w", err)
	}
	var record SessionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return SessionRecord{}, fmt.Errorf("decode ACP session: %w", err)
	}
	if err := validateSessionRecord(id, record); err != nil {
		return SessionRecord{}, err
	}
	if record.Agent != s.agent {
		return SessionRecord{}, fmt.Errorf("ACP session agent mismatch: expected %q, got %q", s.agent, record.Agent)
	}
	return record, nil
}

func sessionStoreRoot(home, agent string) string {
	// Agent 名称是持久会话的身份边界。目录使用稳定哈希，避免用户配置的
	// 名称成为路径片段，同时让不同 adapter 的状态、恢复和远程 ID 完全隔离。
	identity := sha256.Sum256([]byte(strings.TrimSpace(agent)))
	return filepath.Join(home, "acp", "sessions", fmt.Sprintf("%x", identity[:16]))
}

func validateSessionRecord(expectedID string, record SessionRecord) error {
	if record.SchemaVersion != sessionSchemaVersion {
		return fmt.Errorf("unsupported ACP session schema version %d", record.SchemaVersion)
	}
	if record.ID != expectedID || record.ID == "" {
		return fmt.Errorf("ACP session id mismatch: expected %q, got %q", expectedID, record.ID)
	}
	if strings.TrimSpace(record.Agent) == "" || strings.TrimSpace(record.RemoteSessionID) == "" {
		return errors.New("ACP session agent and remote session id are required")
	}
	if !filepath.IsAbs(record.CWD) {
		return fmt.Errorf("ACP session cwd must be absolute: %s", record.CWD)
	}
	for _, directory := range record.AdditionalDirectories {
		if !filepath.IsAbs(directory) {
			return fmt.Errorf("ACP additional directory must be absolute: %s", directory)
		}
	}
	switch record.Status {
	case SessionReady, SessionRunning, SessionInterrupted, SessionClosed, SessionError:
	default:
		return fmt.Errorf("invalid ACP session status %q", record.Status)
	}
	if record.CreatedAt.IsZero() || record.UpdatedAt.IsZero() {
		return errors.New("ACP session timestamps are required")
	}
	return nil
}

func (s *sessionStore) path(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, `/\\`) || strings.Contains(id, "..") {
		return "", newError("ACP_SESSION_ID_INVALID", "invalid ACP session id", false, map[string]any{"session_id": id}, nil)
	}
	path := filepath.Join(s.root, id+".json")
	if filepath.Dir(path) != s.root {
		return "", newError("ACP_SESSION_ID_INVALID", "invalid ACP session id", false, map[string]any{"session_id": id}, nil)
	}
	return path, nil
}
