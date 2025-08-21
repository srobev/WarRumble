package srv

import (
	"math"
)

const eloK = 24

func eloExpected(ra, rb int) float64 {
	return 1 / (1 + math.Pow(10, float64(rb-ra)/400))
}

func eloApply(ra, rb int, win bool) (newA, delta int) {
	E := eloExpected(ra, rb)
	S := 0.0
	if win {
		S = 1.0
	}
	d := int(math.Round(eloK * (S - E)))
	nr := ra + d
	if nr < 0 {
		nr = 0
	}
	if nr > 9999 {
		nr = 9999
	}
	return nr, d
}

func rankName(r int) string {
	switch {
	case r >= 3100:
		return "Mythic"
	case r >= 2900:
		return "Legend"
	case r >= 2700:
		return "Grandmaster"
	case r >= 2500:
		return "General"
	case r >= 2300:
		return "Marshal"
	case r >= 2100:
		return "Commander"
	case r >= 1900:
		return "Champion"
	case r >= 1700:
		return "Warlord"
	case r >= 1500:
		return "Centurion"
	case r >= 1300:
		return "Captain"
	case r >= 1100:
		return "Knight"
	case r >= 900:
		return "Ranger"
	case r >= 700:
		return "Grunt"
	case r >= 400:
		return "Footman"
	default:
		return "Recruit"
	}
}

