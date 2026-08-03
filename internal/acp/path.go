package acp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (m *Manager) resolveCWD(raw string) (string, error) {
	if len(m.opts.Agent.AllowedRoots) == 0 {
		return "", newError("ACP_POLICY_INVALID", "ACP agent has no allowed roots", false, nil, nil)
	}
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		candidate = m.opts.Agent.AllowedRoots[0]
	} else if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(m.opts.Agent.AllowedRoots[0], candidate)
	}
	candidate = filepath.Clean(candidate)
	realPath, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", newError("ACP_CWD_INVALID", "resolve ACP working directory", false, map[string]any{"cwd": raw}, err)
	}
	info, err := os.Stat(realPath)
	if err != nil {
		return "", newError("ACP_CWD_INVALID", "stat ACP working directory", false, map[string]any{"cwd": raw}, err)
	}
	if !info.IsDir() {
		return "", newError("ACP_CWD_INVALID", "ACP working directory is not a directory", false, map[string]any{"cwd": raw}, nil)
	}
	for _, root := range m.opts.Agent.AllowedRoots {
		inside, err := pathInsideRoot(root, realPath)
		if err != nil {
			return "", newError("ACP_CWD_INVALID", "compare ACP working directory with allowed root", false, map[string]any{"cwd": realPath, "root": root}, err)
		}
		if inside {
			return realPath, nil
		}
	}
	return "", newError("ACP_CWD_DENIED", "ACP working directory is outside configured allowed roots", false, map[string]any{"cwd": realPath, "allowed_roots": m.AllowedRoots()}, nil)
}

func (m *Manager) resolveAdditionalDirectories(values []string, cwd string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) > 16 {
		return nil, newError("ACP_ADDITIONAL_DIRECTORIES_INVALID", "ACP additional directories exceed the limit", false, map[string]any{"count": len(values), "limit": 16}, nil)
	}
	result := make([]string, 0, len(values))
	infos := make([]os.FileInfo, 0, len(values))
	for _, raw := range values {
		resolved, err := m.resolveCWD(raw)
		if err != nil {
			return nil, err
		}
		if resolved == cwd {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, newError("ACP_CWD_INVALID", "stat ACP additional directory", false, map[string]any{"path": resolved}, err)
		}
		duplicate := false
		for _, existing := range infos {
			if os.SameFile(existing, info) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		infos = append(infos, info)
		result = append(result, resolved)
	}
	return result, nil
}

func pathInsideRoot(root, candidate string) (bool, error) {
	realRoot, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return false, fmt.Errorf("resolve root: %w", err)
	}
	relative, err := filepath.Rel(realRoot, candidate)
	if err != nil {
		return false, err
	}
	if relative == "." {
		return true, nil
	}
	if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false, nil
	}
	return true, nil
}
