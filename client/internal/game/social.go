package game

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"rumble/shared/protocol"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

// socialSubTab enumerates the Social view: Friends, Guild, Messages.
type socialSubTab int

const (
	socialFriends socialSubTab = iota
	socialGuild
	socialMessages
)

// Minimal guild state placeholders. Integrate with backend later.
type guildMember struct {
	Name string
	Role string // leader/officer/member
	Last string // last online
}

// UpdateSocial handles input for Social tab
func (g *Game) updateSocial() {
	// Use exact cursor position like other tabs
	mx, my := ebiten.CursorPosition()

	// Handle overlays first - they should handle their own clicks and dismiss
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Handle member profile overlay close and button clicks
		if g.memberProfileOverlay {
			w, h := 520, 380
			x := (protocol.ScreenW - w) / 2
			y := (protocol.ScreenH - h) / 2

			// Check buttons first
			isSelf := strings.EqualFold(g.memberProfile.Name, g.name)
			canKick := false
			canPromote := false
			canDemote := false
			canTransfer := false
			isFriend := false
			for _, fr := range g.friends {
				if strings.EqualFold(fr.Name, g.memberProfile.Name) {
					isFriend = true
					break
				}
			}

			meRole := "member"
			selRole := "member"
			for _, m := range g.guildMembers {
				if strings.EqualFold(m.Name, g.name) {
					meRole = strings.ToLower(m.Role)
				}
				if strings.EqualFold(m.Name, g.memberProfile.Name) {
					selRole = strings.ToLower(m.Role)
				}
			}

			if meRole == "leader" {
				canKick = !isSelf
				canPromote = !isSelf
				canDemote = !isSelf
				canTransfer = !isSelf
			} else if meRole == "officer" {
				if selRole == "member" {
					canKick = !isSelf
					canPromote = true
					canDemote = true
				}
			}

			actionsTop := y + int(float64(h)*0.85) - 64

			// Handle button clicks
			if !g.profileFromFriends {
				if canPromote && ptIn(mx, my, image.Rect(x+14+84, actionsTop, x+14+84+96, actionsTop+24)) {
					g.send("PromoteMember", protocol.PromoteMember{User: g.memberProfile.Name})
					g.send("GetGuild", protocol.GetGuild{})
					return
				}
				if canDemote && ptIn(mx, my, image.Rect(x+14+84+110, actionsTop, x+14+84+110+96, actionsTop+24)) {
					g.send("DemoteMember", protocol.DemoteMember{User: g.memberProfile.Name})
					g.send("GetGuild", protocol.GetGuild{})
					return
				}
				if canKick && ptIn(mx, my, image.Rect(x+14+84, actionsTop+30, x+14+84+96, actionsTop+30+24)) {
					g.send("KickMember", protocol.KickMember{User: g.memberProfile.Name})
					g.send("GetGuild", protocol.GetGuild{})
					return
				}
				if canTransfer && ptIn(mx, my, image.Rect(x+14+84+110, actionsTop+30, x+14+84+110+96, actionsTop+30+24)) {
					g.transferLeaderTarget = g.memberProfile.Name
					g.transferLeaderConfirm = true
					return
				}
			}

			// Handle Message and Unfriend buttons
			if isFriend && !isSelf {
				baseY := actionsTop
				if !g.profileFromFriends {
					baseY = actionsTop + 60
				}
				if ptIn(mx, my, image.Rect(x+14+84, baseY, x+14+84+96, baseY+24)) {
					g.selectedFriend = g.memberProfile.Name
					g.socialTab = int(socialFriends)
					g.dmOverlay = true
					g.dmInputFocus = true
					g.guildChatFocus = false
					g.send("GetFriendHistory", protocol.GetFriendHistory{With: g.selectedFriend, Limit: 50})
					g.memberProfileOverlay = false
					g.profileFromFriends = false
					return
				}
				if ptIn(mx, my, image.Rect(x+14+84+110, baseY, x+14+84+110+96, baseY+24)) {
					g.send("RemoveFriend", protocol.RemoveFriend{Name: g.memberProfile.Name})
					return
				}
			}

			// Handle close button
			closeR := image.Rect(x+w-28, y+8, x+w-8, y+28)
			if ptIn(mx, my, closeR) {
				g.memberProfileOverlay = false
				g.profileFromFriends = false
				return
			}
		}

		// Handle transfer leader confirmation
		if g.transferLeaderConfirm {
			if g.handleTransferLeaderConfirmation(mx, my) {
				return
			}
		}
	}

	// If overlays are open, block other UI interactions
	if g.memberProfileOverlay || g.transferLeaderConfirm {
		return
	}

	// Load last selected social tab from disk once
	if !g.socialTabLoaded {
		g.socialTabLoaded = true
		if b, err := os.ReadFile(ConfigPath("social_tab.txt")); err == nil {
			if v, err2 := strconv.Atoi(strings.TrimSpace(string(b))); err2 == nil {
				if v == int(socialFriends) || v == int(socialGuild) || v == int(socialMessages) {
					g.socialTab = v
				}
			}
		}
	}

	// Load guild chat persistence once
	if !g.guildChatLoaded {
		g.guildChatLoaded = true
		// Load guild chat input
		if b, err := os.ReadFile(ConfigPath("guild_chat_input.txt")); err == nil {
			g.guildChatInput = strings.TrimSpace(string(b))
		}
		// Load guild chat scroll position
		if b, err := os.ReadFile(ConfigPath("guild_chat_scroll.txt")); err == nil {
			if v, err2 := strconv.Atoi(strings.TrimSpace(string(b))); err2 == nil {
				g.guildMembersScroll = v
			}
		}
	}

	// Build segmented control rects (Friends | Guild)
	const segW, segH = 220, 36
	segX := pad
	segY := topBarH + pad
	segFriends := image.Rect(segX, segY, segX+segW/2, segY+segH)
	segGuild := image.Rect(segX+segW/2, segY, segX+segW, segY+segH)
	// Top-right buttons when on Guild tab and in a guild - use logical coordinates
	leaveTopBtn := image.Rect(protocol.ScreenW-pad-120, segY+6, protocol.ScreenW-pad, segY+6+24)
	disbandTopBtn := image.Rect(protocol.ScreenW-pad-240, segY+6, protocol.ScreenW-pad-124, segY+6+24)

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		switch {
		case ptIn(mx, my, segFriends):
			g.socialTab = int(socialFriends)
			_ = os.WriteFile(ConfigPath("social_tab.txt"), []byte(fmt.Sprintf("%d", g.socialTab)), 0o644)
		case ptIn(mx, my, segGuild):
			g.socialTab = int(socialGuild)
			_ = os.WriteFile(ConfigPath("social_tab.txt"), []byte(fmt.Sprintf("%d", g.socialTab)), 0o644)
		}
		// Handle top-right buttons
		if g.socialActive() == socialGuild && strings.TrimSpace(g.guildID) != "" {
			if ptIn(mx, my, leaveTopBtn) {
				g.guildLeaveConfirm = true
			}
			// Disband button only for leader
			meRole := "member"
			for _, m := range g.guildMembers {
				if strings.EqualFold(m.Name, g.name) {
					meRole = strings.ToLower(m.Role)
				}
			}
			if meRole == "leader" && ptIn(mx, my, disbandTopBtn) {
				g.guildDisbandConfirm = true
			}
		}
	}

	// Confirm leave popup (simple inline confirmation near top)
	if g.guildLeaveConfirm {
		// Position confirmation under the top bar
		box := image.Rect(protocol.ScreenW-pad-360, segY+segH+8, protocol.ScreenW-pad-12, segY+segH+8+60)
		yes := image.Rect(box.Min.X+12, box.Min.Y+30, box.Min.X+12+64, box.Min.Y+30+22)
		no := image.Rect(box.Min.X+12+72, box.Min.Y+30, box.Min.X+12+72+64, box.Min.Y+30+22)
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			switch {
			case ptIn(mx, my, yes):
				// Restrict leader from leaving while members remain
				meRole := "member"
				others := 0
				for _, m := range g.guildMembers {
					if strings.EqualFold(m.Name, g.name) {
						meRole = strings.ToLower(m.Role)
						continue
					}
					others++
				}
				if meRole == "leader" && others > 0 {
					g.guildLeaveError = "Transfer leadership to another member first."
				} else {
					g.guildLeaveConfirm = false
					g.guildLeaveError = ""
					g.send("LeaveGuild", protocol.LeaveGuild{})
					g.send("GetGuild", protocol.GetGuild{})
				}
			case ptIn(mx, my, no):
				g.guildLeaveConfirm = false
				g.guildLeaveError = ""
			}
		}
		// When confirmation is open, skip other interactions
		return
	}

	if g.guildDisbandConfirm {
		// Confirm disband (leader only)
		box := image.Rect(protocol.ScreenW-pad-420, segY+segH+8, protocol.ScreenW-pad-12, segY+segH+8+80)
		yes := image.Rect(box.Min.X+12, box.Min.Y+46, box.Min.X+12+64, box.Min.Y+46+22)
		no := image.Rect(box.Min.X+12+72, box.Min.Y+46, box.Min.X+12+72+64, box.Min.Y+46+22)
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			switch {
			case ptIn(mx, my, yes):
				g.guildDisbandConfirm = false
				g.guildDisbandError = ""
				g.send("DisbandGuild", struct{}{})
			case ptIn(mx, my, no):
				g.guildDisbandConfirm = false
				g.guildDisbandError = ""
			}
		}
		return
	}

	// Simple interactions for Guild when not in a guild
	if g.socialActive() == socialGuild && strings.TrimSpace(g.guildID) == "" {
		// Match geometry with drawGuild
		contentY := segY + segH + 12
		// Guild name input box - updated position
		nameRect := image.Rect(pad+12, contentY+60, pad+12+260, contentY+60+24)
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.guildNameFocus = ptIn(mx, my, nameRect)
		}
		if g.guildNameFocus {
			for _, k := range inpututil.AppendJustPressedKeys(nil) {
				switch k {
				case ebiten.KeyBackspace:
					if len(g.guildCreateName) > 0 {
						g.guildCreateName = g.guildCreateName[:len(g.guildCreateName)-1]
					}
				case ebiten.KeyEnter:
					// no-op; Create button triggers
				}
			}
			for _, r := range ebiten.AppendInputChars(nil) {
				if r == '\n' || r == '\r' {
					continue
				}
				if r >= 32 {
					g.guildCreateName += string(r)
				}
			}
		}
		btnW, btnH := 160, 32
		create := image.Rect(pad+12, contentY+96, pad+12+btnW, contentY+96+btnH)
		join := image.Rect(pad+12+btnW+12, contentY+96, pad+12+btnW+12+btnW, contentY+96+btnH)
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			switch {
			case ptIn(mx, my, create):
				name := strings.TrimSpace(g.guildCreateName)
				if name == "" {
					base := g.name
					if strings.TrimSpace(base) == "" {
						base = "Player"
					}
					name = base + "'s Guild"
				}
				g.send("CreateGuild", protocol.CreateGuild{Name: name, Desc: "", Privacy: "public", Region: "NA"})
				g.guildCreateName = ""
				g.send("GetGuild", protocol.GetGuild{})
			case ptIn(mx, my, join):
				g.guildBrowse = true
				g.guildListScroll = 0
				g.send("ListGuilds", protocol.ListGuilds{Query: strings.TrimSpace(g.guildFilter)})
			}
		}
		// If browsing, allow clicking first few items to join
		if g.guildBrowse {
			// handle filter focus + typing - updated position
			filterRect := image.Rect(pad+12, contentY+140, pad+12+260, contentY+140+22)
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				g.guildFilterFocus = ptIn(mx, my, filterRect)
			}
			if g.guildFilterFocus {
				for _, k := range inpututil.AppendJustPressedKeys(nil) {
					switch k {
					case ebiten.KeyBackspace:
						if len(g.guildFilter) > 0 {
							g.guildFilter = g.guildFilter[:len(g.guildFilter)-1]
						}
					}
				}
				for _, r := range ebiten.AppendInputChars(nil) {
					if r >= 32 {
						g.guildFilter += string(r)
					}
				}
				// Throttle refreshes to avoid spamming the server
				q := strings.TrimSpace(g.guildFilter)
				if q != g.lastGuildQuery || time.Since(g.lastGuildListReq) > 800*time.Millisecond {
					g.send("ListGuilds", protocol.ListGuilds{Query: q})
					g.lastGuildListReq = time.Now()
					g.lastGuildQuery = q
				}
			}
			// wheel scroll
			_, wy := ebiten.Wheel()
			if wy != 0 {
				g.guildListScroll -= int(wy)
				if g.guildListScroll < 0 {
					g.guildListScroll = 0
				}
				maxStart := maxInt(0, len(g.guildList)-10)
				if g.guildListScroll > maxStart {
					g.guildListScroll = maxStart
				}
			}
			listTop := contentY + 180
			rowH := 22
			start := g.guildListScroll
			maxRows := 8 // Reduced to match drawing
			for i := 0; i < maxRows && start+i < len(g.guildList); i++ {
				y := listTop + i*rowH
				// Join button rect at right
				bx := pad + 12 + 420
				br := image.Rect(bx, y-12, bx+72, y+10)
				if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && ptIn(mx, my, br) {
					gid := g.guildList[start+i].GuildID
					g.send("JoinGuild", protocol.JoinGuild{GuildID: gid})
					g.guildBrowse = false
				}
			}
		}
	}

	// Guild actions when in a guild (new layout geometry)
	if g.socialActive() == socialGuild && strings.TrimSpace(g.guildID) != "" {
		// Extra guard: if our name is not in current roster, drop to browse
		meIn := false
		for _, m := range g.guildMembers {
			if strings.EqualFold(m.Name, g.name) {
				meIn = true
				break
			}
		}
		if !meIn {
			g.guildID = ""
			g.guildBrowse = true
			return
		}
		// Periodically refresh guild info for online status
		if time.Since(g.lastGuildInfoReq) > 5*time.Second {
			g.send("GetGuild", protocol.GetGuild{})
			g.lastGuildInfoReq = time.Now()
		}
		contentY := segY + segH + 12
		x := pad + 12
		fullW := protocol.ScreenW - 2*pad - 24
		availH := (protocol.ScreenH - menuBarH - contentY - pad)
		topH := availH/2 - 20
		if topH < 120 {
			topH = availH / 2
		}
		botH := availH - topH - 36

		// Precompute members list like drawGuild (including sort)
		members := append([]protocol.GuildMember(nil), g.guildMembers...)
		switch g.guildSortMode % 3 {
		case 0: // Name
			sort.Slice(members, func(i, j int) bool { return strings.ToLower(members[i].Name) < strings.ToLower(members[j].Name) })
		case 1: // Status
			sort.Slice(members, func(i, j int) bool {
				if members[i].Online != members[j].Online {
					return members[i].Online
				}
				return strings.ToLower(members[i].Name) < strings.ToLower(members[j].Name)
			})
		case 2: // Rank
			rank := func(r string) int {
				r = strings.ToLower(r)
				if r == "leader" {
					return 0
				}
				if r == "officer" {
					return 1
				}
				return 2
			}
			sort.Slice(members, func(i, j int) bool {
				ri, rj := rank(members[i].Role), rank(members[j].Role)
				if ri != rj {
					return ri < rj
				}
				return strings.ToLower(members[i].Name) < strings.ToLower(members[j].Name)
			})
		}
		rowsTop := contentY + 56 // keep in sync with drawGuild
		rowH := 18
		vis := (topH - (rowsTop - (contentY + 12))) / rowH
		if vis < 0 {
			vis = 0
		}
		if g.guildMembersScroll < 0 {
			g.guildMembersScroll = 0
		}
		maxStart := 0
		if len(members) > vis {
			maxStart = len(members) - vis
		}
		if g.guildMembersScroll > maxStart {
			g.guildMembersScroll = maxStart
		}

		// Rects used in interactions - match exact positions from drawGuild
		sortBtn := image.Rect(x+fullW-120, contentY+12, x+fullW-12, contentY+12+26)
		// Members panel area
		membersArea := image.Rect(x, contentY+12, x+fullW, contentY+12+topH)
		// Guild rewards button - exact positioning from drawGuild
		guildRewardsBtn := image.Rect(x+(fullW-160)/2, contentY+12+topH+6, x+(fullW-160)/2+160, contentY+12+topH+6+26)
		// Chat input area - exact positioning from drawGuild
		chatTop := contentY + 12 + topH + 36
		inputW := fullW - 180
		// leaveR removed (no bottom leave button)

		// Mouse interactions - check specific buttons first, then general areas
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Check specific buttons first (highest priority)
			sendBtnX := x + 8 + inputW + 6
			sendBtnY := chatTop + botH - 26
			sendBtnW := 70
			sendBtnH := 22
			sendBtnRect := image.Rect(sendBtnX, sendBtnY, sendBtnX+sendBtnW, sendBtnY+sendBtnH)

			// 1. Send button in chat
			if ptIn(mx, my, sendBtnRect) && strings.TrimSpace(g.guildChatInput) != "" {
				if time.Since(g.guildSendClickAt) > 120*time.Millisecond {
					g.send("GuildChatSend", protocol.GuildChatSend{Text: strings.TrimSpace(g.guildChatInput)})
					g.guildChatInput = ""
					g.guildSendClickAt = time.Now()
				}
				return
			}

			// 2. Guild Rewards button
			if ptIn(mx, my, guildRewardsBtn) {
				g.armyMsg = "Guild Rewards coming soon!"
				return
			}

			// 3. Sort button
			if ptIn(mx, my, sortBtn) {
				g.guildSortMode = (g.guildSortMode + 1) % 3
				g.guildMembersScroll = 0
				return
			}

			// 4. Check members panel (before general areas)
			if ptIn(mx, my, membersArea) {
				idx := (my - (rowsTop - 12)) / rowH
				if idx >= 0 {
					start := g.guildMembersScroll
					if start+idx >= 0 && start+idx < len(members) {
						name := members[start+idx].Name
						if strings.EqualFold(g.selectedGuildMember, name) {
							g.send("GetUserProfile", protocol.GetUserProfile{Name: name})
						}
						g.selectedGuildMember = name
					}
				}
				return
			}

			// 5. Chat panel focus (lowest priority - if nothing else was clicked)
			chatPanel := image.Rect(x, chatTop, x+fullW, chatTop+botH)
			if ptIn(mx, my, chatPanel) {
				g.guildChatFocus = true
			} else if !ptIn(mx, my, sendBtnRect) {
				g.guildChatFocus = false
			}

			// Click name in chat to open profile (first line of each wrapped message)
			maxRows := (botH - 32) / 16
			lines := g.visibleGuildChatLines(fullW-16, maxRows)
			for i, ln := range lines {
				if !ln.firstLine {
					continue
				}
				yy := chatTop + 14 + i*16
				// name appears after prefix with timestamp and a space
				pref := "[" + ln.ts + "] "
				px := text.BoundString(basicfont.Face7x13, pref).Dx()
				nameW := text.BoundString(basicfont.Face7x13, ln.from).Dx()
				nameRect := image.Rect(x+8+px, yy-12, x+8+px+nameW, yy+4)
				if ptIn(mx, my, nameRect) {
					g.send("GetUserProfile", protocol.GetUserProfile{Name: ln.from})
					break
				}
			}
		}

		// Scroll members list on wheel when hovering members area
		if _, wy := ebiten.Wheel(); wy != 0 {
			if ptIn(mx, my, membersArea) {
				g.guildMembersScroll -= int(wy)
				if g.guildMembersScroll < 0 {
					g.guildMembersScroll = 0
				}
				if len(members) > vis {
					maxStart = len(members) - vis
					if g.guildMembersScroll > maxStart {
						g.guildMembersScroll = maxStart
					}
				}
				// Save guild chat scroll position persistence
				_ = os.WriteFile(ConfigPath("guild_chat_scroll.txt"), []byte(fmt.Sprintf("%d", g.guildMembersScroll)), 0o644)
			}
		}

		// Keyboard input for chat when focused
		if g.guildChatFocus {
			for _, k := range inpututil.AppendJustPressedKeys(nil) {
				switch k {
				case ebiten.KeyEnter:
					if strings.TrimSpace(g.guildChatInput) != "" {
						g.send("GuildChatSend", protocol.GuildChatSend{Text: strings.TrimSpace(g.guildChatInput)})
						g.guildChatInput = ""
						g.chatBackspaceStart = time.Time{}
						g.chatBackspaceLast = time.Time{}
					}
				case ebiten.KeyBackspace:
					if len(g.guildChatInput) > 0 {
						g.guildChatInput = g.guildChatInput[:len(g.guildChatInput)-1]
					}
					if g.chatBackspaceStart.IsZero() {
						g.chatBackspaceStart = time.Now()
						g.chatBackspaceLast = g.chatBackspaceStart
					}
				}
			}
			// Backspace repeat when held
			if ebiten.IsKeyPressed(ebiten.KeyBackspace) && len(g.guildChatInput) > 0 {
				now := time.Now()
				if g.chatBackspaceStart.IsZero() {
					g.chatBackspaceStart = now
					g.chatBackspaceLast = now
				}
				// initial delay 300ms, then every 45ms
				if now.Sub(g.chatBackspaceStart) > 300*time.Millisecond && now.Sub(g.chatBackspaceLast) > 45*time.Millisecond {
					g.guildChatInput = g.guildChatInput[:len(g.guildChatInput)-1]
					g.chatBackspaceLast = now
				}
			} else {
				// reset timers when key not pressed
				g.chatBackspaceStart = time.Time{}
				g.chatBackspaceLast = time.Time{}
			}
			for _, r := range ebiten.AppendInputChars(nil) {
				if r == '\n' || r == '\r' {
					continue
				}
				if r >= 32 {
					g.guildChatInput += string(r)
					// Save guild chat input persistence
					_ = os.WriteFile(ConfigPath("guild_chat_input.txt"), []byte(g.guildChatInput), 0o644)
				}
			}
		}
	}

	// Friends: search/add, guild-like styled list with sort + scroll
	if g.socialActive() == socialFriends {
		// Geometry consistent with drawSocial/drawFriends
		contentY := (topBarH + pad) + 36 + 12 // segY + segH + 12
		x := pad + 12
		fullW := protocol.ScreenW - 2*pad - 24

		// Compute content height like drawSocial and position inputs at bottom
		contentH := protocol.ScreenH - menuBarH - contentY - pad
		inputY := contentY + contentH - 30
		// Search field and add button rects (bottom-aligned)
		searchRect := image.Rect(pad+12, inputY, pad+12+260, inputY+24)
		addBtn := image.Rect(pad+12+270, inputY, pad+12+270+70, inputY+24)

		// Build sorted friends list (Name or Status)
		friends := append([]protocol.FriendInfo(nil), g.friends...)
		switch g.friendSortMode % 2 {
		case 0: // Name
			sort.Slice(friends, func(i, j int) bool { return strings.ToLower(friends[i].Name) < strings.ToLower(friends[j].Name) })
		case 1: // Status (online first), then name
			sort.Slice(friends, func(i, j int) bool {
				if friends[i].Online != friends[j].Online {
					return friends[i].Online
				}
				return strings.ToLower(friends[i].Name) < strings.ToLower(friends[j].Name)
			})
		}

		// List geometry (styled like guild members)
		listTop := contentY + 56
		rowH := 18
		// Visible rows based on content area height, reserving space for bottom inputs
		listBottom := inputY - 8
		usableH := listBottom - listTop
		vis := usableH / rowH
		if vis < 0 {
			vis = 0
		}
		if g.friendScroll < 0 {
			g.friendScroll = 0
		}
		maxStart := 0
		if len(friends) > vis {
			maxStart = len(friends) - vis
		}
		if g.friendScroll > maxStart {
			g.friendScroll = maxStart
		}

		// Sort button and list area for input targeting
		sortBtn := image.Rect(x+fullW-90, contentY+16, x+fullW-12, contentY+16+22)
		listArea := image.Rect(x+6, listTop-12, x+fullW-6, listTop-12+vis*rowH)

		// Mouse interactions
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Focus search
			g.friendSearchFocus = ptIn(mx, my, searchRect)
			if g.friendSearchFocus {
				g.friendSearchError = ""
			}
			// Add friend
			if ptIn(mx, my, addBtn) && strings.TrimSpace(g.friendSearch) != "" {
				name := strings.TrimSpace(g.friendSearch)
				g.friendAddLookup = name
				g.friendSearchError = ""
				g.send("GetUserProfile", protocol.GetUserProfile{Name: name})
				// keep text; we will clear on success in handler
			}
			// Toggle sort
			if ptIn(mx, my, sortBtn) {
				g.friendSortMode = (g.friendSortMode + 1) % 2
				g.friendScroll = 0
			}
			// Click a friend row
			if ptIn(mx, my, listArea) {
				idx := (my - (listTop - 12)) / rowH
				if idx >= 0 {
					start := g.friendScroll
					if start+idx >= 0 && start+idx < len(friends) {
						name := friends[start+idx].Name
						g.selectedFriend = name
						// Open profile overlay directly (matches previous behavior)
						g.profileFromFriends = true
						g.send("GetUserProfile", protocol.GetUserProfile{Name: name})
					}
				}
			}
		}

		// Wheel scroll over list
		if _, wy := ebiten.Wheel(); wy != 0 {
			if ptIn(mx, my, listArea) {
				g.friendScroll -= int(wy)
				if g.friendScroll < 0 {
					g.friendScroll = 0
				}
				if len(friends) > vis {
					maxStart = len(friends) - vis
					if g.friendScroll > maxStart {
						g.friendScroll = maxStart
					}
				}
			}
		}

		// Type into search only when focused
		if g.friendSearchFocus {
			for _, k := range inpututil.AppendJustPressedKeys(nil) {
				if k == ebiten.KeyBackspace && len(g.friendSearch) > 0 {
					g.friendSearch = g.friendSearch[:len(g.friendSearch)-1]
				}
			}
			for _, r := range ebiten.AppendInputChars(nil) {
				if r >= 32 {
					g.friendSearch += string(r)
				}
			}
		}

		// Periodic refresh of friends list for online status
		if time.Since(g.lastFriendsReq) > 5*time.Second {
			g.send("GetFriends", protocol.GetFriends{})
			g.lastFriendsReq = time.Now()
		}
	}

	// DM overlay input handling (since Messages tab is removed)
	if g.dmOverlay && g.selectedFriend != "" {
		// match geometry with draw overlay
		w, h := 420, 300
		x := (protocol.ScreenW - w) / 2
		y := (protocol.ScreenH - h) / 2
		closeR := image.Rect(x+w-28, y+8, x+w-8, y+28)
		btnW := 70
		inputRect := image.Rect(x+12, y+h-30, x+w-12-(btnW+6), y+h-8)
		sendRect := image.Rect(inputRect.Max.X+6, y+h-30, inputRect.Max.X+6+btnW, y+h-8)
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			mx, my := ebiten.CursorPosition()
			if ptIn(mx, my, closeR) {
				g.dmOverlay = false
			}
			if ptIn(mx, my, sendRect) && strings.TrimSpace(g.dmInput) != "" {
				g.send("SendFriendDM", protocol.SendFriendDM{To: g.selectedFriend, Text: g.dmInput})
				g.dmInput = ""
				g.send("GetFriendHistory", protocol.GetFriendHistory{With: g.selectedFriend, Limit: 50})
			} else {
				g.dmInputFocus = ptIn(mx, my, inputRect)
			}
		}
		if g.dmInputFocus {
			for _, k := range inpututil.AppendJustPressedKeys(nil) {
				if k == ebiten.KeyEnter && strings.TrimSpace(g.dmInput) != "" {
					g.send("SendFriendDM", protocol.SendFriendDM{To: g.selectedFriend, Text: g.dmInput})
					g.dmInput = ""
					g.send("GetFriendHistory", protocol.GetFriendHistory{With: g.selectedFriend, Limit: 50})
				} else if k == ebiten.KeyBackspace && len(g.dmInput) > 0 {
					g.dmInput = g.dmInput[:len(g.dmInput)-1]
				}
			}
			for _, r := range ebiten.AppendInputChars(nil) {
				if r == '\n' || r == '\r' {
					continue
				}
				if r >= 32 {
					g.dmInput += string(r)
				}
			}
		}
	}
}

