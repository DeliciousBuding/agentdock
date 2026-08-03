//go:build darwin || linux

package envstore

import "os"

func platformSystemKeys() []string {
	return []string{"GOPATH", "GOMODCACHE", "NODE_PATH"}
}

func completePlatformEnv(env map[string]string) {
	if env["HOME"] == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			env["HOME"] = home
		}
	}
	if env["TMPDIR"] == "" {
		env["TMPDIR"] = os.TempDir()
	}
}
