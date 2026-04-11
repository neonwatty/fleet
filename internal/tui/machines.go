package tui

import (
	"fmt"
	"strings"

	"github.com/neonwatty/fleet/internal/machine"
)

func renderMachinesPanel(healths []machine.Health, _ int) string {
	var b strings.Builder

	header := fmt.Sprintf("%-10s %-8s %-10s %-10s %-5s %-10s",
		"MACHINE", "STATUS", "MEM AVAIL", "SWAP USED", "CC", "HEALTH")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for _, h := range healths {
		if !h.Online {
			b.WriteString(fmt.Sprintf("%-10s ", h.Name))
			b.WriteString(offlineStyle.Render(fmt.Sprintf("%-8s", "offline")))
			b.WriteString("\n")
			continue
		}

		availPct := float64(h.AvailMemory) / float64(h.TotalMemory) * 100
		score := machine.Score(h)

		memRaw := fmt.Sprintf("%.0f%%", availPct)
		swapRaw := fmt.Sprintf("%.1fGB", h.SwapUsedMB/1024)
		claudeRaw := fmt.Sprintf("%d", h.ClaudeCount)

		// Pad plain text first, then apply styles to padded strings
		// so ANSI escape codes don't break column alignment
		nameCol := fmt.Sprintf("%-10s ", h.Name)
		statusCol := onlineStyle.Render(fmt.Sprintf("%-8s", "online")) + " "
		claudeCol := fmt.Sprintf("%-5s ", claudeRaw)
		label := machine.ScoreLabel(score)
		var healthCol string
		switch label {
		case "idle":
			healthCol = onlineStyle.Render(fmt.Sprintf("%-10s", label))
		case "ok":
			healthCol = onlineStyle.Render(fmt.Sprintf("%-10s", label))
		case "busy":
			healthCol = warnStyle.Render(fmt.Sprintf("%-10s", label))
		default:
			healthCol = offlineStyle.Render(fmt.Sprintf("%-10s", label))
		}

		var memCol, swapCol string
		if availPct < 25 {
			memCol = warnStyle.Render(fmt.Sprintf("%-10s", memRaw)) + " "
		} else {
			memCol = fmt.Sprintf("%-10s ", memRaw)
		}
		if h.SwapUsedMB > 4096 {
			swapCol = warnStyle.Render(fmt.Sprintf("%-10s", swapRaw)) + " "
		} else {
			swapCol = fmt.Sprintf("%-10s ", swapRaw)
		}

		b.WriteString(nameCol + statusCol + memCol + swapCol + claudeCol + healthCol)
		b.WriteString("\n")
	}

	return b.String()
}
