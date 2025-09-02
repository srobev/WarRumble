package game

import (
    "encoding/json"
    "os"
    "path/filepath"
    "rumble/shared/protocol"
)

func guildChatPath(guildID string) string {
    if guildID == "" { guildID = "none" }
    fname := "guildchat_" + guildID + ".json"
    return ConfigPath(fname)
}

func (g *Game) loadGuildChatFromDisk(guildID string) []protocol.GuildChatMsg {
    p := guildChatPath(guildID)
    b, err := os.ReadFile(p)
    if err != nil { return nil }
    var msgs []protocol.GuildChatMsg
    _ = json.Unmarshal(b, &msgs)
    return msgs
}

func (g *Game) saveGuildChatToDisk(guildID string, msgs []protocol.GuildChatMsg) {
    p := guildChatPath(guildID)
    _ = os.MkdirAll(filepath.Dir(p), 0o755)
    // keep at most 200 messages
    if len(msgs) > 200 {
        msgs = msgs[len(msgs)-200:]
    }
    b, _ := json.MarshalIndent(msgs, "", "  ")
    _ = os.WriteFile(p, b, 0o644)
}

