package tui

import (
	"fmt"
	"strings"

	"github.com/neonwatty/fleet/internal/machine"
)

func renderMachinesPanel(healths []machine.Health, _ int) string {
	var b strings.Builder

	header := fmt.Sprintf("%-10s %-8s %-10s %-10s %-8s %-6s",
		"MACHINE", "STATUS", "MEM AVAIL", "SWAP USED", "CLAUDE", "SCORE")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for _, h := range healths {
		if !h.Online {
			line := fmt.Sprintf("%-10s %-8s",
				h.Name, offlineStyle.Render("offline"))
			b.WriteString(line)
			b.WriteString("\n")
			continue
		}

		availPct := float64(h.AvailMemory) / float64(h.TotalMemory) * 100
		score := machine.Score(h)

		status := onlineStyle.Render("online")
		memStr := fmt.Sprintf("%.0f%%", availPct)
		swapStr := fmt.Sprintf("%.0fMB", h.SwapUsedMB)
		claudeStr := fmt.Sprintf("%d", h.ClaudeCount)
		scoreStr := fmt.Sprintf("%.1f", score)

		if availPct < 25 {
			memStr = warnStyle.Render(memStr)
		}
		if h.SwapUsedMB > 4000 {
			swapStr = warnStyle.Render(swapStr)
		}

		line := fmt.Sprintf("%-10s %-8s %-10s %-10s %-8s %-6s",
			h.Name, status, memStr, swapStr, claudeStr, scoreStr)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}
