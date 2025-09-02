package game

func (g *Game) arenaForHotspot(worldID, hsID string) string {

	if worldID == "rumble_world" {
		switch hsID {
		case "north_tower":
			return "north_tower"
		case "spawn_west":
			return "west_keep"
		case "spawn_east":
			return "east_gate"
		case "mid_bridge":
			return "mid_bridge"
		case "south_gate":
			return "south_gate"
		}
	}

	return "north_tower"
}

func (g *Game) ensureMapHotspots() {
	if g.mapHotspots != nil {
		return
	}

	g.mapHotspots = map[string][]Hotspot{
		"rumble_world": {
			{ID: "spawn_west", Name: "Western Keep", Info: "Good for melee rush.",
				X: 0.1250, Y: 0.5767, Rpx: 18,
				HitRect: &HSRect{Left: 0.18, Top: 0.54, Right: 0.26, Bottom: 0.62}},
			{ID: "spawn_east", Name: "Eastern Gate", Info: "Open field, risky.",
				X: 0.6317, Y: 0.4367, Rpx: 18,
				HitRect: &HSRect{Left: 0.74, Top: 0.43, Right: 0.82, Bottom: 0.51}},
			{ID: "mid_bridge", Name: "Central Bridge", Info: "Choke point.",
				X: 0.2817, Y: 0.1617, Rpx: 18,
				HitRect: &HSRect{Left: 0.47, Top: 0.47, Right: 0.53, Bottom: 0.53}},
			{ID: "north_tower", Name: "North Tower", Info: "High ground.",
				X: 0.7000, Y: 0.1400, Rpx: 18,
				HitRect: &HSRect{Left: 0.53, Top: 0.22, Right: 0.59, Bottom: 0.30}},
			{ID: "south_gate", Name: "South Gate", Info: "Wide approach.",
				X: 0.3567, Y: 0.7350, Rpx: 18,
				HitRect: &HSRect{Left: 0.40, Top: 0.74, Right: 0.48, Bottom: 0.82}},
		},
	}
}

func (g *Game) ensureMapRects() {
	if g.rectHotspots != nil {
		return
	}
	g.rectHotspots = map[string][]HitRect{
		"rumble_world": {

			{ID: "spawn_west", Name: "Western Keep", Info: "Good for melee rush.", L: 0.185, T: 0.540, R: 0.255, B: 0.615},
			{ID: "spawn_east", Name: "Eastern Gate", Info: "Open field, risky.", L: 0.745, T: 0.440, R: 0.815, B: 0.515},
			{ID: "mid_bridge", Name: "Central Bridge", Info: "Choke point.", L: 0.470, T: 0.475, R: 0.530, B: 0.545},
			{ID: "north_tower", Name: "North Tower", Info: "High ground.", L: 0.530, T: 0.210, R: 0.590, B: 0.280},
			{ID: "south_gate", Name: "South Gate", Info: "Wide approach.", L: 0.410, T: 0.750, R: 0.470, B: 0.820},
		},
	}

	g.ensureMapHotspots()
	for mapID, rects := range g.rectHotspots {
		hs := make([]Hotspot, len(rects))
		for i, r := range rects {
			hs[i] = Hotspot{
				ID: r.ID, Name: r.Name, Info: r.Info,
				X: (r.L + r.R) * 0.5, Y: (r.T + r.B) * 0.5, Rpx: 18,
			}
		}
		g.mapHotspots[mapID] = hs
	}
}
