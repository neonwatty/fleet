package machine

import (
	"testing"
)

func TestScore(t *testing.T) {
	tests := []struct {
		name     string
		health   Health
		wantSign string
	}{
		{
			name: "healthy machine",
			health: Health{
				Online:      true,
				TotalMemory: 16 * 1024 * 1024 * 1024,
				AvailMemory: 12 * 1024 * 1024 * 1024,
				SwapTotalMB: 4096,
				SwapUsedMB:  0,
				ClaudeCount: 0,
			},
			wantSign: "positive",
		},
		{
			name: "stressed machine",
			health: Health{
				Online:      true,
				TotalMemory: 16 * 1024 * 1024 * 1024,
				AvailMemory: 1 * 1024 * 1024 * 1024,
				SwapTotalMB: 7168,
				SwapUsedMB:  6614,
				ClaudeCount: 5,
			},
			wantSign: "negative",
		},
		{
			name: "offline machine",
			health: Health{
				Online: false,
			},
			wantSign: "negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := Score(tt.health)
			switch tt.wantSign {
			case "positive":
				if score <= 0 {
					t.Errorf("Score() = %f, want positive", score)
				}
			case "negative":
				if score >= 0 {
					t.Errorf("Score() = %f, want negative", score)
				}
			}
		})
	}
}

func TestPickBest(t *testing.T) {
	healths := []Health{
		{Name: "mm1", Online: true, TotalMemory: 16e9, AvailMemory: 4e9, SwapTotalMB: 4096, SwapUsedMB: 2000, ClaudeCount: 3},
		{Name: "mm2", Online: true, TotalMemory: 16e9, AvailMemory: 12e9, SwapTotalMB: 4096, SwapUsedMB: 100, ClaudeCount: 0},
		{Name: "mm3", Online: false},
	}

	best, score := PickBest(healths)
	if best.Name != "mm2" {
		t.Errorf("PickBest() = %q, want mm2", best.Name)
	}
	if score <= 0 {
		t.Errorf("score = %f, want positive", score)
	}
}

func TestPickBestAllOffline(t *testing.T) {
	healths := []Health{
		{Name: "mm1", Online: false},
		{Name: "mm2", Online: false},
	}

	_, score := PickBest(healths)
	if score > -999 {
		t.Errorf("score = %f, want <= -999 (all offline)", score)
	}
}
