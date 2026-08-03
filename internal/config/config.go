package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/uvwt/agentdock/internal/fs/securepath"
)

const (
	ProtocolVersion    = "2026-07-28"
	ServerName         = "agentdock"
	Version            = "0.6.1"
	PathModel          = "host"
	BrowserRunnerDir   = "browser-runner"
	BrowserArtifactDir = "browser-artifacts"
	RecallTimeoutMS    = 30000
)

type Config struct {
	AgentDockHome       string
	AgentDockDefaultDir string
	Host                string
	Port                int
	AuthToken           string
	OAuthEnabled        bool
	OAuthServerURL      string
	LogLevel            string
	NexusEndpoint       string
	NexusToken          string
	BrowserEnabled      bool
	BrowserRunnerDir    string
	BrowserNodePath     string
	ACPEnabled          bool
	ACPAgentName        string
	ACPCommand          string
	ACPArgs             []string
	ACPEnvFromEnv       map[string]string
	ACPAllowedRoots     []string
	ACPMaxPrompts       int
	ACPInteractionMS    int
	Stdio               bool
	TrustedProxyCIDRs   []string
}

func FromEnv() (Config, error) {
	port, err := getenvInt("AGENTDOCK_PORT", 8765)
	if err != nil {
		return Config{}, err
	}
	browserEnabled, err := getenvBool("AGENTDOCK_BROWSER_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	oauthEnabled, err := getenvBool("AGENTDOCK_OAUTH_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	stdio, err := getenvBool("AGENTDOCK_STDIO", false)
	if err != nil {
		return Config{}, err
	}
	acpEnabled, err := getenvBool("AGENTDOCK_ACP_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	var acpArgs []string
	var acpEnvFromEnv map[string]string
	acpAgentName := "claude"
	acpCommand := ""
	var acpAllowedRoots []string
	acpMaxPrompts := 2
	acpInteractionMS := 300000
	if acpEnabled {
		acpAgentName = getenv("AGENTDOCK_ACP_AGENT", acpAgentName)
		acpCommand = os.Getenv("AGENTDOCK_ACP_COMMAND")
		acpAllowedRoots = splitCommaSeparated(os.Getenv("AGENTDOCK_ACP_ALLOWED_ROOTS"))
		acpArgs, err = getenvStringSliceJSON("AGENTDOCK_ACP_ARGS_JSON")
		if err != nil {
			return Config{}, err
		}
		acpEnvFromEnv, err = getenvStringMapJSON("AGENTDOCK_ACP_ENV_FROM_ENV_JSON")
		if err != nil {
			return Config{}, err
		}
		acpMaxPrompts, err = getenvInt("AGENTDOCK_ACP_MAX_CONCURRENT_PROMPTS", acpMaxPrompts)
		if err != nil {
			return Config{}, err
		}
		acpInteractionMS, err = getenvInt("AGENTDOCK_ACP_INTERACTION_TIMEOUT_MS", acpInteractionMS)
		if err != nil {
			return Config{}, err
		}
	}
	return Config{
		AgentDockHome:       strings.TrimSpace(os.Getenv("AGENTDOCK_HOME")),
		AgentDockDefaultDir: strings.TrimSpace(os.Getenv("AGENTDOCK_DEFAULT_DIR")),
		Host:                getenv("AGENTDOCK_HOST", "127.0.0.1"),
		Port:                port,
		AuthToken:           os.Getenv("AGENTDOCK_AUTH_TOKEN"),
		OAuthEnabled:        oauthEnabled,
		OAuthServerURL:      os.Getenv("AGENTDOCK_SERVER_URL"),
		LogLevel:            getenv("AGENTDOCK_LOG_LEVEL", "info"),
		NexusEndpoint:       getenv("AGENTDOCK_NEXUS_ENDPOINT", ""),
		NexusToken:          os.Getenv("AGENTDOCK_NEXUS_TOKEN"),
		BrowserEnabled:      browserEnabled,
		BrowserRunnerDir:    os.Getenv("AGENTDOCK_BROWSER_RUNNER_DIR"),
		BrowserNodePath:     os.Getenv("AGENTDOCK_BROWSER_NODE_PATH"),
		ACPEnabled:          acpEnabled,
		ACPAgentName:        acpAgentName,
		ACPCommand:          acpCommand,
		ACPArgs:             acpArgs,
		ACPEnvFromEnv:       acpEnvFromEnv,
		ACPAllowedRoots:     acpAllowedRoots,
		ACPMaxPrompts:       acpMaxPrompts,
		ACPInteractionMS:    acpInteractionMS,
		Stdio:               stdio,
		TrustedProxyCIDRs:   splitCommaSeparated(os.Getenv("AGENTDOCK_TRUSTED_PROXY_CIDRS")),
	}, nil
}

func (c *Config) Normalize() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve user home for AgentDock directories: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return errors.New("resolve user home for AgentDock directories: home directory is empty")
	}
	if c.AgentDockHome == "" {
		c.AgentDockHome = filepath.Join(home, ".agentdock")
	}
	if c.AgentDockDefaultDir == "" {
		c.AgentDockDefaultDir = filepath.Join(home, "AgentDock")
	}
	paths := []struct {
		label string
		value *string
	}{
		{label: "AgentDockHome", value: &c.AgentDockHome},
		{label: "AgentDockDefaultDir", value: &c.AgentDockDefaultDir},
	}
	for _, path := range paths {
		cleaned := filepath.Clean(strings.TrimSpace(*path.value))
		if !filepath.IsAbs(cleaned) {
			return fmt.Errorf("%s must resolve to an absolute path: %s", path.label, cleaned)
		}
		if err := os.MkdirAll(cleaned, 0o700); err != nil {
			return fmt.Errorf("create %s %s: %w", path.label, cleaned, err)
		}
		info, err := os.Stat(cleaned)
		if err != nil {
			return fmt.Errorf("stat %s %s: %w", path.label, cleaned, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory: %s", path.label, cleaned)
		}
		if err := securepath.EnsurePrivate(cleaned); err != nil {
			return fmt.Errorf("secure %s %s: %w", path.label, cleaned, err)
		}
		*path.value = cleaned
	}
	if strings.TrimSpace(c.BrowserRunnerDir) == "" {
		c.BrowserRunnerDir = filepath.Join(c.AgentDockHome, BrowserRunnerDir)
	} else {
		c.BrowserRunnerDir = filepath.Clean(strings.TrimSpace(c.BrowserRunnerDir))
		if !filepath.IsAbs(c.BrowserRunnerDir) {
			return fmt.Errorf("BrowserRunnerDir must resolve to an absolute path: %s", c.BrowserRunnerDir)
		}
	}
	c.BrowserNodePath = strings.TrimSpace(c.BrowserNodePath)
	if c.BrowserNodePath != "" {
		c.BrowserNodePath = filepath.Clean(c.BrowserNodePath)
		if !filepath.IsAbs(c.BrowserNodePath) {
			return fmt.Errorf("BrowserNodePath must resolve to an absolute path: %s", c.BrowserNodePath)
		}
	}
	if err := c.normalizeACP(); err != nil {
		return err
	}
	c.Host = strings.TrimSpace(c.Host)
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	c.OAuthServerURL = strings.TrimSpace(c.OAuthServerURL)
	if c.Port == 0 {
		c.Port = 8765
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535: %d", c.Port)
	}
	c.LogLevel = strings.ToLower(strings.TrimSpace(c.LogLevel))
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.LogLevel == "warning" {
		c.LogLevel = "warn"
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("unsupported log level %q; expected debug, info, warn, or error", c.LogLevel)
	}
	networks := make([]string, 0, len(c.TrustedProxyCIDRs))
	seenNetworks := map[string]struct{}{}
	for _, raw := range c.TrustedProxyCIDRs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("AGENTDOCK_TRUSTED_PROXY_CIDRS contains invalid CIDR %q: %w", raw, err)
		}
		canonical := network.String()
		if _, exists := seenNetworks[canonical]; exists {
			continue
		}
		seenNetworks[canonical] = struct{}{}
		networks = append(networks, canonical)
	}
	c.TrustedProxyCIDRs = networks
	return nil
}

