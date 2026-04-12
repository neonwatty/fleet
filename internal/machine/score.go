package machine

const offlineScore = -1000.0

func Score(h Health) float64 {
	if !h.Online {
		return offlineScore
	}

	if h.TotalMemory == 0 {
		return offlineScore
	}

	availPct := float64(h.AvailMemory) / float64(h.TotalMemory) * 100

	var swapPenalty float64
	if h.SwapTotalMB > 0 {
		swapPenalty = (h.SwapUsedMB / h.SwapTotalMB) * 100 * 0.5
	}

	claudePenalty := float64(h.ClaudeCount) * 5

	return availPct - swapPenalty - claudePenalty
}

func ScoreLabel(score float64) string {
	switch {
	case score >= 30:
		return "free"
	case score >= 10:
		return "ok"
	case score >= -20:
		return "busy"
	default:
		return "stressed"
	}
}

func PickBest(healths []Health) (Health, float64) {
	bestScore := offlineScore - 1
	bestIdx := 0

	for i, h := range healths {
		s := Score(h)
		if s > bestScore {
			bestScore = s
			bestIdx = i
		}
	}

	return healths[bestIdx], bestScore
}
