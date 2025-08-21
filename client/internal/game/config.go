package game

var platform = "desktop"
var playerName = "Player"

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
