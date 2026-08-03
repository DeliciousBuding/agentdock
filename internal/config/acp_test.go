package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFromEnvParsesACPProfile(t *testing.T) {
	t.Setenv("AGENTDOCK_ACP_ENABLED", "true")
	t.Setenv("AGENTDOCK_ACP_AGENT", "claude")
	t.Setenv("AGENTDOCK_ACP_COMMAND", filepath.Join(t.TempDir(), "agent"))
	t.Setenv("AGENTDOCK_ACP_ARGS_JSON", `["adapter.js","--flag"]`)
	t.Setenv("AGENTDOCK_ACP_ENV_FROM_ENV_JSON", `{"ANTHROPIC_API_KEY":"HOST_ANTHROPIC_KEY"}`)
	t.Setenv("AGENTDOCK_ACP_ALLOWED_ROOTS", filepath.Join(t.TempDir(), "one")+","+filepath.Join(t.TempDir(), "two"))
	t.Setenv("AGENTDOCK_ACP_MAX_CONCURRENT_PROMPTS", "3")
	t.Setenv("AGENTDOCK_ACP_INTERACTION_TIMEOUT_MS", "45000")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ACPEnabled || cfg.ACPAgentName != "claude" || cfg.ACPMaxPrompts != 3 || cfg.ACPInteractionMS != 45000 {
		t.Fatalf("ACP config = %#v", cfg)
	}
	if !reflect.DeepEqual(cfg.ACPArgs, []string{"adapter.js", "--flag"}) {
		t.Fatalf("ACP args = %#v", cfg.ACPArgs)
	}
	if cfg.ACPEnvFromEnv["ANTHROPIC_API_KEY"] != "HOST_ANTHROPIC_KEY" {
		t.Fatalf("ACP env mapping = %#v", cfg.ACPEnvFromEnv)
	}
}

func TestNormalizeACPProfileCanonicalizesRoots(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	cfg := Config{
		AgentDockHome: t.TempDir(), AgentDockDefaultDir: t.TempDir(),
		ACPEnabled: true, ACPAgentName: "helper", ACPCommand: executable,
		ACPAllowedRoots: []string{root, link},
	}
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if cfg.ACPMaxPrompts != 2 || cfg.ACPInteractionMS != 300000 {
		t.Fatalf("ACP defaults = prompts %d timeout %d", cfg.ACPMaxPrompts, cfg.ACPInteractionMS)
	}
	if len(cfg.ACPAllowedRoots) != 1 {
		t.Fatalf("deduplicated roots = %#v", cfg.ACPAllowedRoots)
	}
}

func TestNormalizeACPRejectsUnsafeConfiguration(t *testing.T) {
	base := Config{AgentDockHome: t.TempDir(), AgentDockDefaultDir: t.TempDir(), ACPEnabled: true}
	filesystemRoot := string(filepath.Separator)
	if volume := filepath.VolumeName(t.TempDir()); volume != "" {
		filesystemRoot = volume + string(filepath.Separator)
	}
	tests := []struct {
		name   string
		want   string
		mutate func(*Config)
	}{
		{name: "invalid agent name", want: "AGENTDOCK_ACP_AGENT", mutate: func(cfg *Config) {
			cfg.ACPAgentName = "bad\nname"
			cfg.ACPCommand, _ = os.Executable()
			cfg.ACPAllowedRoots = []string{t.TempDir()}
		}},
		{name: "relative command", want: "AGENTDOCK_ACP_COMMAND", mutate: func(cfg *Config) { cfg.ACPCommand = "agent"; cfg.ACPAllowedRoots = []string{t.TempDir()} }},
		{name: "missing roots", want: "AGENTDOCK_ACP_ALLOWED_ROOTS", mutate: func(cfg *Config) { cfg.ACPCommand, _ = os.Executable() }},
		{name: "relative root", want: "AGENTDOCK_ACP_ALLOWED_ROOTS", mutate: func(cfg *Config) { cfg.ACPCommand, _ = os.Executable(); cfg.ACPAllowedRoots = []string{"relative"} }},
		{name: "filesystem root", want: "filesystem root", mutate: func(cfg *Config) { cfg.ACPCommand, _ = os.Executable(); cfg.ACPAllowedRoots = []string{filesystemRoot} }},
		{name: "invalid direct env mapping", want: "AGENTDOCK_ACP_ENV_FROM_ENV_JSON", mutate: func(cfg *Config) {
			cfg.ACPCommand, _ = os.Executable()
			cfg.ACPAllowedRoots = []string{t.TempDir()}
			cfg.ACPEnvFromEnv = map[string]string{"BAD-NAME": "HOST_KEY"}
		}},
		{name: "too many direct args", want: "AGENTDOCK_ACP_ARGS_JSON", mutate: func(cfg *Config) {
			cfg.ACPCommand, _ = os.Executable()
			cfg.ACPAllowedRoots = []string{t.TempDir()}
			cfg.ACPArgs = make([]string, 129)
		}},
		{name: "too many prompts", want: "AGENTDOCK_ACP_MAX_CONCURRENT_PROMPTS", mutate: func(cfg *Config) {
			cfg.ACPCommand, _ = os.Executable()
			cfg.ACPAllowedRoots = []string{t.TempDir()}
			cfg.ACPMaxPrompts = 9
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := base
			test.mutate(&cfg)
			err := cfg.Normalize()
			if err == nil {
				t.Fatal("unsafe ACP configuration was accepted")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to mention %q", err, test.want)
			}
		})
	}
}

func TestFromEnvPreservesACPArgumentBytes(t *testing.T) {
	t.Setenv("AGENTDOCK_ACP_ENABLED", "true")
	t.Setenv("AGENTDOCK_ACP_ARGS_JSON", `["  spaced value  ",""]`)
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.ACPArgs, []string{"  spaced value  ", ""}) {
		t.Fatalf("ACP args were modified: %#v", cfg.ACPArgs)
	}
}

func TestFromEnvRejectsInvalidACPEnvironmentMapping(t *testing.T) {
	t.Setenv("AGENTDOCK_ACP_ENABLED", "true")
	t.Setenv("AGENTDOCK_ACP_ENV_FROM_ENV_JSON", `{"BAD-NAME":"HOST_KEY"}`)
	if _, err := FromEnv(); err == nil {
		t.Fatal("invalid ACP environment mapping was accepted")
	}
}
