package account

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"rumble/shared/game/types"
)

// ShopState represents shop-specific account data
type ShopState struct {
	Roll       []types.ShopSlot `json:"roll"`       // 81 shop slots
	LastReroll int64            `json:"lastReroll"` // unix sec
	Sold       map[int]bool     `json:"sold"`       // slot->true
	NonceSeen  map[string]bool  `json:"nonceSeen"`  // for idempotency
	Version    int              `json:"version"`    // for format changes
}

// Account represents an account record with persisted data
type Account struct {
	ID           string                         `json:"id"`
	Name         string                         `json:"name"`
	Gold         int64                          `json:"gold"`
	AccountXP    int                            `json:"accountXp"`
	Resources    map[string]int                 `json:"resources"` // dust, gems, etc.
	PvPRating    int                            `json:"pvpRating"` // PvP rating points
	PvPRank      string                         `json:"pvpRank"`   // calculated PvP rank name
	Avatar       string                         `json:"avatar"`    // selected avatar filename
	GuildID      string                         `json:"guildId"`   // guild membership ID
	UnitXP       map[string]int                 `json:"unitXp"`    // unit XP for level calculation
	Shop         ShopState                      `json:"shop"`
	Progress     map[string]*types.UnitProgress `json:"progress"`     // unitID -> progress (includes shards)
	Dust         int                            `json:"dust"`         // upgrade dust currency
	Capsules     CapsuleStock                   `json:"capsules"`     // upgrade capsules by rarity
	Army         []string                       `json:"army"`         // active army [champ, 6 minis]
	Armies       map[string][]string            `json:"armies"`       // saved armies
	LastUpdated  int64                          `json:"lastUpdated"`  // unix timestamp for race condition prevention
	SectionTimes SectionUpdateTimes             `json:"sectionTimes"` // individual section update times
}

// CapsuleStock tracks capsule counts by rarity
type CapsuleStock struct {
	Rare      int `json:"rare"`
	Epic      int `json:"epic"`
	Legendary int `json:"legendary"`
}

// SectionUpdateTimes tracks last update times for different account sections
type SectionUpdateTimes struct {
	Progress  int64 `json:"progress"`  // last Progress update timestamp
	Resources int64 `json:"resources"` // last Resources update timestamp
	PvP       int64 `json:"pvp"`       // last PvP data update timestamp
	Army      int64 `json:"army"`      // last Army update timestamp
	Shop      int64 `json:"shop"`      // last Shop update timestamp
	Profile   int64 `json:"profile"`   // last basic profile update timestamp
}

// accountLocks protects per-account operations with sync.Mutex
var accountLocks = make(map[string]*sync.Mutex)
var accountsDir = filepath.Join("data", "profiles")

// safeFileName creates a safe filename from potentially unsafe characters
func safeFileName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	s := re.ReplaceAllString(name, "_")
	if s == "" {
		s = "player"
	}
	return s
}

// getAccountLock returns a mutex for the given account ID
func getAccountLock(id string) *sync.Mutex {
	lock, exists := accountLocks[id]
	if !exists {
		lock = &sync.Mutex{}
		accountLocks[id] = lock
	}
	return lock
}

