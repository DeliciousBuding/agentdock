//go:build windows

package envstore

import (
	"path/filepath"
	"testing"
)

func TestMinimalSystemEnvCompletesWindowsProfilePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", "")
	t.Setenv("APPDATA", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("GOPATH", "")
	t.Setenv("GOMODCACHE", "")
	t.Setenv("TEMP", "")
	t.Setenv("TMP", "")

	environment := MinimalSystemEnv()
	checks := map[string]string{
		"USERPROFILE":  home,
		"APPDATA":      filepath.Join(home, "AppData", "Roaming"),
		"LOCALAPPDATA": filepath.Join(home, "AppData", "Local"),
		"GOPATH":       filepath.Join(home, "go"),
		"GOMODCACHE":   filepath.Join(home, "go", "pkg", "mod"),
	}
	for key, expected := range checks {
		if environment[key] != expected {
			t.Fatalf("%s = %q, want %q", key, environment[key], expected)
		}
	}
	if environment["TEMP"] == "" || environment["TMP"] == "" {
		t.Fatalf("temporary environment was not completed: %#v", environment)
	}
}
