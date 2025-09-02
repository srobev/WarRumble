package srv

import "math"

// xpTable returns per-level XP required for levels 1->2, ..., 30->31.
// Placeholder growth matching client fallback; replace with authoritative data later.
func xpTable() []int {
    arr := make([]int, 30)
    base := 150.0
    mul := 1.15
    val := base
    for i := 0; i < 30; i++ {
        arr[i] = int(val)
        val *= mul
        if i%5 == 4 { val *= 1.05 }
    }
    return arr
}

// computeLevel returns (level, cur, next) given totalXP across levels.
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
    return len(tbl)+1, 0, 0
}

func xpDeltaForRate(current int, rate float64) int {
    _, _, need := computeLevel(current)
    if need <= 0 { return 0 }
    d := int(math.Ceil(float64(need) * rate))
    if d < 1 { d = 1 }
    return d
}

