package game

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func appDataDir() (string, error) {
	// Cross-platform config dir
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "WarRumble"), nil
	}
	if dir, err := os.UserHomeDir(); err == nil && dir != "" {
		return filepath.Join(dir, ".WarRumble"), nil
	}
	return "", errors.New("no config dir")
}

func SaveAvatar(name string) error {
	return os.WriteFile(ConfigPath("avatar.txt"), []byte(strings.TrimSpace(name)), 0o644)
}

func LoadAvatar() (string, error) {
	b, err := os.ReadFile(ConfigPath("avatar.txt"))
	if err != nil {
		return "", nil // missing is fine
	}
	return strings.TrimSpace(string(b)), nil
}
