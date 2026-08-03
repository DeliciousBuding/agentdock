package acp

import (
	"strconv"
	"strings"
)

const claudeAgentACPName = "@agentclientprotocol/claude-agent-acp"

// requiresHostSteeringFallback identifies released Claude adapters whose
// advertised in-turn steering reaches the SDK but settles session/prompt with
// an EDE diagnostic instead of the steered assistant result. Keep the fallback
// narrow so fixed adapter releases automatically use native injection.
func requiresHostSteeringFallback(info AgentInfo) bool {
	if info.Name != claudeAgentACPName {
		return false
	}
	major, minor, patch, ok := parseReleaseVersion(info.Version)
	if !ok || major != 0 {
		return false
	}
	return minor < 64 || (minor == 64 && patch <= 2)
}

func parseReleaseVersion(value string) (major, minor, patch int, ok bool) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	if suffix := strings.IndexAny(value, "-+"); suffix >= 0 {
		value = value[:suffix]
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	values := []*int{&major, &minor, &patch}
	for index, part := range parts {
		parsed, err := strconv.Atoi(part)
		if err != nil || parsed < 0 {
			return 0, 0, 0, false
		}
		*values[index] = parsed
	}
	return major, minor, patch, true
}