// LoadAccount loads an account by username (no longer by numeric ID)
func LoadAccount(username string) (*Account, error) {
	if username == "" {
		return nil, errors.New("empty username")
	}

	lock := getAccountLock(username)
	lock.Lock()
	defer lock.Unlock()

	// Now accounts are stored by username instead of numeric ID
	path := filepath.Join(accountsDir, safeFileName(username)+".json")

	data, err := os.ReadFile(path)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		// Account not found - create new account with unique ID
		// Generate unique ID for new account
		idBytes := make([]byte, 16)
		rand.Read(idBytes)
		newID := hex.EncodeToString(idBytes)

		now := time.Now().Unix()
		log.Printf("ACCOUNT: Creating new account for username '%s' (ID: %s)", username, newID)
		return &Account{
			ID:        newID,
			Name:      username,
			Gold:      1000, // Give starting gold
			AccountXP: 0,
			Resources: make(map[string]int),
			PvPRating: 1200,
			PvPRank:   "Knight", // Default PvP rank
			Avatar:    "default.png",
			GuildID:   "",
			UnitXP:    make(map[string]int),
			Shop: ShopState{
				Roll:       make([]types.ShopSlot, 0),
				LastReroll: 0,
				Sold:       make(map[int]bool),
				NonceSeen:  make(map[string]bool),
				Version:    1,
			},
			Progress:    make(map[string]*types.UnitProgress),
			Army:        nil,
			Armies:      make(map[string][]string),
			LastUpdated: now,
			SectionTimes: SectionUpdateTimes{
				Progress:  now,
				Resources: now,
				PvP:       now,
				Army:      now,
				Shop:      now,
				Profile:   now,
			},
		}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to read account file: %w", err)
	}

	var account Account
	if err := json.Unmarshal(data, &account); err != nil {
		log.Printf("ACCOUNT: Failed to unmarshal account file: %v, creating new account", err)
		// Return default account if parsing fails
		return &Account{
			ID:        username, // Use username as ID for now
			Name:      username,
			Gold:      1000, // Give starting gold
			AccountXP: 0,
			Shop: ShopState{
				Roll:       make([]types.ShopSlot, 0),
				LastReroll: 0,
				Sold:       make(map[int]bool),
				NonceSeen:  make(map[string]bool),
				Version:    1,
			},
			Progress: make(map[string]*types.UnitProgress),
			Army:     nil,
			Armies:   make(map[string][]string),
		}, nil
	}

	// Ensure all maps and slices are initialized, but preserve existing data
	if account.Shop.Sold == nil {
		account.Shop.Sold = make(map[int]bool)
	}
	if account.Shop.NonceSeen == nil {
		account.Shop.NonceSeen = make(map[string]bool)
	}
	if account.Progress == nil {
		account.Progress = make(map[string]*types.UnitProgress)
	}
	if account.Armies == nil {
		account.Armies = make(map[string][]string)
	}
	if account.Resources == nil {
		account.Resources = make(map[string]int)
	}
	if account.UnitXP == nil {
		account.UnitXP = make(map[string]int)
	}
	if account.Army == nil {
		account.Army = make([]string, 0)
	}

	// Set default values only if they're truly empty/unset
	if account.Avatar == "" {
		account.Avatar = "default.png"
	}
	if account.PvPRating == 0 {
		account.PvPRating = 1200
		account.PvPRank = "Knight"
	}

	// Update PvP rank if it's empty but rating is set
	if account.PvPRank == "" && account.PvPRating > 0 {
		account.PvPRank = rankName(account.PvPRating)
	}

	// Give starting gold if account has 0 (new accounts)
	if account.Gold == 0 {
		account.Gold = 1000
		log.Printf("ACCOUNT: Giving starting gold to account %s", username)
		// This call will now use the new username-based saving
		account.Name = username // Ensure name is set properly
		if saveErr := SaveAccount(&account); saveErr != nil {
			log.Printf("ACCOUNT: Failed to save starting gold for %s: %v", username, saveErr)
		}
	}

	return &account, nil
}

// SaveAccount persists an account to disk atomically
// Now saves to username.json instead of user_id.json for cleaner file structure
func SaveAccount(account *Account) error {
	if account == nil {
		log.Printf("ACCOUNT SAVE ERROR: account is nil")
		return errors.New("invalid account: nil pointer")
	}

	// Use name for filename, fallback to ID if name is empty
	filename := strings.TrimSpace(account.Name)
	if filename == "" {
		filename = strings.TrimSpace(account.ID)
	}

	if filename == "" {
		log.Printf("ACCOUNT SAVE ERROR: no valid filename (name or ID)")
		return errors.New("invalid account: no name or ID for filename")
	}

	lock := getAccountLock(account.ID)
	lock.Lock()
	defer lock.Unlock()

	// Create accounts directory if it doesn't exist
	if err := os.MkdirAll(accountsDir, 0755); err != nil {
		return fmt.Errorf("failed to create accounts directory: %w", err)
	}

	path := filepath.Join(accountsDir, safeFileName(filename)+".json")

	// Marshal to JSON (just the account data, not profile data)
	data, err := json.MarshalIndent(account, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal account: %w", err)
	}

	// Write to temp file
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp account file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		// Try to clean up temp file
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename account file: %w", err)
	}

	return nil
}

// UpdateGold updates an account's gold and saves it atomically
// Returns the updated account and success status
func UpdateGold(id string, delta int64) (*Account, error) {
	if id == "" {
		return nil, errors.New("empty account ID")
	}

	lock := getAccountLock(id)
	lock.Lock()
	defer lock.Unlock()

	account, err := LoadAccount(id)
	if err != nil {
		return nil, fmt.Errorf("failed to load account: %w", err)
	}

	// Validate the new gold amount
	newGold := account.Gold + delta
	if newGold < 0 {
		return nil, errors.New("insufficient funds")
	}

	account.Gold = newGold

	// Save the updated account
	if err := SaveAccount(account); err != nil {
		return nil, fmt.Errorf("failed to save account: %w", err)
	}

	return account, nil
}

