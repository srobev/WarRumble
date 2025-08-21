package protocol

type RatingUpdate struct {
    QueueID   string `json:"queue_id"`   // optional correlation id
    NewRating int    `json:"new_rating"`
    Delta     int    `json:"delta"`      // +/-
    Rank      string `json:"rank"`
    OppName   string `json:"opp_name"`   // optional
    OppRating int    `json:"opp_rating"` // optional
    MatchType string `json:"match_type"` // "queue" | "friendly"
}
