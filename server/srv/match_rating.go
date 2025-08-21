package srv

import "rumble/shared/protocol"

func applyQueueRating(room *Room, winnerID int64, hub *Hub) {
		// Collect the two human players (ignore AI if present)
		var ids []int64
				for _, c := range room.players {
						// clients in room, bound to player ids via JoinClient
						if c != nil && c.id != 0 {
								ids = append(ids, c.id)
						}
				}
		if len(ids) < 2 {
				return // need two humans for rating
		}

		aID, bID := ids[0], ids[1]
				a, aok := room.g.players[aID]
				b, bok := room.g.players[bID]
				if !aok || !bok || a == nil || b == nil {
						return
				}

		// Compute Elo deltas
		var dA, dB int
				if winnerID == a.ID {
						a.Rating, dA = eloApply(a.Rating, b.Rating, true)
								b.Rating, dB = eloApply(b.Rating, a.Rating-dA, false)
				} else {
						b.Rating, dB = eloApply(b.Rating, a.Rating, true)
								a.Rating, dA = eloApply(a.Rating, b.Rating-dB, false)
				}
		a.Rank = rankName(a.Rating)
				b.Rank = rankName(b.Rating)

				// Update in-memory Sessions and persist to disk by name
				hub.mu.Lock()
				var ac, bc *client
				for c := range hub.clients {
						if c.id == a.ID { ac = c }
						if c.id == b.ID { bc = c }
				}
		if ac != nil {
				if s := hub.sessions[ac]; s != nil {
						s.Profile.PvPRating = a.Rating
								s.Profile.PvPRank   = a.Rank
								_ = saveProfile(s.Profile)
				}
		}
		if bc != nil {
				if s := hub.sessions[bc]; s != nil {
						s.Profile.PvPRating = b.Rating
								s.Profile.PvPRank   = b.Rank
								_ = saveProfile(s.Profile)
				}
		}
		hub.mu.Unlock()

				// Notify both players
				hub.send(a.ID, "RatingUpdate", protocol.RatingUpdate{
NewRating: a.Rating,
Delta:     dA,
Rank:      a.Rank,
OppName:   b.Name,
OppRating: b.Rating,
MatchType: "queue",
})
hub.send(b.ID, "RatingUpdate", protocol.RatingUpdate{
NewRating: b.Rating,
Delta:     dB,
Rank:      b.Rank,
OppName:   a.Name,
OppRating: a.Rating,
MatchType: "queue",
})

// Push fresh Profile to both (from the sessions we just updated)
if ac != nil {
		hub.mu.Lock()
				if s := hub.sessions[ac]; s != nil {
						hub.mu.Unlock()
								hub.send(a.ID, "Profile", s.Profile)
				} else {
						hub.mu.Unlock()
				}
}
if bc != nil {
		hub.mu.Lock()
				if s := hub.sessions[bc]; s != nil {
						hub.mu.Unlock()
								hub.send(b.ID, "Profile", s.Profile)
				} else {
						hub.mu.Unlock()
				}
}
}

