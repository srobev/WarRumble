package game

// xpTable returns per-level XP required for levels 1->2, 2->3, ..., 19->20
func xpTable() []int {
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

// computeLevel returns (level, cur, next) for totalXP.
// level in [1..20], max level is 20.
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
	return len(tbl) + 1, 0, 0
}

// computeEffectiveLevel combines XP-based level with rank progression
// Returns (effectiveLevel, baseXPLevel, rank, perkSlotsUnlocked, legendaryUnlocked)
func computeEffectiveLevel(totalXP int, rank int, perkSlotsUnlocked int, legendaryUnlocked bool) (int, int, int, int, bool) {
	baseLevel, _, _ := computeLevel(totalXP)
	effectiveLevel := baseLevel + rank - 1 // rank 1 = +0, rank 2 = +1, etc.

	return effectiveLevel, baseLevel, rank, perkSlotsUnlocked, legendaryUnlocked
}
