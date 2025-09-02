package game

import (
    "crypto/sha1"
    "encoding/hex"
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

func sanitize(s string) string {
    s = strings.TrimSpace(strings.ToLower(s))
    s = strings.ReplaceAll(s, " ", "_")
    re := regexp.MustCompile(`[^a-z0-9._-]`)
    s = re.ReplaceAllString(s, "")
    if s == "" {
        s = "default"
    }
    return s
}

// profileID picks a per-binary profile:
// 1) WAR_PROFILE env (e.g., "dev", "prod2")
// 2) <exeBase>-<hash8 of full exe path>
func profileID() string {
    if p := strings.TrimSpace(os.Getenv("WAR_PROFILE")); p != "" {
        return sanitize(p)
    }
    exe, _ := os.Executable()
    base := strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
    sum := sha1.Sum([]byte(exe)) // exe path is stable per copy
    return sanitize(base) + "-" + hex.EncodeToString(sum[:])[:8]
}

// ConfigDir = OS config dir / WarRumble / profileID()
// Examples:
//
//   Windows: %APPDATA%\WarRumble\<profile>\
//   macOS:   ~/Library/Application Support/WarRumble/<profile>/
//   Linux:   ~/.config/WarRumble/<profile>/
func ConfigDir() string {
    root, _ := os.UserConfigDir()
    if root == "" {
        home, _ := os.UserHomeDir()
        root = filepath.Join(home, ".config")
    }
    dir := filepath.Join(root, "WarRumble", profileID())
    _ = os.MkdirAll(dir, 0o755)
    return dir
}

func ConfigPath(name string) string {
    return filepath.Join(ConfigDir(), name)
}