// DrawSocial renders the Social UI
func (g *Game) drawSocial(screen *ebiten.Image) {
	// Segmented control header (Friends | Guild)
	const segW, segH = 220, 36
	segX := pad
	segY := topBarH + pad

	// Draw themed segmented control background
	g.fantasyUI.DrawThemedPanel(screen, segX, segY, segW, segH, 0.8)

	// Draw themed segment buttons
	friendsActive := g.socialActive() == socialFriends
	guildActive := g.socialActive() == socialGuild

	g.fantasyUI.DrawThemedButtonWithStyle(screen, segX, segY, segW/2, segH, "Friends",
		func() ButtonState {
			if friendsActive {
				return ButtonPressed
			}
			return ButtonNormal
		}(), true) // Use grey colors like bottom bar buttons

	g.fantasyUI.DrawThemedButtonWithStyle(screen, segX+segW/2, segY, segW/2, segH, "Guild",
		func() ButtonState {
			if guildActive {
				return ButtonPressed
			}
			return ButtonNormal
		}(), true) // Use grey colors like bottom bar buttons

	// Top-right Leave Guild button (only on Guild tab with a guild)
	if g.socialActive() == socialGuild && strings.TrimSpace(g.guildID) != "" {
		leaveTopBtn := image.Rect(protocol.ScreenW-pad-120, segY+6, protocol.ScreenW-pad, segY+6+24)
		g.fantasyUI.DrawThemedButton(screen, leaveTopBtn.Min.X, leaveTopBtn.Min.Y, leaveTopBtn.Dx(), leaveTopBtn.Dy(), "Leave Guild", ButtonNormal)
		// Disband button for leader only
		meRole := "member"
		for _, m := range g.guildMembers {
			if strings.EqualFold(m.Name, g.name) {
				meRole = strings.ToLower(m.Role)
			}
		}
		if meRole == "leader" {
			disbandTopBtn := image.Rect(protocol.ScreenW-pad-240, segY+6, protocol.ScreenW-pad-124, segY+6+24)
			g.fantasyUI.DrawThemedButton(screen, disbandTopBtn.Min.X, disbandTopBtn.Min.Y, disbandTopBtn.Dx(), disbandTopBtn.Dy(), "Disband", ButtonNormal)
		}
	}

	// Content area box with themed panel
	contentY := segY + segH + 12
	contentH := protocol.ScreenH - menuBarH - contentY - pad
	g.fantasyUI.DrawThemedPanel(screen, pad, contentY, protocol.ScreenW-2*pad, contentH, 0.9)

	switch g.socialActive() {
	case socialFriends:
		g.drawFriends(screen, contentY)
	case socialGuild:
		g.drawGuild(screen, contentY, contentH)
	}

	// Inline leave confirmation popup
	if g.socialActive() == socialGuild && g.guildLeaveConfirm {
		box := image.Rect(protocol.ScreenW-pad-360, segY+segH+8, protocol.ScreenW-pad-12, segY+segH+8+60)
		ebitenutil.DrawRect(screen, float64(box.Min.X), float64(box.Min.Y), float64(box.Dx()), float64(box.Dy()), color.NRGBA{30, 30, 45, 240})
		prompt := "Leave guild " + strings.TrimSpace(g.guildName) + "?"
		text.Draw(screen, prompt, basicfont.Face7x13, box.Min.X+12, box.Min.Y+20, color.White)
		if g.guildLeaveError != "" {
			text.Draw(screen, g.guildLeaveError, basicfont.Face7x13, box.Min.X+12, box.Min.Y+36, color.NRGBA{240, 120, 120, 255})
		}
		yes := image.Rect(box.Min.X+12, box.Min.Y+30, box.Min.X+12+64, box.Min.Y+30+22)
		no := image.Rect(box.Min.X+12+72, box.Min.Y+30, box.Min.X+12+72+64, box.Min.Y+30+22)
		ebitenutil.DrawRect(screen, float64(yes.Min.X), float64(yes.Min.Y), float64(yes.Dx()), float64(yes.Dy()), color.NRGBA{70, 110, 70, 255})
		ebitenutil.DrawRect(screen, float64(no.Min.X), float64(no.Min.Y), float64(no.Dx()), float64(no.Dy()), color.NRGBA{90, 70, 70, 255})
		text.Draw(screen, "Yes", basicfont.Face7x13, yes.Min.X+16, yes.Min.Y+16, color.White)
		text.Draw(screen, "No", basicfont.Face7x13, no.Min.X+20, no.Min.Y+16, color.White)
	}

	if g.socialActive() == socialGuild && g.guildDisbandConfirm {
		box := image.Rect(protocol.ScreenW-pad-420, segY+segH+8, protocol.ScreenW-pad-12, segY+segH+8+80)
		ebitenutil.DrawRect(screen, float64(box.Min.X), float64(box.Min.Y), float64(box.Dx()), float64(box.Dy()), color.NRGBA{30, 30, 45, 240})
		prompt := "Disband guild " + strings.TrimSpace(g.guildName) + "? This cannot be undone."
		text.Draw(screen, prompt, basicfont.Face7x13, box.Min.X+12, box.Min.Y+20, color.White)
		if g.guildDisbandError != "" {
			text.Draw(screen, g.guildDisbandError, basicfont.Face7x13, box.Min.X+12, box.Min.Y+36, color.NRGBA{240, 120, 120, 255})
		}
		yes := image.Rect(box.Min.X+12, box.Min.Y+46, box.Min.X+12+64, box.Min.Y+46+22)
		no := image.Rect(box.Min.X+12+72, box.Min.Y+46, box.Min.X+12+72+64, box.Min.Y+46+22)
		ebitenutil.DrawRect(screen, float64(yes.Min.X), float64(yes.Min.Y), float64(yes.Dx()), float64(yes.Dy()), color.NRGBA{140, 50, 50, 255})
		ebitenutil.DrawRect(screen, float64(no.Min.X), float64(no.Min.Y), float64(no.Dx()), float64(no.Dy()), color.NRGBA{60, 60, 80, 255})
		text.Draw(screen, "Disband", basicfont.Face7x13, yes.Min.X+6, yes.Min.Y+16, color.White)
		text.Draw(screen, "Cancel", basicfont.Face7x13, no.Min.X+8, no.Min.Y+16, color.White)
	}

	// DM overlay popup
	if g.dmOverlay && g.selectedFriend != "" {
		// backdrop
		ebitenutil.DrawRect(screen, 0, 0, float64(protocol.ScreenW), float64(protocol.ScreenH), color.NRGBA{0, 0, 0, 120})
		// dialog
		w, h := 420, 300
		x := (protocol.ScreenW - w) / 2
		y := (protocol.ScreenH - h) / 2
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.NRGBA{30, 34, 50, 245})
		// title
		text.Draw(screen, "Chat: "+g.selectedFriend, basicfont.Face7x13, x+14, y+22, color.White)
		// close X
		closeR := image.Rect(x+w-28, y+8, x+w-8, y+28)
		ebitenutil.DrawRect(screen, float64(closeR.Min.X), float64(closeR.Min.Y), float64(closeR.Dx()), float64(closeR.Dy()), color.NRGBA{60, 60, 80, 255})
		text.Draw(screen, "X", basicfont.Face7x13, closeR.Min.X+6, closeR.Min.Y+14, color.White)
		// history area
		padIn := 12
		panelTop := y + 34
		panelH := h - 34 - 40
		ebitenutil.DrawRect(screen, float64(x+padIn), float64(panelTop), float64(w-2*padIn), float64(panelH), color.NRGBA{0x24, 0x24, 0x30, 0xFF})
		// messages (last N)
		rowH := 16
		maxRows := (panelH - 10) / rowH
		start := 0
		if len(g.dmHistory) > maxRows {
			start = len(g.dmHistory) - maxRows
		}
		for i := start; i < len(g.dmHistory); i++ {
			line := g.dmHistory[i].From + ": " + g.dmHistory[i].Text
			text.Draw(screen, line, basicfont.Face7x13, x+padIn+6, panelTop+16+(i-start)*rowH, color.White)
		}
		// input bar + send button
		btnW := 70
		inputW := (w - 2*padIn) - (btnW + 6)
		inputR := image.Rect(x+padIn, y+h-30, x+padIn+inputW, y+h-8)
		sendR := image.Rect(inputR.Max.X+6, y+h-30, inputR.Max.X+6+btnW, y+h-8)
		ebitenutil.DrawRect(screen, float64(inputR.Min.X), float64(inputR.Min.Y), float64(inputR.Dx()), float64(inputR.Dy()), color.NRGBA{24, 28, 40, 220})
		text.Draw(screen, g.dmInput, basicfont.Face7x13, inputR.Min.X+6, inputR.Min.Y+14, color.White)
		ebitenutil.DrawRect(screen, float64(sendR.Min.X), float64(sendR.Min.Y), float64(sendR.Dx()), float64(sendR.Dy()), color.NRGBA{70, 110, 70, 255})
		text.Draw(screen, "Send", basicfont.Face7x13, sendR.Min.X+18, sendR.Min.Y+16, color.White)
		// handle clicks
		mx, my := g.logicalCursor()
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if ptIn(mx, my, closeR) {
				g.dmOverlay = false
			} else if ptIn(mx, my, sendR) && strings.TrimSpace(g.dmInput) != "" {
				g.send("SendFriendDM", protocol.SendFriendDM{To: g.selectedFriend, Text: g.dmInput})
				g.dmInput = ""
				g.send("GetFriendHistory", protocol.GetFriendHistory{With: g.selectedFriend, Limit: 50})
			}
		}
	}

	// Member profile overlay popup
	if g.memberProfileOverlay {
		ebitenutil.DrawRect(screen, 0, 0, float64(protocol.ScreenW), float64(protocol.ScreenH), color.NRGBA{0, 0, 0, 120})
		w, h := 520, 380
		x := (protocol.ScreenW - w) / 2
		y := (protocol.ScreenH - h) / 2
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.NRGBA{30, 34, 50, 245})
		// close X
		closeR := image.Rect(x+w-28, y+8, x+w-8, y+28)
		ebitenutil.DrawRect(screen, float64(closeR.Min.X), float64(closeR.Min.Y), float64(closeR.Dx()), float64(closeR.Dy()), color.NRGBA{60, 60, 80, 255})
		text.Draw(screen, "X", basicfont.Face7x13, closeR.Min.X+6, closeR.Min.Y+14, color.White)
		// avatar (left)
		if img := g.ensureAvatarImage(g.memberProfile.Avatar); img != nil {
			aw, ah := img.Bounds().Dx(), img.Bounds().Dy()
			side := 72
			sx := float64(side) / float64(aw)
			sy := float64(side) / float64(ah)
			s := sx
			if sy < s {
				s = sy
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			// center inside a square area
			ax := float64(x + 14)
			ay := float64(y + 36)
			// adjust to center
			ax += (float64(side) - float64(aw)*s) / 2
			ay += (float64(side) - float64(ah)*s) / 2
			op.GeoM.Translate(ax, ay)
			screen.DrawImage(img, op)
		}
		text.Draw(screen, g.memberProfile.Name, basicfont.Face7x13, x+14+84, y+48, color.White)
		text.Draw(screen, fmt.Sprintf("PvP: %d (%s)", g.memberProfile.PvPRating, g.memberProfile.PvPRank), basicfont.Face7x13, x+14+84, y+68, color.White)
		if strings.TrimSpace(g.guildID) != "" {
			text.Draw(screen, "Guild: "+strings.TrimSpace(g.guildName), basicfont.Face7x13, x+14+84, y+88, color.White)
		}

		// Display equipped army as visual grid with average cost and level
		if len(g.memberProfile.Army) > 0 {
			// Add centered "Current Army:" header
			headerY := y + 116
			headerText := "Current Army:"
			headerWidth := text.BoundString(basicfont.Face7x13, headerText).Dx()
			headerX := x + (w-headerWidth)/2
			text.Draw(screen, headerText, basicfont.Face7x13, headerX, headerY, color.NRGBA{220, 220, 230, 255})

			// Move army down to accommodate header
			armyTop := y + 130

			// Calculate average cost and level
			totalCost := 0
			levelSum := 0
			unitCount := 0
			costMap := map[string]int{}
			for _, mini := range g.minisAll {
				costMap[mini.Name] = mini.Cost
			}

			xpLevels := []int{0, 2, 5, 10, 20, 35, 65, 120, 210, 375, 675, 1200, 2100, 3750, 6500, 12000, 25000, 50000, 100000, 200000}

			for _, unitName := range g.memberProfile.Army {
				if cost, ok := costMap[unitName]; ok {
					totalCost += cost
					unitCount++
				}
				if xp, exists := g.memberProfile.UnitXP[unitName]; exists {
					level := 0
					for i, reqXP := range xpLevels {
						if xp >= reqXP {
							level = i
						} else {
							break
						}
					}
					levelSum += level
				}
			}

			if unitCount > 0 {
				avgCost := float64(totalCost) / float64(unitCount)
				avgLevel := float64(levelSum) / float64(unitCount)

				// Center the champion and mini units horizontally in the overlay
				championSize := 80 // Smaller champion to fit better
				miniSize := 50

				// Calculate total width needed: champion + spacing + mini grid
				miniGridWidth := 3*miniSize + 2*2               // 3 units + 2 spacing gaps
				totalWidth := championSize + 30 + miniGridWidth // champion + spacing + grid

				// Center everything horizontally in the overlay
				startX := x + (w-totalWidth)/2

				// Draw champion unit (first in army)
				championLeft := startX
				if len(g.memberProfile.Army) > 0 {
					if img := g.ensureMiniImageByName(g.memberProfile.Army[0]); img != nil {
						scale := float64(championSize) / math.Max(float64(img.Bounds().Dx()), float64(img.Bounds().Dy()))
						op := &ebiten.DrawImageOptions{}
						if img.Bounds().Dx() > img.Bounds().Dy() {
							scale = float64(championSize) / float64(img.Bounds().Dx())
						} else {
							scale = float64(championSize) / float64(img.Bounds().Dy())
						}
						op.GeoM.Scale(scale, scale)
						op.GeoM.Translate(float64(championLeft), float64(armyTop))
						screen.DrawImage(img, op)
					}
				}

				// Draw mini units 3x2 grid (units 2-7)
				miniLeft := startX + championSize + 30 // After champion + spacing
				for i := 1; i < len(g.memberProfile.Army) && i < 7; i++ {
					row := (i - 1) / 3
					col := (i - 1) % 3
					mx := miniLeft + col*(miniSize+2)
					my := armyTop + row*(miniSize+2)

					img := g.ensureMiniImageByName(g.memberProfile.Army[i])
					if img != nil {
						scale := float64(miniSize) / math.Max(float64(img.Bounds().Dx()), float64(img.Bounds().Dy()))
						op := &ebiten.DrawImageOptions{}
						if img.Bounds().Dx() > img.Bounds().Dy() {
							scale = float64(miniSize) / float64(img.Bounds().Dx())
						} else {
							scale = float64(miniSize) / float64(img.Bounds().Dy())
						}
						op.GeoM.Scale(scale, scale)
						op.GeoM.Translate(float64(mx), float64(my))
						screen.DrawImage(img, op)
					}
				}

				// Display averages under the unit grid (well below portraits)
				championBottom := armyTop + championSize
				minisBottom := armyTop + (miniSize+2)*2
				statsTop := championBottom
				if minisBottom > championBottom {
					statsTop = minisBottom
				}
				statsTop += 15 // Extra spacing below the grid

				// Center both texts within the centered area
				centerX := startX + totalWidth/2
				costText := fmt.Sprintf("Avg Cost: %.1f gold", avgCost)
				levelText := fmt.Sprintf("Avg Level: %.1f", avgLevel)
				costWidth := text.BoundString(basicfont.Face7x13, costText).Dx()
				levelWidth := text.BoundString(basicfont.Face7x13, levelText).Dx()

				// Position cost text left, level text right, evenly spaced
				costX := centerX - (costWidth+levelWidth+10)/2
				levelX := costX + costWidth + 10

				text.Draw(screen, costText, basicfont.Face7x13, costX, statsTop, color.NRGBA{240, 196, 25, 255})
				text.Draw(screen, levelText, basicfont.Face7x13, levelX, statsTop, color.NRGBA{120, 180, 255, 255})
			}
		}

		// Admin actions (leader/officer) and Unfriend
		meRole := "member"
		selRole := "member"
		for _, m := range g.guildMembers {
			if strings.EqualFold(m.Name, g.name) {
				meRole = strings.ToLower(m.Role)
			}
			if strings.EqualFold(m.Name, g.memberProfile.Name) {
				selRole = strings.ToLower(m.Role)
			}
		}
		// Move action buttons 20% north
		isSelf := strings.EqualFold(g.memberProfile.Name, g.name)
		canKick := false
		canPromote := false
		canDemote := false
		canTransfer := false
		isFriend := false
		for _, fr := range g.friends {
			if strings.EqualFold(fr.Name, g.memberProfile.Name) {
				isFriend = true
				break
			}
		}
		if meRole == "leader" {
			canKick = !isSelf
			canPromote = !isSelf
			canDemote = !isSelf
			canTransfer = !isSelf
		} else if meRole == "officer" {
			// officers manage members only
			if selRole == "member" {
				canKick = !isSelf
				canPromote = true
				canDemote = true
			}
		}

		// Draw buttons below statistics - ensure no overlap
		actionsTop := y + int(float64(h)*0.85) - 64
		if !g.profileFromFriends {
			if canPromote {
				g.fantasyUI.DrawThemedButtonWithStyle(screen, x+14+84, actionsTop, 96, 24,
					"Promote", ButtonNormal, true)
			}
			if canDemote {
				g.fantasyUI.DrawThemedButtonWithStyle(screen, x+14+84+110, actionsTop, 96, 24,
					"Demote", ButtonNormal, true)
			}
			if canKick {
				g.fantasyUI.DrawThemedButtonWithStyle(screen, x+14+84, actionsTop+30, 96, 24,
					"Kick", ButtonNormal, true)
			}
			if canTransfer {
				g.fantasyUI.DrawThemedButtonWithStyle(screen, x+14+84+110, actionsTop+30, 96, 24,
					"Make Leader", ButtonNormal, true)
			}
		}
		// Unfriend button if viewing a friend (not self)
		if isFriend && !isSelf {
			baseY := actionsTop
			if !g.profileFromFriends {
				baseY = actionsTop + 60
			}
			g.fantasyUI.DrawThemedButtonWithStyle(screen, x+14+84, baseY, 96, 24,
				"Message", ButtonNormal, true)
			g.fantasyUI.DrawThemedButtonWithStyle(screen, x+14+84+110, baseY, 96, 24,
				"Unfriend", ButtonNormal, true)
		}

		// Transfer leader confirm popup
		if g.transferLeaderConfirm && g.transferLeaderTarget != "" {
			box := image.Rect(x+40, y+h-98, x+w-40, y+h-42)
			ebitenutil.DrawRect(screen, float64(box.Min.X), float64(box.Min.Y), float64(box.Dx()), float64(box.Dy()), color.NRGBA{30, 30, 45, 240})
			msg := "Give leadership to " + g.transferLeaderTarget + "?"
			text.Draw(screen, msg, basicfont.Face7x13, box.Min.X+10, box.Min.Y+18, color.White)
			yes := image.Rect(box.Min.X+10, box.Min.Y+28, box.Min.X+10+64, box.Min.Y+28+22)
			no := image.Rect(box.Min.X+84, box.Min.Y+28, box.Min.X+84+64, box.Min.Y+28+22)
			ebitenutil.DrawRect(screen, float64(yes.Min.X), float64(yes.Min.Y), float64(yes.Dx()), float64(yes.Dy()), color.NRGBA{70, 110, 70, 255})
			ebitenutil.DrawRect(screen, float64(no.Min.X), float64(no.Min.Y), float64(no.Dx()), float64(no.Dy()), color.NRGBA{90, 70, 70, 255})
			text.Draw(screen, "Yes", basicfont.Face7x13, yes.Min.X+16, yes.Min.Y+16, color.White)
			text.Draw(screen, "No", basicfont.Face7x13, no.Min.X+20, no.Min.Y+16, color.White)
			// Move mouse handling to updateSocial - click handling doesn't belong in draw
		}
	}
}