// LoadAccountByName loads an account by username (matches Profile.Name)
// This is now a simple wrapper around LoadAccount since both use usernames
func LoadAccountByName(name string) (*Account, error) {
	return LoadAccount(name)
}

// UpdateTimestamps updates the account's timestamp information for tracking and race condition prevention
func (a *Account) UpdateTimestamps(section string) {
	now := time.Now().Unix()
	a.LastUpdated = now

	switch section {
	case "progress":
		a.SectionTimes.Progress = now
	case "resources":
		a.SectionTimes.Resources = now
	case "pvp":
		a.SectionTimes.PvP = now
	case "army":
		a.SectionTimes.Army = now
	case "shop":
		a.SectionTimes.Shop = now
	case "profile":
		a.SectionTimes.Profile = now
	}
}

// ValidateUpdateTime validates that an update operation won't cause race conditions
func (a *Account) ValidateUpdateTime(section string, clientTime int64) bool {
	var lastUpdateTime int64

	switch section {
	case "progress":
		lastUpdateTime = a.SectionTimes.Progress
	case "resources":
		lastUpdateTime = a.SectionTimes.Resources
	case "pvp":
		lastUpdateTime = a.SectionTimes.PvP
	case "army":
		lastUpdateTime = a.SectionTimes.Army
	case "shop":
		lastUpdateTime = a.SectionTimes.Shop
	case "profile":
		lastUpdateTime = a.SectionTimes.Profile
	default:
		// For unknown sections, use overall last update time
		lastUpdateTime = a.LastUpdated
	}

	// If client time is older than server time, reject the update
	if clientTime > 0 && clientTime < lastUpdateTime {
		log.Printf("ACCOUNT: Race condition detected for %s, client time %d < server time %d", section, clientTime, lastUpdateTime)
		return false
	}

	return true
}

// GetResource returns the amount of a specific resource
func (a *Account) GetResource(resourceType string) int {
	if a.Resources == nil {
		return 0
	}
	return a.Resources[resourceType]
}

// AddResource adds an amount to a specific resource and updates timestamps
func (a *Account) AddResource(resourceType string, amount int) {
	if a.Resources == nil {
		a.Resources = make(map[string]int)
	}
	a.Resources[resourceType] += amount
	a.UpdateTimestamps("resources")
}

// SetResource sets the amount of a specific resource and updates timestamps
func (a *Account) SetResource(resourceType string, amount int) {
	if a.Resources == nil {
		a.Resources = make(map[string]int)
	}
	a.Resources[resourceType] = amount
	a.UpdateTimestamps("resources")
}

// UpdatePvPRating updates PvP rating and rank, with automatic rank calculation
func (a *Account) UpdatePvPRating(newRating int) {
	if newRating < 0 {
		newRating = 0
	}
	a.PvPRating = newRating
	a.PvPRank = rankName(newRating)
	a.UpdateTimestamps("pvp")
}

// UpdateArmy updates the current army and updates timestamps
func (a *Account) UpdateArmy(newArmy []string) {
	a.Army = newArmy
	a.UpdateTimestamps("army")
}

// EnsureAccountInitialized ensures all account fields are properly initialized
func (a *Account) EnsureAccountInitialized() {
	// Initialize maps if nil
	if a.Resources == nil {
		a.Resources = make(map[string]int)
	}
	if a.UnitXP == nil {
		a.UnitXP = make(map[string]int)
	}
	if a.Progress == nil {
		a.Progress = make(map[string]*types.UnitProgress)
	}
	if a.Armies == nil {
		a.Armies = make(map[string][]string)
	}
	if a.Army == nil {
		a.Army = make([]string, 0)
	}

	// Set default avatar if empty
	if a.Avatar == "" {
		a.Avatar = "default.png"
	}

	// Set default PvP rating and rank if zero
	if a.PvPRating == 0 {
		a.PvPRating = 1200
		a.PvPRank = "Knight"
	}
}

// Progression functions integrated into account service

