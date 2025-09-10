package balance

import "time"

// V1_3_APPLIED: true  (sentinel for v1.3)

var (
    // Economy
    PerkPriceGold             = 250
    MaxSimultaneousPerkOffers = 2

    // Combat cadence
    AuraTick = 200 * time.Millisecond

    // GRID reroll rate limit (token bucket)
    GridRerollRefillSec = 15.0  // 1 token every 15s
    GridRerollBurst     = 2.0   // allow 2 quick rerolls

    // Pity: guarantee >= 1 perk for an eligible unit every N rerolls
    PerkPityEveryRerolls = 4
)
