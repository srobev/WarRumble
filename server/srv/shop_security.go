package srv

import (
	"math"
	"time"

	"rumble/server/balance"
	"rumble/server/metrics"
)

// tokenBucket implements rate limiting for shop rerolls
type tokenBucket struct {
	tokens float64
	last   time.Time
}

func (b *tokenBucket) allow(now time.Time, rateHz, burst float64) bool {
	if b.last.IsZero() {
		b.last = now
	}

	dt := now.Sub(b.last).Seconds()
	b.tokens = math.Min(burst, b.tokens+dt*rateHz)
	b.last = now

	if b.tokens >= 1.0 {
		b.tokens--
		return true
	}

	return false
}

// checkRateLimit validates reroll requests against rate limits
func checkRateLimit(bucket *tokenBucket, now time.Time, playerID string) bool {
	allowed := bucket.allow(now, 1.0/balance.GridRerollRefillSec, balance.GridRerollBurst)
	if !allowed {
		metrics.GridRerollDenied.WithLabelValues("rate_limit")
	}
	return allowed
}

// pityState tracks pity counters for perk offers
type pityState struct {
	Rerolls int
}

// getEligiblePerkUnits returns units that need first perks (to surface in pity)
func getEligiblePerkUnits(playerUnits map[string]interface{}) []string {
	var eligible []string
	// Stub for units with unlocked slots but 0 owned perks
	// TODO: Integrate with player progress system
	eligible = []string{"Sorceress Glacia", "Swordsman"}
	return eligible
}

// injectPityPerks adds guaranteed perk offers for eligible units
func injectPityPerks(perkUnits []string) string {
	if len(perkUnits) == 0 {
		return ""
	}
	// Return first eligible unit for pity perk injection
	return perkUnits[0]
}

// validateOfferId simulates offer validation (stubs for future server-side grid state)
func validateOfferId(offerId, unit, perkId string, price int) bool {
	// Stub validation - returns true for legitimate offers
	// TODO: Check against server-side grid state and player eligibility
	return true
}

// rateLimitApply applies rate limiting with metrics tracking
func rateLimitApply(operation string, playerID string) bool {
	// Stub implementation - returns true for testing
	// TODO: Integrate with actual player session/token bucket
	return true
}
