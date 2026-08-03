package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func getenvStringMapJSON(key string) (map[string]string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, nil
	}
	var raw map[string]string
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return nil, fmt.Errorf("parse %s as JSON string map: %w", key, err)
	}
	if len(raw) > 64 {
		return nil, fmt.Errorf("%s contains %d mappings; maximum is 64", key, len(raw))
	}
	if err := validateACPEnvironmentMapping(raw); err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	result := make(map[string]string, len(raw))
	for rawChild, rawHost := range raw {
		child := strings.TrimSpace(rawChild)
		host := strings.TrimSpace(rawHost)
		result[child] = host
	}
	return result, nil
}

func validateACPEnvironmentMapping(values map[string]string) error {
	if len(values) > 64 {
		return fmt.Errorf("contains %d mappings; maximum is 64", len(values))
	}
	for rawChild, rawHost := range values {
		child := strings.TrimSpace(rawChild)
		host := strings.TrimSpace(rawHost)
		if child != rawChild || host != rawHost || !validEnvName(child) || !validEnvName(host) {
			return fmt.Errorf("contains an invalid environment variable mapping %q -> %q", rawChild, rawHost)
		}
	}
	return nil
}

func validEnvName(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		if index == 0 {
			if char != '_' && (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') {
				return false
			}
			continue
		}
		if char != '_' && (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}
