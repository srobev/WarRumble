package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"rumble/shared/protocol"
)

// Account represents an account record with persisted data
type Account struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Gold      int64  `json:"gold"`
	AccountXP int    `json:"accountXp"`
	// Add more fields as needed
}

// accountLocks protects per-account operations with sync.Mutex
var accountLocks = make(map[string]*sync.Mutex)
var accountsDir = filepath.Join("data", "profiles")

// getAccountLock returns a mutex for the given account ID
func getAccountLock(id string) *sync.Mutex {
	lock, exists := accountLocks[id]
	if !exists {
		lock = &sync.Mutex{}
		accountLocks[id] = lock
	}
	return lock
}

// LoadAccount loads an account by ID from disk
func LoadAccount(id string) (*Account, error) {
	if id == "" {
		return nil, errors.New("empty account ID")
	}

	lock := getAccountLock(id)
	lock.Lock()
	defer lock.Unlock()

	path := filepath.Join(accountsDir, fmt.Sprintf("%s.json", id))
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Return default account if not found
			return &Account{
				ID:        id,
				Name:      id, // Use ID as default name
				Gold:      0,
				AccountXP: 0,
			}, nil
		}
		return nil, fmt.Errorf("failed to read account file: %w", err)
	}

	var account Account
	if err := json.Unmarshal(data, &account); err != nil {
		return nil, fmt.Errorf("failed to unmarshal account: %w", err)
	}

	return &account, nil
}

// SaveAccount persists an account to disk atomically
func SaveAccount(account *Account) error {
	if account == nil || account.ID == "" {
		return errors.New("invalid account")
	}

	lock := getAccountLock(account.ID)
	lock.Lock()
	defer lock.Unlock()

	// Create accounts directory if it doesn't exist
	if err := os.MkdirAll(accountsDir, 0755); err != nil {
		return fmt.Errorf("failed to create accounts directory: %w", err)
	}

	path := filepath.Join(accountsDir, fmt.Sprintf("%s.json", account.ID))

	// Marshal to JSON
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
func LoadAccountByName(name string) (*Account, error) {
	if name == "" {
		return nil, errors.New("empty account name")
	}

	// Find the account ID by name from profiles
	profilesDir := filepath.Join("data", "profiles")
	files, err := os.ReadDir(profilesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles directory: %w", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		path := filepath.Join(profilesDir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var profile protocol.Profile
		if err := json.Unmarshal(data, &profile); err != nil {
			continue
		}

		if profile.Name == name {
			// Found the profile, load corresponding account
			return LoadAccount(fmt.Sprintf("%d", profile.PlayerID))
		}
	}

	// Account not found
	return &Account{
		ID:        "", // Will be set when saved
		Name:      name,
		Gold:      0,
		AccountXP: 0,
	}, nil
}
