//go:build windows

package envstore

import (
	"os"
	"path/filepath"
)

func platformSystemKeys() []string {
	return []string{
		"USERPROFILE", "HOMEDRIVE", "HOMEPATH", "APPDATA", "LOCALAPPDATA", "PROGRAMDATA",
		"SYSTEMROOT", "COMSPEC", "PATHEXT", "TEMP", "TMP", "GOPATH", "GOMODCACHE", "NODE_PATH",
	}
}

func completePlatformEnv(env map[string]string) {
	home := env["USERPROFILE"]
	if home == "" {
		home = env["HOME"]
	}
	if home == "" {
		if resolved, err := os.UserHomeDir(); err == nil {
			home = resolved
		}
	}
	if home != "" {
		env["USERPROFILE"] = home
		if env["HOME"] == "" {
			env["HOME"] = home
		}
		if env["APPDATA"] == "" {
			env["APPDATA"] = filepath.Join(home, "AppData", "Roaming")
		}
		if env["LOCALAPPDATA"] == "" {
			env["LOCALAPPDATA"] = filepath.Join(home, "AppData", "Local")
		}
		if env["GOPATH"] == "" {
			env["GOPATH"] = filepath.Join(home, "go")
		}
		if env["GOMODCACHE"] == "" {
			env["GOMODCACHE"] = filepath.Join(env["GOPATH"], "pkg", "mod")
		}
	}
	temporary := env["TEMP"]
	if temporary == "" && home != "" {
		temporary = filepath.Join(home, "AppData", "Local", "Temp")
	}
	if temporary == "" {
		temporary = os.TempDir()
	}
	if temporary != "" {
		env["TEMP"] = temporary
		if env["TMP"] == "" {
			env["TMP"] = temporary
		}
	}
}
