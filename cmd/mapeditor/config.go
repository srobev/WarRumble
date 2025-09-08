package main

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Utility functions for configuration and file operations

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// sanitize: clean string for use in filesystem paths
func sanitize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "_")
	re := regexp.MustCompile(`[^a-z0-9._-]`)
	return re.ReplaceAllString(s, "")
}

// profileID: generate unique profile identifier
func profileID() string {
	if p := strings.TrimSpace(os.Getenv("WAR_PROFILE")); p != "" {
		return sanitize(p)
	}
	exe, _ := os.Executable()
	base := strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	h := sha1.Sum([]byte(exe))
	return sanitize(base) + "-" + hex.EncodeToString(h[:])[:8]
}

// configDir: get configuration directory path
func configDir() string {
	root, _ := os.UserConfigDir()
	if root == "" {
		if home, _ := os.UserHomeDir(); home != "" {
			root = filepath.Join(home, ".config")
		}
	}
	d := filepath.Join(root, "WarRumble", profileID())
	_ = os.MkdirAll(d, 0o755)
	return d
}

// loadToken: read authentication token from config
func loadToken() string {
	b, _ := os.ReadFile(filepath.Join(configDir(), "token.json"))
	return strings.TrimSpace(string(b))
}
