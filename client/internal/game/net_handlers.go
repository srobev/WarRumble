package game

import (
	"encoding/json"
	"fmt"
	"log"
	"rumble/shared/game/types"
	"rumble/shared/protocol"
	"strings"
	"time"
)

// safeTrim is a UI-safe version of trim that avoids odd replacement glyphs.
func safeTrim(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "..."
}

func (g *Game) handle(env Msg) {
	switch env.Type {
	case "Profile":
		var p protocol.Profile
		json.Unmarshal(env.Data, &p)
		g.playerID = p.PlayerID
		if strings.TrimSpace(p.Name) != "" {
			g.name = p.Name
		}
		g.pvpRating = p.PvPRating
		g.pvpRank = p.PvPRank
		g.avatar = p.Avatar
		g.unitXP = p.UnitXP
		// Synchronize account gold from profile on login
		g.accountGold = int64(p.Gold)

		g.send("ListMinis", protocol.ListMinis{})
		g.send("ListMaps", protocol.ListMaps{})
		if len(g.avatars) == 0 {
			g.avatars = g.listAvatars()
		}

		if g.champToMinis == nil {
			g.champToMinis = map[string]map[string]bool{}
		}
		g.champToMinis = map[string]map[string]bool{}
		g.champToOrder = map[string][6]string{}
		for ch, minis := range p.Armies {
			set := map[string]bool{}
			var ord [6]string
			for i, m := range minis {
				set[m] = true
				if i < 6 {
					ord[i] = m
				}
			}
			g.champToMinis[ch] = set
			g.champToOrder[ch] = ord
		}

		g.selectedChampion = ""
		g.selectedMinis = map[string]bool{}
		if len(p.Army) > 0 {
			g.selectedChampion = p.Army[0]
			for _, n := range p.Army[1:] {
				g.selectedMinis[n] = true
			}

			if _, ok := g.champToMinis[g.selectedChampion]; !ok {
				set := map[string]bool{}
				for k := range g.selectedMinis {
					set[k] = true
				}
				g.champToMinis[g.selectedChampion] = set
			}
			// Initialize slot order from saved order or derive from current selection
			g.selectedOrder = [6]string{}
			if ord, ok := g.champToOrder[g.selectedChampion]; ok {
				g.selectedOrder = ord
			} else {
				// derive in minisOnly iteration order
				idx := 0
				for _, m := range g.minisOnly {
					if g.selectedMinis[m.Name] {
						if idx < 6 {
							g.selectedOrder[idx] = m.Name
							idx++
						}
					}
				}
			}
		} else {

			for ch, set := range g.champToMinis {
				g.selectedChampion = ch
				g.selectedMinis = map[string]bool{}
				for m := range set {
					g.selectedMinis[m] = true
				}
				// set order
				g.selectedOrder = [6]string{}
				if ord, ok := g.champToOrder[ch]; ok {
					g.selectedOrder = ord
				} else {
					idx := 0
					for _, mi := range g.minisOnly {
						if g.selectedMinis[mi.Name] {
							if idx < 6 {
								g.selectedOrder[idx] = mi.Name
								idx++
							}
						}
					}
				}
				break
			}
		}
	case "RatingUpdate":
		var ru protocol.RatingUpdate
		json.Unmarshal(env.Data, &ru)
		if ru.MatchType == "queue" {
			g.pvpRating = ru.NewRating
			g.pvpRank = ru.Rank
			sign := "+"
			if ru.Delta < 0 {
				sign = ""
			}
			// Show names vs names with ranks/ratings in brackets
			me := safeTrim(g.name, 14)
			opp := safeTrim(ru.OppName, 14)
			result := "lost"
			if ru.Delta > 0 {
				result = "won"
			}
			g.pvpStatus = fmt.Sprintf("%s (%s) vs %s (%d) — %s, Rating %s%d => %d",
				me, ru.Rank, opp, ru.OppRating, result, sign, ru.Delta, ru.NewRating)
		}
	case "Leaderboard":
		var lb protocol.Leaderboard
		json.Unmarshal(env.Data, &lb)
		g.pvpLeaders = lb.Items
		g.lbLastStamp = lb.GeneratedAt
	case "Minis":
		var m protocol.Minis
		json.Unmarshal(env.Data, &m)
		g.minisAll = m.Items
		g.nameToMini = make(map[string]protocol.MiniInfo, len(g.minisAll))

		g.champions = g.champions[:0]
		g.minisOnly = g.minisOnly[:0]
		g.nameToMini = make(map[string]protocol.MiniInfo, len(g.minisAll))
		for _, it := range g.minisAll {
			g.nameToMini[it.Name] = it
			if strings.EqualFold(it.Role, "champion") || strings.EqualFold(it.Class, "champion") {
				g.champions = append(g.champions, it)
			} else if strings.EqualFold(it.Role, "mini") {
				g.minisOnly = append(g.minisOnly, it)
			}
		}
		g.champScroll, g.miniScroll = 0, 0

	case "Maps":
		var m protocol.Maps
		json.Unmarshal(env.Data, &m)
		g.maps = m.Items

		if g.currentMap == "" {
			g.currentMap = defaultMapID
		}
		g.ensureMapHotspots()
		g.hoveredHS, g.selectedHS = -1, -1

	case "Init":
		var m protocol.Init
		json.Unmarshal(env.Data, &m)
		g.playerID = m.PlayerID
		g.hand = m.Hand
		g.next = m.Next

		var tmp struct {
			OpponentAvatar string `json:"opponentAvatar"`
		}
		_ = json.Unmarshal(env.Data, &tmp)
		g.enemyAvatar = strings.TrimSpace(tmp.OpponentAvatar)

		if g.pendingArena != "" {
			g.currentArena = g.pendingArena
			g.pendingArena = ""
		}

		g.selectedIdx = -1
		g.dragActive = false
		g.endActive = false
		g.endVictory = false
		g.gameOver = false
		g.victory = false
		g.world = &World{
			Units: make(map[int64]*RenderUnit),
			Bases: make(map[int64]protocol.BaseState),
		}

		// Populate obstacles and lanes if available
		if g.currentMapDef != nil {
			g.world.Obstacles = g.currentMapDef.Obstacles
			g.world.Lanes = g.currentMapDef.Lanes
		}

		g.enemyAvatar = ""
		g.enemyTargetThumb = nil
		g.enemyBossPortrait = ""

		// Initialize timer for battle
		g.timerRemainingSeconds = 180 // Default 3:00 minutes
		g.timerPaused = false
		g.pauseOverlay = false

		// Initialize camera for battle map scrolling and zooming - start with 20% zoom in
		g.cameraZoom = 1.2    // Start with 20% zoom in
		g.cameraMinZoom = 1.2 // Can't zoom out beyond 20% (prevents seeing too much)
		g.cameraMaxZoom = 3.0 // Allow zooming in up to 300%
		g.cameraDragging = false

		// Flag to center camera on player's base once bases are populated
		// (bases are populated later via StateDelta/FullSnapshot)
		g.needsCameraCenter = true

		// Clear particle effects when transitioning to battle
		if g.particleSystem != nil {
			g.particleSystem = NewParticleSystem()
		}

		g.scr = screenBattle

	case "GoldUpdate":
		var m protocol.GoldUpdate
		json.Unmarshal(env.Data, &m)
		if m.PlayerID == g.playerID {
			g.gold = m.Gold
		}

	case "GoldSynced":
		var m protocol.GoldSynced
		json.Unmarshal(env.Data, &m)
		g.accountGold = m.Gold
		// Update shop gold display if shop is open
		if g.shopView != nil {
			g.shopView.UpdateGold(m.Gold)
		}

	case "DustSynced":
		var m protocol.DustSynced
		json.Unmarshal(env.Data, &m)
		g.dust = m.Dust

	case "CapsulesSynced":
		var m protocol.CapsulesSynced
		json.Unmarshal(env.Data, &m)
		g.capsules = CapsuleStock{
			Rare:      m.Capsules.Rare,
			Epic:      m.Capsules.Epic,
			Legendary: m.Capsules.Legendary,
		}

	case "StateDelta":
		var d protocol.StateDelta
		json.Unmarshal(env.Data, &d)
		// Ignore state updates if game is paused
		if g.timerPaused {
			return
		}
		// Ensure we have a pre-battle XP snapshot (in case RoomCreated arrived before Profile)
		if g.preBattleXP == nil || len(g.preBattleXP) == 0 {
			if g.unitXP != nil {
				g.preBattleXP = map[string]int{}
				for k, v := range g.unitXP {
					g.preBattleXP[k] = v
				}
			}
		}
		g.world.ApplyDelta(d)

		// Center camera on player's base if needed
		if g.needsCameraCenter && g.scr == screenBattle {
			g.centerCameraOnPlayerBase()
		}

	case "FullSnapshot":
		var s protocol.FullSnapshot
		json.Unmarshal(env.Data, &s)
		g.world = buildWorldFromSnapshot(s, g.currentMapDef)

		// Center camera on player's base if needed
		if g.needsCameraCenter && g.scr == screenBattle {
			g.centerCameraOnPlayerBase()
		}

	case "Error":
		var em protocol.ErrorMsg
		json.Unmarshal(env.Data, &em)
		log.Println("server error:", em.Message)
		if g.socialActive() == socialFriends {
			g.friendSearchError = strings.TrimSpace(em.Message)
		}

	// Shop messages
	case "ShopRollSynced":
		var rollSync protocol.ShopRollSynced
		json.Unmarshal(env.Data, &rollSync)
		// Convert from protocol.ShopRoll to shared types.ShopRoll
		if rollData, ok := rollSync.Roll.(map[string]interface{}); ok {
			if slots, ok := rollData["slots"].([]interface{}); ok {
				shopRoll := types.ShopRoll{
					Version: 1,
				}
				shopRoll.Slots = make([]types.ShopSlot, len(slots))
				for i, slotInt := range slots {
					if slotMap, ok := slotInt.(map[string]interface{}); ok {
						slot := types.ShopSlot{
							Slot:       int(slotMap["slot"].(float64)),
							UnitID:     slotMap["unitId"].(string),
							IsChampion: slotMap["isChampion"].(bool),
							PriceGold:  int64(slotMap["priceGold"].(float64)),
							Sold:       slotMap["sold"].(bool),
						}
						shopRoll.Slots[i] = slot
					}
				}
				// Update local shop state
				g.shopRoll = shopRoll
				// Update shop grid if shop view is active
				if g.shopView != nil && g.activeTab == tabShop {
					g.shopView.UpdateShopRoll(shopRoll)
				}
			}
		}

	case "BuyShopResult":
		var result protocol.BuyShopResult
		json.Unmarshal(env.Data, &result)

		// Update local state
		g.accountGold = result.Gold

		// Clear any pending purchase state
		if g.shopQueryIdx >= 0 && g.shopQueryIdx < len(g.shopRoll.Slots) {
			g.shopRoll.Slots[g.shopQueryIdx].Sold = true
			g.shopQueryIdx = -1
		}

		// Notify shop UI of successful purchase
		if g.shopView != nil {
			g.shopView.OnBuyResult(result)
		}

		log.Printf("Shop purchase successful: %s (shards: %d/%d)", result.UnitID, result.Shards, result.Threshold)

		// Update unit progression data for display
		if g.unitProgression == nil {
			g.unitProgression = make(map[string]protocol.UnitProgressSynced)
		}
		g.unitProgression[result.UnitID] = protocol.UnitProgressSynced{
			UnitID:      result.UnitID,
			Rank:        result.Rank,
			ShardsOwned: result.Shards,
		}

	case "UnitProgressSynced":
		var progress protocol.UnitProgressSynced
		json.Unmarshal(env.Data, &progress)

		// Update local unit progression state
		if g.unitProgression == nil {
			g.unitProgression = make(map[string]protocol.UnitProgressSynced)
		}
		g.unitProgression[progress.UnitID] = progress

		log.Printf("Unit progression updated: %s (rank %d, shards %d)", progress.UnitID, progress.Rank, progress.ShardsOwned)

	case "HandUpdate":
		var hu protocol.HandUpdate
		json.Unmarshal(env.Data, &hu)
		g.hand = hu.Hand
		g.next = hu.Next

	case "GameOver":
		var m protocol.GameOver
		json.Unmarshal(env.Data, &m)
		g.gameOver = true
		g.victory = (m.WinnerID == g.playerID)
		// Compute XP gains from pre-battle snapshot
		g.xpGains = map[string]int{}
		if g.preBattleXP != nil && g.unitXP != nil {
			names := g.battleArmy
			if len(names) == 0 {
				if g.selectedChampion != "" {
					names = append(names, g.selectedChampion)
				}
				for i := 0; i < 6; i++ {
					if g.selectedOrder[i] != "" {
						names = append(names, g.selectedOrder[i])
					}
				}
			}
			// helper: case-insensitive lookup
			getXP := func(m map[string]int, key string) int {
				if v, ok := m[key]; ok {
					return v
				}
				for k, v := range m {
					if strings.EqualFold(k, key) {
						return v
					}
				}
				return 0
			}
			for _, n := range names {
				after := getXP(g.unitXP, n)
				before := getXP(g.preBattleXP, n)
				if d := after - before; d > 0 {
					g.xpGains[n] = d
				} else {
					g.xpGains[n] = 0
				}
			}
		}
	case "TimerUpdate":
		var tu protocol.TimerUpdate
		json.Unmarshal(env.Data, &tu)
		g.timerRemainingSeconds = tu.RemainingSeconds
		g.timerPaused = tu.IsPaused
		// Close pause overlay if game is resumed
		if !tu.IsPaused {
			g.pauseOverlay = false
		}
	case "HealingEvent":
		var he protocol.HealingEvent
		json.Unmarshal(env.Data, &he)

		// Trigger P-style healing particle effects (healer wave + target particles)
		if g.particleSystem != nil {
			// Healing wave from healer (like P key effect)
			g.particleSystem.CreateUnitAbilityEffect(he.HealerX, he.HealerY, "heal")
			// Green particles on healed target
			g.particleSystem.CreateTargetHealingEffect(he.TargetX, he.TargetY)
		}
	case "UnitDeathEvent":
		var de protocol.UnitDeathEvent
		json.Unmarshal(env.Data, &de)

		// Trigger unit death particle effects
		if g.particleSystem != nil {
			g.particleSystem.CreateUnitDeathEffect(de.UnitX, de.UnitY, de.UnitClass, de.UnitSubclass)
		}
	case "UnitSpawnEvent":
		var se protocol.UnitSpawnEvent
		json.Unmarshal(env.Data, &se)

		// Trigger unit spawn animation (zoomed image falling from sky)
		if g.world != nil {
			g.world.StartSpawnAnimation(se.UnitID, se.UnitName, se.UnitClass, se.UnitSubclass, se.UnitX, se.UnitY)
		}
	case "VictoryEvent":
		var ve protocol.VictoryEvent
		json.Unmarshal(env.Data, &ve)

		// Trigger victory celebration effects
		if g.particleSystem != nil {
			g.particleSystem.CreateVictoryCelebration()
		}

		// Set victory state
		g.victory = true
		g.gameOver = true

		log.Printf("Victory! Gold earned: %d, XP gained: %d", ve.GoldEarned, ve.XPGained)

	case "DefeatEvent":
		var de protocol.DefeatEvent
		json.Unmarshal(env.Data, &de)

		// Trigger defeat effects
		if g.particleSystem != nil {
			g.particleSystem.CreateDefeatEffect()
		}

		// Set defeat state
		g.victory = false
		g.gameOver = true

		log.Printf("Defeat! Lost to %s", de.WinnerName)
	case "BaseDamageEvent":
		var bde protocol.BaseDamageEvent
		json.Unmarshal(env.Data, &bde)

		// Trigger base damage effects
		if g.particleSystem != nil {
			g.particleSystem.CreateBaseDamageEffect(bde.BaseX, bde.BaseY, bde.Damage, bde.BaseHP, bde.BaseMaxHP)
		}

		log.Printf("Base damage: %d damage dealt by %s, base HP: %d/%d", bde.Damage, bde.AttackerName, bde.BaseHP, bde.BaseMaxHP)
	case "AoEDamageEvent":
		var aoe protocol.AoEDamageEvent
		json.Unmarshal(env.Data, &aoe)

		// Trigger AoE visual effect - show the damage circle
		if g.particleSystem != nil {
			g.particleSystem.CreateAoEImpactEffect(aoe.ImpactX, aoe.ImpactY, aoe.Damage)
		}

		log.Printf("AoE damage: %d damage to %s at (%.1f, %.1f)", aoe.Damage, aoe.TargetName, aoe.TargetX, aoe.TargetY)
	case "MapDef":
		var md protocol.MapDefMsg
		json.Unmarshal(env.Data, &md)
		g.currentMapDef = &md.Def
		// Set current arena for background loading if this is an arena
		if md.Def.IsArena {
			g.currentArena = md.Def.ID
		}

		// Update world with obstacles and lanes from the map definition
		if g.world != nil {
			g.world.Obstacles = md.Def.Obstacles
			g.world.Lanes = md.Def.Lanes
		}
	case "FriendlyCode":
		var m protocol.FriendlyCode
		json.Unmarshal(env.Data, &m)
		g.pvpCode = strings.ToUpper(strings.TrimSpace(m.Code))
		g.pvpStatus = "Share this code: " + g.pvpCode

	case "RoomCreated":
		var rc protocol.RoomCreated
		json.Unmarshal(env.Data, &rc)
		g.roomID = rc.RoomID

		g.pvpQueued = false
		g.pvpHosting = false
		// Snapshot XP before battle starts
		g.preBattleXP = map[string]int{}
		for k, v := range g.unitXP {
			g.preBattleXP[k] = v
		}
		// Capture the army composition at battle start
		g.battleArmy = nil
		if g.selectedChampion != "" {
			g.battleArmy = append(g.battleArmy, g.selectedChampion)
		}
		for i := 0; i < 6; i++ {
			if g.selectedOrder[i] != "" {
				g.battleArmy = append(g.battleArmy, g.selectedOrder[i])
			}
		}

	case "LoggedOut":

		g.resetToLoginNoAutoConnect()
		return

	// -------- Guild / Social --------
	case "GuildNone":
		g.guildID = ""
		g.guildName = ""
		g.guildMembers = nil
		g.selectedGuildMember = ""
		g.guildChat = nil
		g.guildBrowse = true
		g.prevGuildRoles = nil
		g.havePrevGuildRoster = false
		g.guildList = nil
		g.send("ListGuilds", protocol.ListGuilds{Query: ""})
	case "GuildInfo":
		var gi protocol.GuildInfo
		json.Unmarshal(env.Data, &gi)
		g.guildID = gi.Profile.GuildID
		g.guildName = gi.Profile.Name
		g.guildDescEdit = gi.Profile.Desc
		// Load any persisted chat first (so new system lines append to existing)
		g.guildChat = g.loadGuildChatFromDisk(g.guildID)
		// compute diffs before overwriting
		old := g.prevGuildRoles
		curr := make(map[string]string, len(gi.Profile.Members))
		for _, m := range gi.Profile.Members {
			curr[m.Name] = strings.ToLower(m.Role)
		}
		if !g.havePrevGuildRoster {
			g.prevGuildRoles = curr
			g.havePrevGuildRoster = true
		} else {
			// joins
			for name := range curr {
				if _, ok := old[name]; !ok {
					g.guildChat = append(g.guildChat, protocol.GuildChatMsg{From: "", Text: name + " joined the guild", Ts: time.Now().UnixMilli(), System: true})
				}
			}
			// leaves
			for name := range old {
				if _, ok := curr[name]; !ok {
					g.guildChat = append(g.guildChat, protocol.GuildChatMsg{From: "", Text: name + " left the guild", Ts: time.Now().UnixMilli(), System: true})
				}
			}
			// promotions/demotions
			for name, newRole := range curr {
				if prevRole, ok := old[name]; ok && prevRole != newRole {
					msg := name + " is now " + newRole
					g.guildChat = append(g.guildChat, protocol.GuildChatMsg{From: "", Text: msg, Ts: time.Now().UnixMilli(), System: true})
				}
			}
			g.prevGuildRoles = curr
		}
		g.guildMembers = gi.Profile.Members
		// If we are no longer in the roster, treat as GuildNone fallback
		meIn := false
		for _, m := range g.guildMembers {
			if strings.EqualFold(m.Name, g.name) {
				meIn = true
				break
			}
		}
		if !meIn {
			g.guildID = ""
			g.guildName = ""
			g.guildMembers = nil
			g.selectedGuildMember = ""
			g.guildChat = nil
			g.guildBrowse = true
			g.prevGuildRoles = nil
			g.havePrevGuildRoster = false
			g.guildList = nil
			g.send("ListGuilds", protocol.ListGuilds{Query: ""})
			return
		}
		// Persist chat updates if any (after appending system lines)
		if g.guildID != "" {
			g.saveGuildChatToDisk(g.guildID, g.guildChat)
		}
	case "GuildList":
		var gl protocol.GuildList
		json.Unmarshal(env.Data, &gl)
		g.guildList = gl.Items
	case "GuildChatMsg":
		var m protocol.GuildChatMsg
		json.Unmarshal(env.Data, &m)
		g.guildChat = append(g.guildChat, m)
		g.saveGuildChatToDisk(g.guildID, g.guildChat)

	// Friends / DMs
	case "FriendsList":
		var fl protocol.FriendsList
		json.Unmarshal(env.Data, &fl)
		g.friends = fl.Items
	case "FriendDM":
		var dm protocol.FriendDM
		json.Unmarshal(env.Data, &dm)
		// Append only if relevant (to or from me)
		g.dmHistory = append(g.dmHistory, dm)
	case "FriendHistory":
		var fh protocol.FriendHistory
		json.Unmarshal(env.Data, &fh)
		// Replace local history when fetching
		g.dmHistory = fh.Items
	// (handled earlier)
	case "UserProfile":
		var up protocol.UserProfile
		json.Unmarshal(env.Data, &up)
		// If we're in the middle of an Add Friend lookup, handle that flow
		if name := strings.TrimSpace(g.friendAddLookup); name != "" {
			// Clear lookup flag now; we'll set error or send add
			g.friendAddLookup = ""
			profName := strings.TrimSpace(up.Profile.Name)
			if profName != "" && strings.EqualFold(profName, name) && !strings.EqualFold(profName, g.name) {
				// Check not already a friend
				already := false
				for _, fr := range g.friends {
					if strings.EqualFold(fr.Name, profName) {
						already = true
						break
					}
				}
				if already {
					g.friendSearchError = "Already in your friends"
				} else {
					g.friendSearchError = ""
					g.send("AddFriend", protocol.AddFriend{Name: profName})
					// After server responds FriendsList, UI will refresh
					g.friendSearch = ""
					g.friendSearchFocus = false
				}
			} else {
				if strings.EqualFold(profName, g.name) {
					g.friendSearchError = "Cannot add yourself"
				} else {
					g.friendSearchError = "Player not found"
				}
			}
			// Do not open overlay when it's an add lookup
			return
		}
		// Otherwise, open profile overlay as usual
		g.memberProfile = up.Profile
		g.memberProfileOverlay = true
	}
}

