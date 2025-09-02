package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
//	"regexp"
	"rumble/shared/protocol"
)

var profilesDir = filepath.Join("data", "profiles")

/**func safeFileName(name string) string {
	// lowercase, non-alnum -> underscore
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	s := re.ReplaceAllString(name, "_")
	if s == "" {
		s = "player"
	}
	return s
}
**/
func ensureProfilesDir() error {
	return os.MkdirAll(profilesDir, 0o755)
}
/**
func profilePath(name string) string {
	return filepath.Join(profilesDir, safeFileName(name)+".json")
}**/

func loadProfile(name string) (protocol.Profile, error) {
	_ = ensureProfilesDir()
	path := profilePath(name)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		// new profile with sane defaults
		return protocol.Profile{
			Name:      name,
			Army:      nil,
			Armies:    map[string][]string{},
			Gold:      0,
			AccountXP: 0,
			UnitXP:    map[string]int{},
			Resources: map[string]int{},
		}, nil
	}
	if err != nil {
		return protocol.Profile{}, err
	}
	var p protocol.Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return protocol.Profile{}, err
	}
	// ensure maps not nil
	if p.Armies == nil {
		p.Armies = map[string][]string{}
	}
	if p.UnitXP == nil {
		p.UnitXP = map[string]int{}
	}
	if p.Resources == nil {
		p.Resources = map[string]int{}
	}
	return p, nil
}

func saveProfile(p protocol.Profile) error {
	_ = ensureProfilesDir()
	path := profilePath(p.Name)
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
