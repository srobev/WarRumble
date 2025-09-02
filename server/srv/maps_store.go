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

func ensureMapsDir() error { return os.MkdirAll(mapsDir, 0o755) }

func mapPath(id string) string { return filepath.Join(mapsDir, id+".json") }

// listMaps reads all map json files and returns MapInfo entries.
func listMaps() []protocol.MapInfo {
    _ = ensureMapsDir()
    out := []protocol.MapInfo{}
    _ = filepath.WalkDir(mapsDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return nil }
        if d.IsDir() { return nil }
        low := strings.ToLower(d.Name())
        if !strings.HasSuffix(low, ".json") { return nil }
        b, err := os.ReadFile(path)
        if err != nil { return nil }
        var def protocol.MapDef
        if err := json.Unmarshal(b, &def); err != nil { return nil }
        id := def.ID
        if id == "" {
            base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
            id = base
        }
        name := def.Name
        if strings.TrimSpace(name) == "" { name = id }
        out = append(out, protocol.MapInfo{ID: id, Name: name})
        return nil
    })
    return out
}

func loadMapDef(id string) (protocol.MapDef, error) {
    _ = ensureMapsDir()
    p := mapPath(id)
    b, err := os.ReadFile(p)
    if err != nil {
        return protocol.MapDef{}, err
    }
    var def protocol.MapDef
    if err := json.Unmarshal(b, &def); err != nil { return protocol.MapDef{}, err }
    if def.ID == "" { def.ID = id }
    if strings.TrimSpace(def.Name) == "" { def.Name = id }
    return def, nil
}

func saveMapDef(def protocol.MapDef) error {
    _ = ensureMapsDir()
    id := strings.TrimSpace(def.ID)
    if id == "" { id = strings.TrimSpace(def.Name) }
    if id == "" { id = "map" }
    def.ID = id
    b, err := json.MarshalIndent(def, "", "  ")
    if err != nil { return err }
    tmp := mapPath(id) + ".tmp"
    if err := os.WriteFile(tmp, b, 0o644); err != nil { return err }
    return os.Rename(tmp, mapPath(id))
}