// LoadUnitProgress loads unit progress from account's Progress map
func (a *Account) LoadUnitProgress(unitID string) (*types.UnitProgress, error) {
	if a.Progress == nil {
		a.Progress = make(map[string]*types.UnitProgress)
	}

	progress, exists := a.Progress[unitID]
	if exists && progress != nil {
		return progress, nil
	}

	// Return defaults if unit not found
	return &types.UnitProgress{
		UnitID:        unitID,
		Rarity:        types.RarityCommon, // Would need to be loaded from unit data
		Rank:          1,
		ShardsOwned:   0,
		PerksUnlocked: []types.PerkID{},
		ActivePerk:    nil,
	}, nil
}

// SaveUnitProgress saves unit progress to account
// This automatically saves the entire account to maintain consistency
func (a *Account) SaveUnitProgress(progress *types.UnitProgress, username string) error {
	if progress == nil {
		return errors.New("progress is nil")
	}

	if a.Progress == nil {
		a.Progress = make(map[string]*types.UnitProgress)
	}

	a.Progress[progress.UnitID] = progress
	a.UpdateTimestamps("progress")

	log.Printf("ACCOUNT: Saving progress for unit %s, shards: %d, rank: %d",
		progress.UnitID, progress.ShardsOwned, progress.Rank)

	return SaveAccount(a)
}

// AddShards adds shards to unit progress (manual rank-up system)
// NOTE: This ONLY adds shards - rank-ups happen manually via HandleUpgradeUnit
func (a *Account) AddShards(unitID string, add int) (rankUps int, err error) {
	if add <= 0 {
		return 0, nil
	}

	progress, err := a.LoadUnitProgress(unitID)
	if err != nil {
		return 0, fmt.Errorf("failed to load unit progress: %w", err)
	}

	// SIMPLE TRADITIONAL LOGIC:
	// First shard = unit ownership (we already have this)
	// Additional shards = clone counter for ranking up

	// Add all shards directly (no weird calculations)
	progress.ShardsOwned += add

	// Save the updated progress
	if saveErr := a.SaveUnitProgress(progress, a.Name); saveErr != nil {
		return 0, fmt.Errorf("failed to save updated progress: %w", saveErr)
	}

	return 0, nil // Never auto rank-up
}

// HandleUnitProgressUpdate processes adding shards to unit progress (integrated into account)
func (a *Account) HandleUnitProgressUpdate(username, unitID string, deltaShards int) error {
	// Add shards and update progress (AddShards loads progress internally)
	if deltaShards > 0 {
		_, addErr := a.AddShards(unitID, deltaShards)
		if addErr != nil {
			return fmt.Errorf("failed to add shards: %w", addErr)
		}
	}

	return nil
}

// GetUpgradeCost delegates to shared function (DRY principle)
func (a *Account) GetUpgradeCost(currentRank int) int {
	return types.GetUpgradeCost(currentRank)
}

// HandleUpgradeUnit processes unit upgrade (consuming shards, dust, and capsules)
func (a *Account) HandleUpgradeUnit(username, unitID string) error {
	progress, err := a.LoadUnitProgress(unitID)
	if err != nil {
		return fmt.Errorf("failed to load progress: %w", err)
	}

	// Check if already at max rank (5)
	if progress.Rank >= 5 {
		return fmt.Errorf("unit at max rank (5)")
	}

	// Get shard requirements (existing logic)
	requiredShards := a.GetUpgradeCost(progress.Rank)
	if progress.ShardsOwned < requiredShards {
		return fmt.Errorf("insufficient shards: have %d, need %d", progress.ShardsOwned, requiredShards)
	}

	// Get dust and capsule requirements based on unit rarity
	rarityStr := a.getRarityString(progress.Rarity)
	dustNeeded, capsuleNeeded, capsuleType := a.GetUpgradeCosts(rarityStr, progress.Rank)

	// Check dust availability
	if a.Dust < dustNeeded {
		return fmt.Errorf("insufficient dust: have %d, need %d", a.Dust, dustNeeded)
	}

	// Check capsule availability
	capsulesHave := a.GetCapsule(capsuleType)
	if capsulesHave < capsuleNeeded {
		return fmt.Errorf("insufficient %s capsules: have %d, need %d", capsuleType, capsulesHave, capsuleNeeded)
	}

	// All resources validated - consume them
	progress.ShardsOwned -= requiredShards
	a.Dust -= dustNeeded
	if capsuleNeeded > 0 {
		a.AddCapsule(capsuleType, -capsuleNeeded) // Negative value to reduce capsules
	}
	progress.Rank++

	if err := a.SaveUnitProgress(progress, username); err != nil {
		return fmt.Errorf("failed to save progress: %w", err)
	}

	if err := SaveAccount(a); err != nil {
		return fmt.Errorf("failed to save account: %w", err)
	}

	log.Printf("[%s] Upgraded unit %s to rank %d (consumed %d shards, %d dust, %d %s capsules)",
		username, unitID, progress.Rank, requiredShards, dustNeeded, capsuleNeeded, capsuleType)

	return nil
}

