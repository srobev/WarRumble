package game

import (
    "encoding/json"
    "os"
)

// xpTable returns per-level XP required for levels 1->2, 2->3, ..., 30->31
// It tries to read a JSON array from ConfigPath("xp_levels.json").
// If missing, it falls back to a reasonable growth curve (placeholder).
func xpTable() []int {
    p := ConfigPath("xp_levels.json")
    if b, err := os.ReadFile(p); err == nil {
        var arr []int
        if json.Unmarshal(b, &arr) == nil && len(arr) >= 30 { // 30 steps to reach 31
            return arr
        }
    }
    // Placeholder exponential-ish growth; replace by real data when available.
    // Level 1->2 starts small, ramps up.
    arr := make([]int, 30)
    base := 150
    mul := 1.15
    val := float64(base)
    for i := 0; i < 30; i++ {
        arr[i] = int(val)
        val *= mul
        if i%5 == 4 { // small bump every 5 levels
            val *= 1.05
        }
    }
    return arr
}

// computeLevel returns (level, cur, next) for totalXP.
// level in [1..31], but note that XP-based max is 24 per design; caller can clamp.
func computeLevel(totalXP int) (int, int, int) {
    tbl := xpTable()
    lvl := 1
    for i := 0; i < len(tbl); i++ {
        need := tbl[i]
        if totalXP < need {
            return lvl, totalXP, need
        }
        totalXP -= need
        lvl++
    }
    // reached top of table
    return len(tbl)+1, 0, 0
}
