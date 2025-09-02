package srv

import (
    "encoding/json"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "time"

    "rumble/shared/protocol"
)

type Social struct {
    mu      sync.RWMutex
    fpath   string
    mpath   string
    friends map[string]map[string]bool // user -> set(friends)
    dms     map[string][]protocol.FriendDM // convoKey -> messages
}

func NewSocial(dataDir string) (*Social, error) {
    s := &Social{
        fpath: filepath.Join(dataDir, "friends.json"),
        mpath: filepath.Join(dataDir, "messages.json"),
        friends: map[string]map[string]bool{},
        dms: map[string][]protocol.FriendDM{},
    }
    if b, err := os.ReadFile(s.fpath); err == nil {
        _ = json.Unmarshal(b, &s.friends)
    }
    if b, err := os.ReadFile(s.mpath); err == nil {
        _ = json.Unmarshal(b, &s.dms)
    }
    return s, nil
}

func (s *Social) save() {
    s.mu.RLock()
    fb, _ := json.MarshalIndent(s.friends, "", "  ")
    mb, _ := json.MarshalIndent(s.dms, "", "  ")
    s.mu.RUnlock()
    _ = os.MkdirAll(filepath.Dir(s.fpath), 0o755)
    _ = os.WriteFile(s.fpath, fb, 0o644)
    _ = os.WriteFile(s.mpath, mb, 0o644)
}

func (s *Social) AddFriend(a, b string) {
    a = strings.ToLower(strings.TrimSpace(a))
    b = strings.ToLower(strings.TrimSpace(b))
    if a == "" || b == "" || a == b { return }
    s.mu.Lock()
    if s.friends[a] == nil { s.friends[a] = map[string]bool{} }
    if s.friends[b] == nil { s.friends[b] = map[string]bool{} }
    s.friends[a][b] = true
    s.friends[b][a] = true
    s.mu.Unlock()
    s.save()
}

func (s *Social) RemoveFriend(a, b string) {
    a = strings.ToLower(strings.TrimSpace(a))
    b = strings.ToLower(strings.TrimSpace(b))
    s.mu.Lock()
    if s.friends[a] != nil { delete(s.friends[a], b) }
    if s.friends[b] != nil { delete(s.friends[b], a) }
    s.mu.Unlock()
    s.save()
}

func (s *Social) ListFriends(user string) []string {
    user = strings.ToLower(strings.TrimSpace(user))
    s.mu.RLock(); defer s.mu.RUnlock()
    var out []string
    for f := range s.friends[user] {
        out = append(out, f)
    }
    sort.Slice(out, func(i,j int) bool { return out[i] < out[j] })
    return out
}

func convoKey(a,b string) string {
    a = strings.ToLower(a); b = strings.ToLower(b)
    if a < b { return a+"|"+b }
    return b+"|"+a
}

func (s *Social) AppendDM(from, to, text string) protocol.FriendDM {
    dm := protocol.FriendDM{From: from, To: to, Text: text, Ts: time.Now().UnixMilli()}
    k := convoKey(from, to)
    s.mu.Lock()
    s.dms[k] = append(s.dms[k], dm)
    s.mu.Unlock()
    s.save()
    return dm
}

func (s *Social) History(a, b string, limit int) []protocol.FriendDM {
    k := convoKey(a,b)
    s.mu.RLock(); defer s.mu.RUnlock()
    msgs := s.dms[k]
    if limit <= 0 || len(msgs) <= limit { return append([]protocol.FriendDM(nil), msgs...) }
    return append([]protocol.FriendDM(nil), msgs[len(msgs)-limit:]...)
}

