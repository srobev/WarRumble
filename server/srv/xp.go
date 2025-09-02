package srv

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
)

// xpTable returns per-level XP required for levels 1->2, ..., 19->20.
func xpTable() []int {
	// Try to read from server/data/xp_levels.json
	dataDir := filepath.Join("server", "data")
	p := filepath.Join(dataDir, "xp_levels.json")
	if b, err := os.ReadFile(p); err == nil {
		var arr []int
		if json.Unmarshal(b, &arr) == nil && len(arr) >= 19 {
			return arr
		}
	}
	// Fallback to hardcoded values
	return []int{
		2,      // 1->2
		5,      // 2->3
		10,     // 3->4
		20,     // 4->5
		35,     // 5->6
		65,     // 6->7
		120,    // 7->8
		210,    // 8->9
		375,    // 9->10
		675,    // 10->11
		1200,   // 11->12
		2100,   // 12->13
		3750,   // 13->14
		6500,   // 14->15
		12000,  // 15->16
		25000,  // 16->17
		50000,  // 17->18
		100000, // 18->19
		200000, // 19->20
	}
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
	return len(tbl) + 1, 0, 0
}

func xpDeltaForRate(current int, rate float64) int {
	_, _, need := computeLevel(current)
	if need <= 0 {
		return 0
	}
	d := int(math.Ceil(float64(need) * rate))
	if d < 1 {
		d = 1
	}
	return d
}
