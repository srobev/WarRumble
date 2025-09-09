package currency

import (
	"fmt"
	"log"
	"sync"

	"rumble/server/account"
	"rumble/shared/protocol"
)

type SessionCtx struct {
	AccountID string
}

// spentNonces tracks nonces to prevent duplicate spends
var spentNonces = make(map[string]bool)
var nonceMutex = sync.Mutex{}

// GrantGold handles granting gold to a player's account
func HandleGrantGold(ctx *SessionCtx, req protocol.GrantGold) error {
	if ctx.AccountID == "" {
		return fmt.Errorf("invalid session: no account ID")
	}

	if req.Amount <= 0 {
		return &CurrencyError{Code: "INVALID_AMOUNT", Message: "Amount must be > 0"}
	}

	// Load current account
	acct, err := account.LoadAccount(ctx.AccountID)
	if err != nil {
		return fmt.Errorf("failed to load account: %w", err)
	}

	acct.Gold += req.Amount

	// Save updated account
	if err := account.SaveAccount(acct); err != nil {
		return fmt.Errorf("failed to save account: %w", err)
	}

	log.Printf("Granted %d gold to account %s (%s), new balance: %d",
		req.Amount, ctx.AccountID, req.Reason, acct.Gold)

	return nil
}

// SpendGold handles spending gold from a player's account
func HandleSpendGold(ctx *SessionCtx, req protocol.SpendGold) error {
	if ctx.AccountID == "" {
		return fmt.Errorf("invalid session: no account ID")
	}

	if req.Amount <= 0 {
		return &CurrencyError{Code: "INVALID_AMOUNT", Message: "Amount must be > 0"}
	}

	// Check nonce to prevent duplicate spends
	nonceMutex.Lock()
	if spentNonces[req.Nonce] {
		nonceMutex.Unlock()
		log.Printf("Duplicate spend attempt with nonce %s", req.Nonce)
		return nil // Silently ignore duplicate
	}
	spentNonces[req.Nonce] = true
	nonceMutex.Unlock()

	// Load current account
	acct, err := account.LoadAccount(ctx.AccountID)
	if err != nil {
		return fmt.Errorf("failed to load account: %w", err)
	}

	// Check balance
	if acct.Gold < req.Amount {
		return &CurrencyError{Code: "INSUFFICIENT_FUNDS", Message: "Not enough gold"}
	}

	// Deduct gold
	acct.Gold -= req.Amount

	// Save updated account
	if err := account.SaveAccount(acct); err != nil {
		// Undo nonce marking on save failure
		nonceMutex.Lock()
		delete(spentNonces, req.Nonce)
		nonceMutex.Unlock()
		return fmt.Errorf("failed to save account: %w", err)
	}

	log.Printf("Spent %d gold from account %s (%s), new balance: %d",
		req.Amount, ctx.AccountID, req.Reason, acct.Gold)

	return nil
}

// PushGoldSynced sends the current gold balance to the client
func PushGoldSynced(ctx *SessionCtx, gold int64) error {
	// Load current account to verify gold amount
	if ctx.AccountID != "" {
		account, err := account.LoadAccount(ctx.AccountID)
		if err != nil {
			return fmt.Errorf("failed to load account for sync: %w", err)
		}
		// Use authoritative value instead of passed value
		gold = account.Gold
	}

	return PushGoldSyncedDirect(gold)
}

// PushGoldSyncedDirect sends gold sync message without loading account
func PushGoldSyncedDirect(gold int64) error {
	// This would normally send the message to the connected client
	// For now, just return nil - actual networking handled elsewhere
	return nil
}

// CurrencyError represents a currency-related error
type CurrencyError struct {
	Code    string
	Message string
}

func (e *CurrencyError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// CleanExpiredNonces cleans up old nonces (can be called periodically)
// In production, you'd want a time-based expiration
func CleanExpiredNonces() {
	nonceMutex.Lock()
	defer nonceMutex.Unlock()

	// Simple cleanup - in production, use timestamps and expire old nonces
	// For now, clear all nonces (transactions should rarely have duplicates hour+ later)
	if len(spentNonces) > 1000 { // arbitrary threshold
		spentNonces = make(map[string]bool)
	}
}
