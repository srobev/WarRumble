package game

var platform = "desktop"
var playerName = "Player"

// DebugPerksOverlay enables debug visual overlays for perk effects in battle
var DebugPerksOverlay = true

func SetPlatform(p string) {
	if p != "" {
		platform = p
	}
}
func SetPlayerName(n string) {
	if n != "" {
		playerName = n
	}
}
