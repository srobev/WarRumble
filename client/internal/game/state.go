package game

import (
	"rumble/shared/protocol"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type Game struct {
	// connection/boot UI
	connOnce        sync.Once
	connCh          chan connResult
	connSt          connState
	connErrMsg      string
	connRetryAt     time.Time
	connectInFlight bool

	auth *AuthUI

	lobbyRequested bool
	lastLobbyReq   time.Time

	net   *Net
	world *World

	// app/profile
	scr       screen
	activeTab tab
	nameInput string
	playerID  int64
	name      string

	// battle
	gold              int
	hand              []protocol.MiniCardView
	next              protocol.MiniCardView
	selectedIdx       int
	playerHB          *battleHPBar
	enemyHB           *battleHPBar
	enemyTargetThumb  *ebiten.Image // cached thumbnail of current target (base/boss)
	enemyBossPortrait string        // optional: set this later for boss fights
	enemyAvatar       string

	// Army tab: split pickers
	minisAll            []protocol.MiniInfo // everything from server
	champions           []protocol.MiniInfo // role/class champion
	minisOnly           []protocol.MiniInfo // role mini && class != spell
	selectedChampion    string
	selectedMinis       map[string]bool
	armyMsg             string
	armyBg, armyBgLayer *ebiten.Image

	// scroll state + clickable rects
	champScroll int
	miniScroll  int
	// minis collection drag/touch (vertical)
	collDragActive bool
	collDragStartY int
	collDragLastY  int
	collDragAccum  int
	collTouchID    ebiten.TouchID

	// Map tab
	maps        []protocol.MapInfo
	mapRects    []rect
	selectedMap int
	roomID      string
	startBtn    rect
	// --- Top bar (Home only) ---
	accountGold int64 // separate "meta" currency (not battle gold)
	userBtn     rect  // clickable later for profile/statistics
	titleArea   rect
	goldArea    rect
	topBarBg    *ebiten.Image
	// bottom bar buttons
	armyBtn, mapBtn, pvpBtn, socialBtn, settingsBtn rect
	bottomBarBg                                     *ebiten.Image

	// settings
	fullscreen        bool
	fsOnBtn, fsOffBtn rect
	autoSaveEnabled   bool
	logoutBtn         rect

	assets     Assets
	nameToMini map[string]protocol.MiniInfo // by mini Name
	currentMap string

	// drag & drop state
	dragActive bool
	dragIdx    int
	dragStartX int
	dragStartY int

	gameOver bool
	victory  bool

	endActive   bool
	endVictory  bool
	continueBtn rect
	// XP results at battle end
	preBattleXP map[string]int
	xpGains     map[string]int // name -> +XP
	battleArmy  []string       // names used at battle start

	// Army grid UI
	showChamp bool // true: show champions, false: show minis

	// --- Army: new UI state ---
	// champion strip (top)
	champStripArea   rect
	champStripRects  []rect
	champStripScroll int // index of first visible champion
	// --- Champion strip drag/touch scroll ---
	champDragActive bool
	champDragStartX int
	champDragLastX  int
	champDragAccum  int // pixels accumulated to convert into column steps

	// touch tracking (single-pointer drag)
	activeTouchID ebiten.TouchID

	// selected champion + minis (2x3 grid)
	armySlotRects [7]rect // [0]=champion big card, [1..6] mini slots
	selectedOrder [6]string
	// drag between equipped slots only
	slotDragFrom   int // -1 if none, 0..5 otherwise
	slotDragStartX int
	slotDragStartY int
	slotDragActive bool

	// collection grid (minis only)
	collArea   rect
	collRects  []rect
	collScroll int // row-based scroll for minis collection

	// data for per-champion armies (client-side cache)
	champToMinis map[string]map[string]bool // champName -> set(miniName)
	champToOrder map[string][6]string       // champName -> slot order

	// --- Minis XP / Level overlay ---
	unitXP            map[string]int // from Profile.UnitXP
	miniOverlayOpen   bool
	miniOverlayName   string
	miniOverlayFrom   string // "slot" or "collection"
	miniOverlaySlot   int    // 0..5 for slots when from=="slot"
	miniOverlayMode   string // "" | "switch_target_slot" | "switch_target_collection"
	overlayJustClosed bool   // consume this click to avoid reopening
	xpBarHovered      bool   // true when mouse is hovering over XP bar

	// Map tab (new hotspot UI)
	mapHotspots  map[string][]Hotspot // key: mapID -> hotspots
	hoveredHS    int                  // -1 if none
	selectedHS   int                  // -1 if none
	mapDebug     bool                 // add in Game struct
	rectHotspots map[string][]HitRect
	showRects    bool // optional debug outline toggle

	currentArena string
	pendingArena string

	// HP bar FX (recent-damage yellow chip)
	hpFxUnits map[int64]*hpFx
	hpFxBases map[int64]*hpFx

	// Particle effects system
	particleSystem *ParticleSystem

	// Map definition for deploy zones
	currentMapDef *protocol.MapDef

	// Camera system for battle map scrolling and zooming
	cameraX            float64
	cameraY            float64
	cameraZoom         float64
	cameraMinZoom      float64
	cameraMaxZoom      float64
	cameraDragging     bool
	cameraDragStartX   int
	cameraDragStartY   int
	cameraDragInitialX float64
	cameraDragInitialY float64
	// Left mouse drag for scrolling (only when no mini selected)
	cameraLeftDragging     bool
	cameraLeftDragStartX   int
	cameraLeftDragStartY   int
	cameraLeftDragInitialX float64
	cameraLeftDragInitialY float64
	// Flag to center camera on player's base once bases are populated
	needsCameraCenter bool

	// --- PvP UI state ---
	pvpStatus      string // status line at the top of the PvP tab
	pvpQueued      bool   // currently in matchmaking queue
	pvpHosting     bool   // currently hosting a friendly code
	pvpCode        string // last code we got back from the server (when hosting)
	pvpCodeInput   string // what the user typed into the "Join with code" field
	pvpInputActive bool   // text input focus for the code field
	pvpCodeArea    rect   // This will be the pvpcode copy area
	// profile PvP
	pvpRating int
	pvpRank   string
	// PvP leaderboard
	pvpLeaders  []protocol.LeaderboardEntry
	lbLastReq   time.Time
	lbLastStamp int64 // server GeneratedAt (optional)

	// --- Timer UI state ---
	timerRemainingSeconds int    // remaining seconds
	timerPaused           bool   // whether timer is paused
	timerBtn              rect   // pause button rect
	timerDisplay          string // formatted time display
	lastTimerUpdate       int64  // last time timer was updated (Unix timestamp)
	pauseOverlay          bool   // whether pause overlay is shown
	pauseRestartBtn       rect   // restart button in pause overlay
	pauseSurrenderBtn     rect   // surrender button in pause overlay

	// --- Base shooting system ---
	baseLastShot map[int64]int64 // baseID -> last shot timestamp (UnixMilli)

	// --- Unit targeting system ---
	// targetValidator *shared.UnitTargetValidator // Not used in client

	//Avatars and profile
	avatar      string
	showProfile bool

	avatars       []string // discovered from assets/ui/avatars
	avatarRects   []rect
	profileOpen   bool
	profCloseBtn  rect
	profLogoutBtn rect

	// --- Social / Guilds ---
	socialTab           int // 0=friends,1=guild,2=messages
	guildID             string
	guildName           string
	guildMembers        []protocol.GuildMember
	guildList           []protocol.GuildSummary
	guildChat           []protocol.GuildChatMsg
	guildChatInput      string
	guildChatFocus      bool
	guildBrowse         bool
	selectedGuildMember string
	guildDescEdit       string
	guildDescFocus      bool
	guildCreateName     string
	guildFilter         string
	guildListScroll     int
	guildNameFocus      bool
	guildFilterFocus    bool
	guildMembersScroll  int
	guildSortMode       int // 0=name,1=status,2=rank
	guildLeaveConfirm   bool
	guildLeaveError     string
	guildDisbandConfirm bool
	guildDisbandError   string
	chatBackspaceStart  time.Time
	chatBackspaceLast   time.Time

	// Friends / Messages
	friends              []protocol.FriendInfo
	friendSearch         string
	friendSearchFocus    bool
	friendSearchError    string
	friendAddLookup      string // non-empty when awaiting GetUserProfile to validate add
	lastFriendsReq       time.Time
	friendScroll         int
	selectedFriend       string
	dmInput              string
	dmInputFocus         bool
	dmHistory            []protocol.FriendDM
	dmScroll             int
	confirmRemoveFriend  string
	dmOverlay            bool
	memberProfileOverlay bool
	memberProfile        protocol.Profile
	// Friends list sorting: 0=name,1=status
	friendSortMode        int
	profileFromFriends    bool
	transferLeaderConfirm bool
	transferLeaderTarget  string
	socialTabLoaded       bool
	guildChatLoaded       bool
	// Track previous guild roster to generate system events (join/leave/promote)
	prevGuildRoles      map[string]string
	havePrevGuildRoster bool

	// Throttle guild list requests
	lastGuildListReq time.Time
	lastGuildQuery   string
	lastGuildInfoReq time.Time
	guildSendClickAt time.Time

	// Hover tooltips for Army tab
	hoveredChampionLevel int // index of hovered champion for level tooltip (-1 if none)
	hoveredChampionCost  int // index of hovered champion for cost tooltip (-1 if none)
	hoveredChampionCard  int // index of hovered champion card for frame effect (-1 if none)

	// Additional hover states for all Army tab elements
	hoveredSelectedChampionLevel bool // true when hovering selected champion level badge
	hoveredSelectedChampionCost  bool // true when hovering selected champion cost text
	hoveredSelectedChampionXP    bool // true when hovering selected champion XP bar
	hoveredSelectedChampionCard  bool // true when hovering selected champion card for frame effect
	hoveredMiniSlotLevel         int  // index of hovered equipped mini slot level (-1 if none)
	hoveredMiniSlotCost          int  // index of hovered equipped mini slot cost (-1 if none)
	hoveredMiniSlotXP            int  // index of hovered equipped mini slot XP bar (-1 if none)
	hoveredMiniSlotCard          int  // index of hovered equipped mini slot card for frame effect (-1 if none)
	hoveredCollectionLevel       int  // index of hovered collection item level (-1 if none)
	hoveredCollectionCost        int  // index of hovered collection item cost (-1 if none)
	hoveredCollectionXP          int  // index of hovered collection item XP bar (-1 if none)
	hoveredCollectionCard        int  // index of hovered collection item card for frame effect (-1 if none)
	hoveredOverlayLevel          bool // true when hovering mini overlay level badge
	hoveredOverlayCost           bool // true when hovering mini overlay cost text

	// Fantasy UI System
	fantasyUI *FantasyUI // manages themed UI elements across the home screen
}