func (g *Game) onArmySave(cards []string) {
	g.send("SaveArmy", protocol.SaveArmy{Cards: cards})
	g.send("GetProfile", protocol.GetProfile{})
}

func (g *Game) onMapClicked(arenaID string) {
	g.currentArena = arenaID
	g.send("CreatePve", protocol.CreatePve{MapID: arenaID})
}
func (g *Game) onStartBattle() { g.send("StartBattle", protocol.StartBattle{}) }
func (g *Game) onLeaveRoom() {
	g.send("LeaveRoom", protocol.LeaveRoom{})
	g.currentArena = ""
}

// centerCameraOnPlayerBase centers the camera with bottom aligned to map bottom
func (g *Game) centerCameraOnPlayerBase() {
	if g.world == nil {
		return
	}

	// Find player's base
	playerBaseX, playerBaseY := 0.0, 0.0
	found := false
	for _, b := range g.world.Bases {
		if b.OwnerID == g.playerID {
			playerBaseX = float64(b.X + b.W/2) // Center of base
			playerBaseY = float64(b.Y + b.H/2)
			found = true
			break
		}
	}

	if found {
		// Position camera so bottom of screen aligns with map bottom
		// Keep player's base in view but prioritize showing full map height
		screenCenterX := float64(protocol.ScreenW) / 2
		g.cameraX = screenCenterX - playerBaseX*g.cameraZoom

		// Position camera so bottom of screen is at map bottom (assuming map height is available)
		// For now, position to show more of the map from the bottom
		g.cameraY = float64(protocol.ScreenH) - playerBaseY*g.cameraZoom - 100 // Small offset from bottom

		g.needsCameraCenter = false // Clear the flag
	}
}
