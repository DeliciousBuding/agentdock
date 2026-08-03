//go:build windows

package config

import "os"

func validateACPCommandPlatform(_ string, _ os.FileInfo) error { return nil }