// drawGuild renders the Guild section. Rewards are present but disabled (grayed out).
func (g *Game) drawGuild(screen *ebiten.Image, y0, h int) {
	// If no guild membership, show Join/Create panel
	if strings.TrimSpace(g.guildID) == "" {
		text.Draw(screen, "No guild yet", basicfont.Face7x13, pad+12, y0+24, color.White)
		text.Draw(screen, "Create or Join a guild to play with others.", basicfont.Face7x13, pad+12, y0+44, color.NRGBA{200, 200, 210, 255})

		// Guild name input with themed styling - moved down to avoid overlap
		g.fantasyUI.DrawThemedPanel(screen, pad+12, y0+60, 260, 24, 0.9)
		name := g.guildCreateName
		if name == "" && !g.guildNameFocus {
			name = "Enter guild name..."
		}
		text.Draw(screen, name, basicfont.Face7x13, pad+18, y0+76, color.White)

		// Buttons with themed styling - moved down
		btnW, btnH := 160, 32
		create := rect{x: pad + 12, y: y0 + 96, w: btnW, h: btnH}
		join := rect{x: pad + 12 + btnW + 12, y: y0 + 96, w: btnW, h: btnH}
		g.fantasyUI.DrawThemedButton(screen, create.x, create.y, create.w, create.h, "Create Guild", ButtonNormal)
		g.fantasyUI.DrawThemedButton(screen, join.x, join.y, join.w, join.h, "Join Guild", ButtonNormal)

		// Browse list (optional) - moved down further
		if g.guildBrowse {
			// filter box with themed styling
			g.fantasyUI.DrawThemedPanel(screen, pad+12, y0+140, 260, 22, 0.9)
			ftxt := g.guildFilter
			if ftxt == "" && !g.guildFilterFocus {
				ftxt = "Filter..."
			}
			text.Draw(screen, ftxt, basicfont.Face7x13, pad+18, y0+156, color.White)
			listTop := y0 + 190
			rowH := 22
			start := g.guildListScroll
			maxRows := 8 // Reduced to fit better
			for i := 0; i < maxRows && start+i < len(g.guildList); i++ {
				yy := listTop + i*rowH
				it := g.guildList[start+i]
				line := fmt.Sprintf("%s  (%d/25)", it.Name, it.MembersCount)
				if i%2 == 0 {
					ebitenutil.DrawRect(screen, float64(pad+12), float64(yy-14), float64(480), 20, color.NRGBA{0x2c, 0x2c, 0x3c, 0xFF})
				}
				text.Draw(screen, line, basicfont.Face7x13, pad+18, yy, color.White)
				// Join button with themed styling - aligned exactly with row background
				jb := image.Rect(pad+12+420, yy-14, pad+12+420+72, yy+6)
				g.fantasyUI.DrawThemedButton(screen, jb.Min.X, jb.Min.Y, jb.Dx(), jb.Dy(), "Join", ButtonNormal)
			}
		}
		return
	}

	// New layout: top members, center rewards button, bottom chat
	x := pad + 12
	fullW := protocol.ScreenW - 2*pad - 24
	availH := h - 24
	topH := availH/2 - 20
	if topH < 120 {
		topH = availH / 2
	}
	botH := availH - topH - 36

	// Members box with themed panel
	g.fantasyUI.DrawThemedPanel(screen, x, y0+12, fullW, topH, 0.9)
	// Header with count and sort
	text.Draw(screen, fmt.Sprintf("Members %d/25", len(g.guildMembers)), basicfont.Face7x13, x+8, y0+28, color.White)
	sortBtn := image.Rect(x+fullW-90, y0+16, x+fullW-12, y0+16+22)
	sortLabel := []string{"Name", "Status", "Rank"}
	g.fantasyUI.DrawThemedButton(screen, sortBtn.Min.X, sortBtn.Min.Y, sortBtn.Dx(), sortBtn.Dy(), "Sort: "+sortLabel[g.guildSortMode%3], ButtonNormal)

	// Sorted members and rows
	members := append([]protocol.GuildMember(nil), g.guildMembers...)
	switch g.guildSortMode % 3 {
	case 0:
		sort.Slice(members, func(i, j int) bool { return strings.ToLower(members[i].Name) < strings.ToLower(members[j].Name) })
	case 1:
		sort.Slice(members, func(i, j int) bool {
			if members[i].Online != members[j].Online {
				return members[i].Online
			}
			return strings.ToLower(members[i].Name) < strings.ToLower(members[j].Name)
		})
	case 2:
		rank := func(r string) int {
			r = strings.ToLower(r)
			if r == "leader" {
				return 0
			}
			if r == "officer" {
				return 1
			}
			return 2
		}
		sort.Slice(members, func(i, j int) bool {
			ri, rj := rank(members[i].Role), rank(members[j].Role)
			if ri != rj {
				return ri < rj
			}
			return strings.ToLower(members[i].Name) < strings.ToLower(members[j].Name)
		})
	}
	rowsTop := y0 + 56
	rowH := 18
	vis := (topH - (rowsTop - (y0 + 12))) / rowH
	if vis < 0 {
		vis = 0
	}
	start := g.guildMembersScroll
	if start < 0 {
		start = 0
	}
	maxStart := 0
	if len(members) > vis {
		maxStart = len(members) - vis
	}
	if g.guildMembersScroll > maxStart {
		g.guildMembersScroll = maxStart
		start = g.guildMembersScroll
	}
	for i := 0; i < vis && start+i < len(members); i++ {
		m := members[start+i]
		yy := rowsTop + i*rowH
		bg := color.NRGBA{0x2c, 0x2c, 0x3c, 0xFF}
		lr := strings.ToLower(m.Role)
		if lr == "leader" {
			bg = color.NRGBA{40, 46, 70, 255}
		} else if lr == "officer" {
			bg = color.NRGBA{36, 56, 48, 255}
		}
		ebitenutil.DrawRect(screen, float64(x+6), float64(yy-12), float64(fullW-12), 16, bg)
		text.Draw(screen, m.Name+" ("+m.Role+")", basicfont.Face7x13, x+12, yy, color.White)
		dx := float32(x + fullW - 18)
		dy := float32(yy - 6)
		dc := color.NRGBA{220, 60, 60, 255}
		if m.Online {
			dc = color.NRGBA{60, 200, 80, 255}
		}
		vector.DrawFilledCircle(screen, dx, dy, 4, dc, true)
	}

	// Center rewards button with themed styling
	rb := image.Rect(x+(fullW-160)/2, y0+12+topH+6, x+(fullW-160)/2+160, y0+12+topH+6+26)
	g.fantasyUI.DrawThemedButton(screen, rb.Min.X, rb.Min.Y, rb.Dx(), rb.Dy(), "Guild Rewards", ButtonNormal)

	// Chat bottom with themed panel
	chatTop := y0 + 12 + topH + 36
	g.fantasyUI.DrawThemedPanel(screen, x, chatTop, fullW, botH, 0.9)
	// Wrap messages to fit width and render last N lines
	maxRows := (botH - 32) / 16
	lines := g.visibleGuildChatLines(fullW-16, maxRows)
	nameCol := color.NRGBA{120, 180, 255, 255}
	for i, ln := range lines {
		y := chatTop + 14 + i*16
		if ln.firstLine && !ln.system {
			// draw prefix + name + rest with different colors
			prefix := "[" + ln.ts + "] "
			px := x + 8
			text.Draw(screen, prefix, basicfont.Face7x13, px, y, color.White)
			px += text.BoundString(basicfont.Face7x13, prefix).Dx()
			text.Draw(screen, ln.from, basicfont.Face7x13, px, y, nameCol)
			px += text.BoundString(basicfont.Face7x13, ln.from).Dx()
			text.Draw(screen, ": ", basicfont.Face7x13, px, y, color.White)
			px += text.BoundString(basicfont.Face7x13, ": ").Dx()
			// compute remainder text by stripping prefix and from + ': '
			base := "[" + ln.ts + "] " + ln.from + ": "
			rest := strings.TrimPrefix(ln.text, base)
			text.Draw(screen, rest, basicfont.Face7x13, px, y, color.White)
		} else {
			var col color.Color = color.White
			if ln.system {
				col = color.NRGBA{240, 196, 25, 255}
			}
			text.Draw(screen, ln.text, basicfont.Face7x13, x+8, y, col)
		}
	}
	inputW := fullW - 180
	inputX := x + 8
	inputY := chatTop + botH - 26
	inputH := 22

	// Draw themed panel with focus indication
	g.fantasyUI.DrawThemedPanel(screen, inputX, inputY, inputW, inputH, 0.9)

	// Add focus border when chat input is focused
	if g.guildChatFocus {
		vector.StrokeRect(screen, float32(inputX), float32(inputY), float32(inputW), float32(inputH), 2, g.fantasyUI.Theme.Secondary, true)
	}

	// For long input, draw tail part that fits
	inp := g.guildChatInput
	maxW := inputW - 8
	for text.BoundString(basicfont.Face7x13, inp).Dx() > maxW && len(inp) > 0 {
		_, sz := utf8.DecodeRuneInString(inp)
		inp = inp[sz:]
	}
	text.Draw(screen, inp, basicfont.Face7x13, x+14, chatTop+botH-10, color.White)
	sendR := image.Rect(x+8+inputW+6, chatTop+botH-26, x+8+inputW+6+70, chatTop+botH-26+22)
	g.fantasyUI.DrawThemedButton(screen, sendR.Min.X, sendR.Min.Y, sendR.Dx(), sendR.Dy(), "Send", ButtonNormal)

	return
}

