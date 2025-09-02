package srv

import (
    "encoding/json"
    "errors"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "time"

    "rumble/shared/protocol"
)

// In-memory guild store with JSON persistence.
type Guilds struct {
    mu     sync.RWMutex
    path   string
    guilds map[string]*Guild
}

type Guild struct {
    GuildID string            `json:"guildId"`
    Name    string            `json:"name"`
    Desc    string            `json:"desc"`
    Privacy string            `json:"privacy"` // public/private
    Region  string            `json:"region"`
    Leader  string            `json:"leader"`
    Members map[string]string `json:"members"` // username -> role
    Created int64             `json:"created"`
}

func NewGuilds(dataDir string) (*Guilds, error) {
    p := filepath.Join(dataDir, "guilds.json")
    g := &Guilds{path: p, guilds: map[string]*Guild{}}
    b, err := os.ReadFile(p)
    if err == nil {
        _ = json.Unmarshal(b, &g.guilds)
    }
    return g, nil
}

// saveUnsafe marshals and writes without acquiring the RWMutex.
// Caller must hold g.mu (at least read lock) or ensure external synchronization.
func (g *Guilds) saveUnsafe() error {
    b, _ := json.MarshalIndent(g.guilds, "", "  ")
    _ = os.MkdirAll(filepath.Dir(g.path), 0o755)
    return os.WriteFile(g.path, b, 0o644)
}

// save acquires a read lock and writes the file; use this when no outer lock is held.
func (g *Guilds) save() error {
    g.mu.RLock()
    b, _ := json.MarshalIndent(g.guilds, "", "  ")
    g.mu.RUnlock()
    _ = os.MkdirAll(filepath.Dir(g.path), 0o755)
    return os.WriteFile(g.path, b, 0o644)
}

func (g *Guilds) Create(name, desc, privacy, region, leader string) (*Guild, error) {
    name = strings.TrimSpace(name)
    if name == "" { return nil, errors.New("missing name") }
    if privacy == "" { privacy = "public" }
    id := makeRoomID("g")
    gg := &Guild{GuildID: id, Name: name, Desc: desc, Privacy: privacy, Region: region, Leader: leader, Members: map[string]string{leader: "leader"}, Created: time.Now().Unix()}
    g.mu.Lock()
    g.guilds[id] = gg
    // write under lock to avoid re-read races; use unsafe to prevent self-deadlock
    _ = g.saveUnsafe()
    g.mu.Unlock()
    return gg, nil
}

func (g *Guilds) Join(guildID, username string) error {
    g.mu.Lock()
    gg := g.guilds[guildID]
    if gg == nil { return errors.New("guild not found") }
    if len(gg.Members) >= 25 { g.mu.Unlock(); return errors.New("guild is full (25)") }
    if gg.Members == nil { gg.Members = map[string]string{} }
    gg.Members[username] = "member"
    err := g.saveUnsafe()
    g.mu.Unlock()
    return err
}

func (g *Guilds) Leave(guildID, username string) error {
    g.mu.Lock()
    gg := g.guilds[guildID]
    if gg == nil { return errors.New("guild not found") }
    delete(gg.Members, username)
    // transfer leadership if leader left
    if gg.Leader == username {
        gg.Leader = ""
        for u, role := range gg.Members {
            if role == "officer" || role == "member" {
                gg.Leader = u
                break
            }
        }
    }
    // delete empty guild
    if len(gg.Members) == 0 {
        delete(g.guilds, guildID)
    }
    err := g.saveUnsafe()
    g.mu.Unlock()
    return err
}

// Admin operations (synchronous, simple validation)
func (g *Guilds) SetRole(gid, actor, user, role string) error {
    g.mu.Lock()
    defer g.mu.Unlock()
    gg := g.guilds[gid]
    if gg == nil { return errors.New("guild not found") }
    // only leader/officer may set roles; only leader can transfer leader
    ar := gg.Members[actor]
    if ar != "leader" && ar != "officer" { return errors.New("insufficient role") }
    if role == "leader" && ar != "leader" { return errors.New("only leader may assign leader") }
    if gg.Members == nil { gg.Members = map[string]string{} }
    gg.Members[user] = role
    if role == "leader" { gg.Leader = user; gg.Members[actor] = "officer" }
    return g.saveUnsafe()
}

func (g *Guilds) Kick(gid, actor, user string) error {
    g.mu.Lock()
    defer g.mu.Unlock()
    gg := g.guilds[gid]
    if gg == nil { return errors.New("guild not found") }
    ar := gg.Members[actor]
    if ar != "leader" && ar != "officer" { return errors.New("insufficient role") }
    // Officers may only kick members (not officers or leader)
    if ar == "officer" {
        if rr, ok := gg.Members[user]; ok && rr != "member" {
            return errors.New("officer can only kick members")
        }
    }
    delete(gg.Members, user)
    if user == gg.Leader { gg.Leader = actor }
    return g.saveUnsafe()
}

func (g *Guilds) SetDesc(gid, actor, desc string) error {
    g.mu.Lock()
    gg := g.guilds[gid]
    if gg == nil { return errors.New("guild not found") }
    ar := gg.Members[actor]
    if ar != "leader" && ar != "officer" { g.mu.Unlock(); return errors.New("insufficient role") }
    gg.Desc = desc
    err := g.saveUnsafe()
    g.mu.Unlock()
    return err
}

func (g *Guilds) List(query string) []protocol.GuildSummary {
    g.mu.RLock(); defer g.mu.RUnlock()
    items := make([]protocol.GuildSummary, 0, len(g.guilds))
    q := strings.ToLower(strings.TrimSpace(query))
    for _, gg := range g.guilds {
        if q != "" && !strings.Contains(strings.ToLower(gg.Name), q) {
            continue
        }
        items = append(items, protocol.GuildSummary{
            GuildID: gg.GuildID,
            Name: gg.Name,
            MembersCount: len(gg.Members),
            Privacy: gg.Privacy,
            Region: gg.Region,
            Activity: len(gg.Members),
        })
    }
    sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
    return items
}

func (g *Guilds) BuildProfile(guildID string) (protocol.GuildProfile, bool) {
    g.mu.RLock(); defer g.mu.RUnlock()
    gg := g.guilds[guildID]
    if gg == nil { return protocol.GuildProfile{}, false }
    prof := protocol.GuildProfile{
        GuildID: gg.GuildID,
        Name: gg.Name,
        Desc: gg.Desc,
        Privacy: gg.Privacy,
        Region: gg.Region,
        Leader: gg.Leader,
    }
    for u, role := range gg.Members {
        prof.Members = append(prof.Members, protocol.GuildMember{
            PlayerID: 0,
            Name: u,
            Role: role,
            LastOnlineMillis: 0,
            Contribution: 0,
            LeagueRank: "",
        })
    }
    sort.Slice(prof.Members, func(i, j int) bool { return strings.ToLower(prof.Members[i].Name) < strings.ToLower(prof.Members[j].Name) })
    return prof, true
}
