package srv

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"rumble/shared/protocol"
)

var mapsDir = filepath.Join("data", "maps")
var arenasDir = filepath.Join("data", "arenas")
var duelsDir = filepath.Join("data", "duels")

func ensureMapsDir() error   { return os.MkdirAll(mapsDir, 0o755) }
func ensureArenasDir() error { return os.MkdirAll(arenasDir, 0o755) }
func ensureDuelsDir() error  { return os.MkdirAll(duelsDir, 0o755) }

func mapPath(id string) string   { return filepath.Join(mapsDir, id+".json") }
func arenaPath(id string) string { return filepath.Join(arenasDir, id+".json") }
func duelPath(id string) string  { return filepath.Join(duelsDir, id+".json") }

// listMaps reads all map json files and returns MapInfo entries.
func listMaps() []protocol.MapInfo {
	_ = ensureMapsDir()
	_ = ensureArenasDir()
	_ = ensureDuelsDir()
	out := []protocol.MapInfo{}

	// Scan regular maps
	_ = filepath.WalkDir(mapsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		low := strings.ToLower(d.Name())
		if !strings.HasSuffix(low, ".json") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var def protocol.MapDef
		if err := json.Unmarshal(b, &def); err != nil {
			return nil
		}
		id := def.ID
		if id == "" {
			base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			id = base
		}
		name := def.Name
		if strings.TrimSpace(name) == "" {
			name = id
		}
		out = append(out, protocol.MapInfo{ID: id, Name: name})
		return nil
	})

	// Scan arenas
	_ = filepath.WalkDir(arenasDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		low := strings.ToLower(d.Name())
		if !strings.HasSuffix(low, ".json") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var def protocol.MapDef
		if err := json.Unmarshal(b, &def); err != nil {
			return nil
		}
		id := def.ID
		if id == "" {
			base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			id = base
		}
		name := def.Name
		if strings.TrimSpace(name) == "" {
			name = id
		}
		// Add [ARENA] prefix to distinguish arenas from regular maps
		if def.IsArena {
			name = "[ARENA] " + name
		}
		out = append(out, protocol.MapInfo{ID: id, Name: name})
		return nil
	})

	// Scan duels
	_ = filepath.WalkDir(duelsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		low := strings.ToLower(d.Name())
		if !strings.HasSuffix(low, ".json") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var def protocol.MapDef
		if err := json.Unmarshal(b, &def); err != nil {
			return nil
		}
		id := def.ID
		if id == "" {
			base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			id = base
		}
		name := def.Name
		if strings.TrimSpace(name) == "" {
			name = id
		}
		// Add [DUEL] prefix to distinguish duels
		if def.IsArena {
			name = "[DUEL] " + name
		}
		out = append(out, protocol.MapInfo{ID: id, Name: name})
		return nil
	})

	return out
}

func loadMapDef(id string) (protocol.MapDef, error) {
	_ = ensureMapsDir()
	_ = ensureArenasDir()
	_ = ensureDuelsDir()

	// First try to load from arenas directory
	p := arenaPath(id)
	b, err := os.ReadFile(p)
	if err == nil {
		var def protocol.MapDef
		if err := json.Unmarshal(b, &def); err != nil {
			return protocol.MapDef{}, err
		}
		if def.ID == "" {
			def.ID = id
		}
		if strings.TrimSpace(def.Name) == "" {
			def.Name = id
		}

		// If this is an arena, mirror the bottom 50% to create the top 50%
		if def.IsArena {
			def = mirrorArenaMap(def)
		}
		return def, nil
	}

	// Try duels directory
	p = duelPath(id)
	b, err = os.ReadFile(p)
	if err == nil {
		var def protocol.MapDef
		if err := json.Unmarshal(b, &def); err != nil {
			return protocol.MapDef{}, err
		}
		if def.ID == "" {
			def.ID = id
		}
		if strings.TrimSpace(def.Name) == "" {
			def.Name = id
		}

		// If this is an arena, mirror the bottom 50% to create the top 50%
		if def.IsArena {
			def = mirrorArenaMap(def)
		}
		return def, nil
	}

	// Fall back to regular maps directory
	p = mapPath(id)
	b, err = os.ReadFile(p)
	if err != nil {
		return protocol.MapDef{}, err
	}
	var def protocol.MapDef
	if err := json.Unmarshal(b, &def); err != nil {
		return protocol.MapDef{}, err
	}
	if def.ID == "" {
		def.ID = id
	}
	if strings.TrimSpace(def.Name) == "" {
		def.Name = id
	}
	return def, nil
}

// mirrorArenaMap takes an arena definition (bottom 50%) and mirrors it to create the full map (top 50%)
func mirrorArenaMap(def protocol.MapDef) protocol.MapDef {
	// Create a copy of the definition
	mirrored := def

	// Mirror deploy zones - bottom zones become top zones
	for _, zone := range def.DeployZones {
		// Mirror Y coordinate: 0.1 becomes 0.9, 0.2 becomes 0.8, etc.
		mirroredY := 1.0 - zone.Y

		// Swap owner for mirrored zones
		mirroredOwner := "enemy"
		if zone.Owner == "enemy" {
			mirroredOwner = "player"
		}

		mirroredZone := protocol.DeployZone{
			X:     zone.X,
			Y:     mirroredY,
			W:     zone.W,
			H:     zone.H,
			Owner: mirroredOwner,
		}
		mirrored.DeployZones = append(mirrored.DeployZones, mirroredZone)
	}

	// Mirror meeting stones
	for _, stone := range def.MeetingStones {
		mirroredStone := protocol.PointF{
			X: stone.X,
			Y: 1.0 - stone.Y, // Mirror Y coordinate
		}
		mirrored.MeetingStones = append(mirrored.MeetingStones, mirroredStone)
	}

	// Mirror gold mines
	for _, mine := range def.GoldMines {
		mirroredMine := protocol.PointF{
			X: mine.X,
			Y: 1.0 - mine.Y, // Mirror Y coordinate
		}
		mirrored.GoldMines = append(mirrored.GoldMines, mirroredMine)
	}

	// Mirror lanes
	for _, lane := range def.Lanes {
		mirroredLane := protocol.Lane{
			Points: make([]protocol.PointF, len(lane.Points)),
			Dir:    -lane.Dir, // Reverse direction for mirrored lane
		}
		for i, point := range lane.Points {
			mirroredLane.Points[i] = protocol.PointF{
				X: point.X,
				Y: 1.0 - point.Y, // Mirror Y coordinate
			}
		}
		mirrored.Lanes = append(mirrored.Lanes, mirroredLane)
	}

	// Mirror base positions
	if def.PlayerBase.X >= 0 && def.PlayerBase.Y >= 0 {
		mirrored.PlayerBase = protocol.PointF{
			X: def.PlayerBase.X,
			Y: 1.0 - def.PlayerBase.Y,
		}
	}
	if def.EnemyBase.X >= 0 && def.EnemyBase.Y >= 0 {
		mirrored.EnemyBase = protocol.PointF{
			X: def.EnemyBase.X,
			Y: 1.0 - def.EnemyBase.Y,
		}
	}

	return mirrored
}

func saveMapDef(def protocol.MapDef) error {
	_ = ensureMapsDir()
	id := strings.TrimSpace(def.ID)
	if id == "" {
		id = strings.TrimSpace(def.Name)
	}
	if id == "" {
		id = "map"
	}
	def.ID = id
	b, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		return err
	}
	tmp := mapPath(id) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, mapPath(id))
}