// Friends tab: simple search/add and list
func (g *Game) drawFriends(screen *ebiten.Image, y int) {
	// Geometry
	x := pad + 12
	fullW := protocol.ScreenW - 2*pad - 24

	// Content height and bottom-aligned input
	contentH := protocol.ScreenH - menuBarH - y - pad
	inputY := y + contentH - 30
	// Search field (bottom) with themed styling
	g.fantasyUI.DrawThemedPanel(screen, pad+12, inputY, 260, 24, 0.9)
	s := g.friendSearch
	if s == "" && !g.friendSearchFocus {
		s = "Search or add..."
	}
	text.Draw(screen, s, basicfont.Face7x13, pad+18, inputY+16, color.NRGBA{160, 160, 180, 255})
	// Add button (bottom) with themed styling
	g.fantasyUI.DrawThemedButton(screen, pad+12+270, inputY, 70, 24, "+ Add", ButtonNormal)
	// Error line above inputs
	if strings.TrimSpace(g.friendSearchError) != "" {
		text.Draw(screen, g.friendSearchError, basicfont.Face7x13, pad+18, inputY-8, color.NRGBA{240, 120, 120, 255})
	}

	// Header with count and sort
	text.Draw(screen, fmt.Sprintf("Friends %d", len(g.friends)), basicfont.Face7x13, x+8, y+28, color.White)
	sortBtn := image.Rect(x+fullW-90, y+16, x+fullW-12, y+16+22)
	sortLabel := []string{"Name", "Status"}
	g.fantasyUI.DrawThemedButton(screen, sortBtn.Min.X, sortBtn.Min.Y, sortBtn.Dx(), sortBtn.Dy(), "Sort: "+sortLabel[g.friendSortMode%2], ButtonNormal)

	// Sorted list
	friends := append([]protocol.FriendInfo(nil), g.friends...)
	switch g.friendSortMode % 2 {
	case 0:
		sort.Slice(friends, func(i, j int) bool { return strings.ToLower(friends[i].Name) < strings.ToLower(friends[j].Name) })
	case 1:
		sort.Slice(friends, func(i, j int) bool {
			if friends[i].Online != friends[j].Online {
				return friends[i].Online
			}
			return strings.ToLower(friends[i].Name) < strings.ToLower(friends[j].Name)
		})
	}

	// Rows (like guild members)
	rowsTop := y + 56
	rowH := 18
	// Visible rows based on content area (drawn by drawSocial), reserving input space
	listBottom := inputY - 8
	usableH := listBottom - rowsTop
	vis := usableH / rowH
	if vis < 0 {
		vis = 0
	}
	start := g.friendScroll
	if start < 0 {
		start = 0
	}
	maxStart := 0
	if len(friends) > vis {
		maxStart = len(friends) - vis
	}
	if g.friendScroll > maxStart {
		g.friendScroll = maxStart
		start = g.friendScroll
	}
	for i := 0; i < vis && start+i < len(friends); i++ {
		f := friends[start+i]
		yy := rowsTop + i*rowH
		// Alternate row background
		bg := color.NRGBA{0x2c, 0x2c, 0x3c, 0xFF}
		if (start+i)%2 == 1 {
			bg = color.NRGBA{0x30, 0x30, 0x40, 0xFF}
		}
		ebitenutil.DrawRect(screen, float64(x+6), float64(yy-12), float64(fullW-12), 16, bg)
		text.Draw(screen, f.Name, basicfont.Face7x13, x+12, yy, color.White)
		// Online/offline dot like guild
		dx := float32(x + fullW - 18)
		dy := float32(yy - 6)
		dc := color.NRGBA{220, 60, 60, 255}
		if f.Online {
			dc = color.NRGBA{60, 200, 80, 255}
		}
		vector.DrawFilledCircle(screen, dx, dy, 4, dc, true)
	}
}