// getRarityString converts Rarity enum to string
func (a *Account) getRarityString(rarity types.Rarity) string {
	switch rarity {
	case types.RarityRare:
		return "rare"
	case types.RarityEpic:
		return "epic"
	case types.RarityLegendary:
		return "legendary"
	default:
		return "rare"
	}
}

// HandleSetActivePerk processes setting active perk for unit
func (a *Account) HandleSetActivePerk(username, unitID string, perkID types.PerkID) error {
	progress, err := a.LoadUnitProgress(unitID)
	if err != nil {
		return fmt.Errorf("failed to load progress: %w", err)
	}

	// Check if perk is purchased
	hasPerk := false
	for _, unlocked := range progress.PerksUnlocked {
		if unlocked == perkID {
			hasPerk = true
			break
		}
	}

	if !hasPerk {
		return fmt.Errorf("perk not purchased")
	}

	progress.ActivePerk = &perkID

	if err := a.SaveUnitProgress(progress, username); err != nil {
		return fmt.Errorf("failed to save progress: %w", err)
	}

	return nil
}

// Dust and Capsule management functions

// AddDust adds dust to the account
func (a *Account) AddDust(amount int) {
	a.Dust += amount
	a.UpdateTimestamps("resources")
}

// GetDust returns current dust amount
func (a *Account) GetDust() int {
	return a.Dust
}

// AddCapsule adds capsules to a specific rarity
func (a *Account) AddCapsule(rarity string, amount int) {
	switch rarity {
	case "rare":
		a.Capsules.Rare += amount
	case "epic":
		a.Capsules.Epic += amount
	case "legendary":
		a.Capsules.Legendary += amount
	}
	a.UpdateTimestamps("resources")
}

// GetCapsule returns count of a specific capsule type
func (a *Account) GetCapsule(rarity string) int {
	switch rarity {
	case "rare":
		return a.Capsules.Rare
	case "epic":
		return a.Capsules.Epic
	case "legendary":
		return a.Capsules.Legendary
	default:
		return 0
	}
}

// GetUpgradeCosts returns the dust and capsule requirements for upgrading from currentRank
func (a *Account) GetUpgradeCosts(rarity string, currentRank int) (dustNeeded, capsuleNeeded int, capsuleType string) {
	// Specific costs per rank transition (NO scaling, fixed values)
	switch currentRank {
	case 1: // Rank 1 → Rank 2: 3 shards + 500 dust (no capsule)
		dustNeeded = 500
		capsuleNeeded = 0
		capsuleType = ""
	case 2: // Rank 2 → Rank 3: 10 shards + 2000 dust + 1 rare capsule
		dustNeeded = 2000
		capsuleNeeded = 1
		capsuleType = "rare"
	case 3: // Rank 3 → Rank 4: 25 shards + 8000 dust + 1 epic capsule
		dustNeeded = 8000
		capsuleNeeded = 1
		capsuleType = "epic"
	case 4: // Rank 4 → Rank 5: 25 shards + 20000 dust + 1 legendary capsule
		dustNeeded = 20000
		capsuleNeeded = 1
		capsuleType = "legendary"
	default: // Max rank reached
		dustNeeded = 0
		capsuleNeeded = 0
		capsuleType = ""
	}

	return dustNeeded, capsuleNeeded, capsuleType
}

// PerkSlotsUnlocked returns the number of perk slots unlocked for the unit's rarity
func (a *Account) PerkSlotsUnlocked(unitID string) int {
	progress, err := a.LoadUnitProgress(unitID)
	if err != nil {
		return 0
	}
	return int(progress.Rarity)
}

// rankName calculates PvP rank name from rating points
func rankName(rating int) string {
	switch {
	case rating >= 2500:
		return "Myth"
	case rating >= 2200:
		return "Legend"
	case rating >= 1900:
		return "Hero"
	case rating >= 1600:
		return "Champion"
	case rating >= 1300:
		return "Elite"
	default:
		return "Knight"
	}
}