func (c Config) AuthRequired() bool {
	return c.AuthToken != "" || c.OAuthEnabled
}

func (c Config) ValidateAuth() error {
	// stdio 不开放网络监听；HTTP 模式只允许回环地址在无认证下启动。
	// AgentDock 暴露命令和文件写入能力，非回环无认证不是可接受的默认配置。
	if !c.Stdio && !c.AuthRequired() && !isLoopbackBindHost(c.Host) {
		return fmt.Errorf("non-loopback host %q requires AGENTDOCK_AUTH_TOKEN or OAuth", c.Host)
	}
	if !c.OAuthEnabled {
		return nil
	}
	missing := []string{}
	if c.OAuthServerURL == "" {
		missing = append(missing, "AGENTDOCK_SERVER_URL")
	}
	if os.Getenv("AGENTDOCK_OAUTH_PASSWORD") == "" {
		missing = append(missing, "AGENTDOCK_OAUTH_PASSWORD")
	}
	if os.Getenv("AGENTDOCK_OAUTH_TOKEN_SECRET") == "" {
		missing = append(missing, "AGENTDOCK_OAUTH_TOKEN_SECRET")
	}
	if len(missing) > 0 {
		return fmt.Errorf("OAuth enabled by AGENTDOCK_OAUTH_ENABLED but missing required environment variable(s): %s", strings.Join(missing, ", "))
	}
	password := os.Getenv("AGENTDOCK_OAUTH_PASSWORD")
	if len([]rune(password)) < 12 {
		return errors.New("AGENTDOCK_OAUTH_PASSWORD must contain at least 12 characters")
	}
	tokenSecret := os.Getenv("AGENTDOCK_OAUTH_TOKEN_SECRET")
	if len(tokenSecret) < 32 {
		return errors.New("AGENTDOCK_OAUTH_TOKEN_SECRET must contain at least 32 bytes")
	}
	serverURL, err := url.Parse(strings.TrimSpace(c.OAuthServerURL))
	if err != nil || serverURL.Scheme == "" || serverURL.Host == "" {
		return fmt.Errorf("AGENTDOCK_SERVER_URL must be an absolute HTTP(S) URL: %q", c.OAuthServerURL)
	}
	if serverURL.User != nil || serverURL.RawQuery != "" || serverURL.Fragment != "" {
		return fmt.Errorf("AGENTDOCK_SERVER_URL must not contain user info, a query, or a fragment: %q", c.OAuthServerURL)
	}
	if serverURL.Path != "" && serverURL.Path != "/" {
		return fmt.Errorf("AGENTDOCK_SERVER_URL must be an origin without a path: %q", c.OAuthServerURL)
	}
	if serverURL.Scheme == "https" {
		return nil
	}
	if serverURL.Scheme != "http" {
		return fmt.Errorf("AGENTDOCK_SERVER_URL must use https, or http for a loopback host: %q", c.OAuthServerURL)
	}
	hostname := strings.ToLower(serverURL.Hostname())
	if hostname != "localhost" {
		ip := net.ParseIP(hostname)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("AGENTDOCK_SERVER_URL must use https for non-loopback hosts: %q", c.OAuthServerURL)
		}
	}
	return nil
}

func isLoopbackBindHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(strings.TrimSpace(host), "[]"))
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func splitCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if cleaned := strings.TrimSpace(part); cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}

func (c *Config) normalizeACP() error {
	if !c.ACPEnabled {
		c.ACPAgentName = "claude"
		c.ACPCommand = ""
		c.ACPArgs = nil
		c.ACPEnvFromEnv = nil
		c.ACPAllowedRoots = nil
		c.ACPMaxPrompts = 2
		c.ACPInteractionMS = 300000
		return nil
	}
	c.ACPAgentName = strings.TrimSpace(c.ACPAgentName)
	if c.ACPAgentName == "" {
		c.ACPAgentName = "claude"
	}
	if c.ACPMaxPrompts == 0 {
		c.ACPMaxPrompts = 2
	}
	if c.ACPInteractionMS == 0 {
		c.ACPInteractionMS = 300000
	}
	if !validACPAgentName(c.ACPAgentName) {
		return fmt.Errorf("AGENTDOCK_ACP_AGENT must be a 1-64 character identifier using letters, numbers, dot, underscore, or hyphen: %q", c.ACPAgentName)
	}
	if c.ACPMaxPrompts < 1 || c.ACPMaxPrompts > 8 {
		return fmt.Errorf("AGENTDOCK_ACP_MAX_CONCURRENT_PROMPTS must be between 1 and 8: %d", c.ACPMaxPrompts)
	}
	if c.ACPInteractionMS < 1000 || c.ACPInteractionMS > 3600000 {
		return fmt.Errorf("AGENTDOCK_ACP_INTERACTION_TIMEOUT_MS must be between 1000 and 3600000: %d", c.ACPInteractionMS)
	}
	if err := validateACPArguments(c.ACPArgs); err != nil {
		return fmt.Errorf("AGENTDOCK_ACP_ARGS_JSON: %w", err)
	}
	if err := validateACPEnvironmentMapping(c.ACPEnvFromEnv); err != nil {
		return fmt.Errorf("AGENTDOCK_ACP_ENV_FROM_ENV_JSON: %w", err)
	}
	c.ACPCommand = filepath.Clean(strings.TrimSpace(c.ACPCommand))
	if c.ACPCommand == "." || !filepath.IsAbs(c.ACPCommand) {
		return fmt.Errorf("AGENTDOCK_ACP_COMMAND must be an absolute executable path: %s", c.ACPCommand)
	}
	info, err := os.Stat(c.ACPCommand)
	if err != nil {
		return fmt.Errorf("stat AGENTDOCK_ACP_COMMAND %s: %w", c.ACPCommand, err)
	}
	if info.IsDir() {
		return fmt.Errorf("AGENTDOCK_ACP_COMMAND is not a file: %s", c.ACPCommand)
	}
	if err := validateACPCommandPlatform(c.ACPCommand, info); err != nil {
		return err
	}
	if len(c.ACPAllowedRoots) == 0 {
		return errors.New("AGENTDOCK_ACP_ALLOWED_ROOTS must contain at least one directory when ACP is enabled")
	}
	if len(c.ACPAllowedRoots) > 64 {
		return fmt.Errorf("AGENTDOCK_ACP_ALLOWED_ROOTS contains %d directories; maximum is 64", len(c.ACPAllowedRoots))
	}
	roots := make([]string, 0, len(c.ACPAllowedRoots))
	rootInfos := make([]os.FileInfo, 0, len(c.ACPAllowedRoots))
	for _, raw := range c.ACPAllowedRoots {
		cleaned := filepath.Clean(strings.TrimSpace(raw))
		if !filepath.IsAbs(cleaned) {
			return fmt.Errorf("AGENTDOCK_ACP_ALLOWED_ROOTS contains a non-absolute path: %s", raw)
		}
		realRoot, err := filepath.EvalSymlinks(cleaned)
		if err != nil {
			return fmt.Errorf("resolve ACP allowed root %s: %w", cleaned, err)
		}
		if filepath.Dir(realRoot) == realRoot {
			return fmt.Errorf("refusing to use filesystem root as an ACP allowed root: %s", realRoot)
		}
		info, err := os.Stat(realRoot)
		if err != nil {
			return fmt.Errorf("stat ACP allowed root %s: %w", realRoot, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("ACP allowed root is not a directory: %s", realRoot)
		}
		duplicate := false
		for _, existing := range rootInfos {
			if os.SameFile(existing, info) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		rootInfos = append(rootInfos, info)
		roots = append(roots, realRoot)
	}
	c.ACPAllowedRoots = roots
	return nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s as integer: %w", key, err)
	}
	return parsed, nil
}

func getenvBool(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %s as boolean: %w", key, err)
	}
	return parsed, nil
}

func getenvStringSliceJSON(key string) ([]string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, nil
	}
	var result []string
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil, fmt.Errorf("parse %s as JSON string array: %w", key, err)
	}
	if err := validateACPArguments(result); err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	return result, nil
}

func validateACPArguments(arguments []string) error {
	if len(arguments) > 128 {
		return fmt.Errorf("contains %d arguments; maximum is 128", len(arguments))
	}
	totalBytes := 0
	for _, argument := range arguments {
		if strings.ContainsRune(argument, 0) {
			return errors.New("contains an argument with an invalid NUL byte")
		}
		totalBytes += len(argument)
	}
	if totalBytes > 64<<10 {
		return errors.New("argument payload exceeds 65536 bytes")
	}
	return nil
}

func validACPAgentName(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '.', char == '_', char == '-':
		default:
			return false
		}
	}
	return true
}