func (g *Game) drawMessages(screen *ebiten.Image, y int) {
	// Header shows selected friend
	who := g.selectedFriend
	if who == "" {
		who = "(select a friend)"
	}
	text.Draw(screen, "Chat with "+who, basicfont.Face7x13, pad+12, y+18, color.White)
	// History panel
	panelTop := y + 36
	panelH := protocol.ScreenH - menuBarH - y - 36 - 44
	ebitenutil.DrawRect(screen, float64(pad+12), float64(panelTop), float64(protocol.ScreenW-2*pad-24), float64(panelH), color.NRGBA{0x24, 0x24, 0x30, 0xFF})
	// wheel scroll
	_, wy := ebiten.Wheel()
	if wy != 0 {
		g.dmScroll -= int(wy)
		if g.dmScroll < 0 {
			g.dmScroll = 0
		}
	}
	// render last N with scroll offset
	rowH := 16
	maxRows := panelH/rowH - 2
	start := maxInt(0, len(g.dmHistory)-maxRows-g.dmScroll)
	for i := 0; i < maxRows && start+i < len(g.dmHistory); i++ {
		dm := g.dmHistory[start+i]
		line := dm.From + ": " + dm.Text
		text.Draw(screen, line, basicfont.Face7x13, pad+18, panelTop+18+i*rowH, color.White)
	}
	// Input
	ebitenutil.DrawRect(screen, float64(pad+12), float64(panelTop+panelH+6), float64(protocol.ScreenW-2*pad-24), 24, color.NRGBA{24, 28, 40, 220})
	text.Draw(screen, g.dmInput, basicfont.Face7x13, pad+18, panelTop+panelH+22, color.White)
}

