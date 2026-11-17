package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DataDir resolves an explicit override, SKY_DATA_DIR, then the platform default.
// Configuration/secrets are not stored in this directory by Sky.
func DataDir(override string) (string, error) {
	dir := override
	if dir == "" {
		dir = os.Getenv("SKY_DATA_DIR")
	}
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		dir = platformDataDir(runtime.GOOS, home, os.Getenv)
	}
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("data directory must not be blank")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve data directory: %w", err)
	}
	return abs, nil
}

func platformDataDir(goos, home string, getenv func(string) string) string {
	switch goos {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "sky")
	case "windows":
		if local := getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "Sky")
		}
		if roaming := getenv("APPDATA"); roaming != "" {
			return filepath.Join(roaming, "Sky")
		}
		return filepath.Join(home, "AppData", "Local", "Sky")
	default:
		if data := getenv("XDG_DATA_HOME"); data != "" && filepath.IsAbs(data) {
			return filepath.Join(data, "sky")
		}
		return filepath.Join(home, ".local", "share", "sky")
	}
}
