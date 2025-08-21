package protocol

type LeaderboardEntry struct {
    Name   string `json:"name"`
    Rating int    `json:"rating"`
    Rank   string `json:"rank"`
}

type Leaderboard struct {
    Items       []LeaderboardEntry `json:"items"`
    GeneratedAt int64              `json:"generated_at"` // Unix ms (optional, for cache/debug)
}

// Empty request. Client sends this to fetch the board.
type GetLeaderboard struct{}