// Helpers for Social state encapsulated on Game without changing state.go too much
func (g *Game) socialActive() socialSubTab {
	switch g.socialTab {
	case 1:
		return socialGuild
	case 2:
		return socialMessages
	default:
		return socialFriends
	}
}

func fmtInt(n int) string { return fmt.Sprintf("%d", n) }

// ---- Chat line wrapping helpers ----
type chatLine struct {
	text      string
	from      string
	ts        string
	firstLine bool
	system    bool
}

// wrapString breaks s into lines that fit within maxW pixels using basicfont.Face7x13.
func wrapString(s string, maxW int) []string {
	if s == "" {
		return []string{""}
	}
	words := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '\t' })
	lines := []string{}
	cur := ""
	space := ""
	for _, w := range words {
		cand := cur + space + w
		if text.BoundString(basicfont.Face7x13, cand).Dx() <= maxW {
			cur = cand
			space = " "
			continue
		}
		if cur != "" {
			lines = append(lines, cur)
			cur = w
			space = " "
		} else {
			// a single word too long: hard wrap by runes
			runes := []rune(w)
			start := 0
			for start < len(runes) {
				end := start + 1
				for end <= len(runes) {
					seg := string(runes[start:end])
					if text.BoundString(basicfont.Face7x13, seg).Dx() > maxW {
						end--
						break
					}
					end++
				}
				if end <= start {
					end = start + 1
				}
				lines = append(lines, string(runes[start:end]))
				start = end
			}
			cur = ""
			space = ""
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

func (g *Game) handleTransferLeaderConfirmation(mx, my int) bool {
	w, h := 520, 380
	x := (protocol.ScreenW - w) / 2
	y := (protocol.ScreenH - h) / 2
	box := image.Rect(x+40, y+h-98, x+w-40, y+h-42)
	yes := image.Rect(box.Min.X+10, box.Min.Y+28, box.Min.X+10+64, box.Min.Y+28+22)
	no := image.Rect(box.Min.X+84, box.Min.Y+28, box.Min.X+84+64, box.Min.Y+28+22)

	if ptIn(mx, my, yes) {
		g.send("TransferLeader", protocol.TransferLeader{To: g.transferLeaderTarget})
		g.transferLeaderConfirm = false
		g.transferLeaderTarget = ""
		g.send("GetGuild", protocol.GetGuild{})
		return true
	}
	if ptIn(mx, my, no) {
		g.transferLeaderConfirm = false
		g.transferLeaderTarget = ""
		return true
	}
	return false
}

// visibleGuildChatLines returns the last maxRows lines for the chat panel, wrapped.
func (g *Game) visibleGuildChatLines(maxTextW int, maxRows int) []chatLine {
	// Build all lines from messages, then cut to last maxRows
	tmp := make([]chatLine, 0, maxRows)
	// walk from the end to accumulate up to maxRows
	for i := len(g.guildChat) - 1; i >= 0; i-- {
		m := g.guildChat[i]
		// ts
		ts := m.Ts
		var t time.Time
		if ts > 1e11 {
			t = time.UnixMilli(ts)
		} else {
			t = time.Unix(ts, 0)
		}
		tsStr := t.Format("2006-01-02 15:04:05")
		prefix := "[" + tsStr + "] "
		base := prefix + m.From + ": "
		contentW := maxTextW
		// First line has base prefix; subsequent lines no prefix
		// Compute how much fits after base on the first line
		firstAvail := contentW - text.BoundString(basicfont.Face7x13, base).Dx()
		if firstAvail < 40 {
			firstAvail = 40
		}
		firstLines := []string{}
		restLines := []string{}
		// Split text into wrapped lines; put as first line remainder and rest lines
		if m.System {
			all := wrapString(prefix+m.Text, contentW)
			for j, s := range all {
				cl := chatLine{text: s, from: m.From, ts: tsStr, firstLine: j == 0, system: true}
				tmp = append(tmp, cl)
				if len(tmp) >= maxRows {
					break
				}
			}
		} else {
			// wrap message content with awareness of first line reduced width
			cont := m.Text
			if cont == "" {
				cont = " "
			}
			// first chunk
			words := wrapString(cont, firstAvail)
			if len(words) > 0 {
				firstLines = append(firstLines, words[0])
			}
			if len(words) > 1 {
				restLines = append(restLines, words[1:]...)
			}
			// wrap remaining content normally
			if len(restLines) > 0 {
				restLines = wrapString(strings.Join(restLines, " "), contentW)
			}
			// assemble lines backwards for tmp (since we iterate from end)
			// add rest lines
			for j := len(restLines) - 1; j >= 0; j-- {
				s := restLines[j]
				tmp = append(tmp, chatLine{text: s, from: m.From, ts: tsStr, firstLine: false, system: false})
				if len(tmp) >= maxRows {
					break
				}
			}
			if len(tmp) < maxRows {
				// add first line with prefix
				fl := firstLines
				firstText := base + ""
				if len(fl) > 0 {
					firstText = base + fl[0]
				}
				tmp = append(tmp, chatLine{text: firstText, from: m.From, ts: tsStr, firstLine: true, system: false})
			}
		}
		if len(tmp) >= maxRows {
			break
		}
	}
	// reverse to chronological order and take up to maxRows
	if len(tmp) > maxRows {
		tmp = tmp[:maxRows]
	}
	for i, j := 0, len(tmp)-1; i < j; i, j = i+1, j-1 {
		tmp[i], tmp[j] = tmp[j], tmp[i]
	}
	return tmp
}
