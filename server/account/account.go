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
	ID        string                         `json:"id"`
	Name      string                         `json:"name"`
	Gold      int64                          `json:"gold"`
	AccountXP int                            `json:"accountXp"`
	Shop      ShopState                      `json:"shop"`
	Progress  map[string]*types.UnitProgress `json:"progress"` // unitID -> progress
	Army      []string                       `json:"army"`     // active army [champ, 6 minis]
	Armies    map[string][]string            `json:"armies"`   // saved armies
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

		log.Printf("ACCOUNT: Creating new account for username '%s' (ID: %s)", username, newID)
		return &Account{
			ID:        newID,
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

	// Ensure shop state is properly initialized
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
