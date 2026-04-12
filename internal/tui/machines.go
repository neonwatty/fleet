package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
)

func renderMachinesPanel(
	healths []machine.Health,
	sessions []session.Session,
	labels map[string][]session.MachineLabel,
	ccPIDs map[string][]int,
	liveSessionIDs map[string]bool,
	_ int,
) string {
	var b strings.Builder

	header := fmt.Sprintf("%-22s %-8s %-10s %-10s %-5s %-10s %s",
		"MACHINE", "STATUS", "MEM AVAIL", "SWAP USED", "CC", "HEALTH", "LABELS")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for _, h := range healths {
		nameCell := machineNameCell(h.Name, sessions)

		if !h.Online {
			fmt.Fprintf(&b, "%-22s ", nameCell)
			b.WriteString(offlineStyle.Render(fmt.Sprintf("%-8s", "offline")))
			b.WriteString("  ")
			b.WriteString(formatLabelList(labels[h.Name], nil, liveSessionIDs))
			b.WriteString("\n")
			continue
		}

		availPct := float64(h.AvailMemory) / float64(h.TotalMemory) * 100
		score := machine.Score(h)

		memRaw := fmt.Sprintf("%.0f%%", availPct)
		swapRaw := fmt.Sprintf("%.1fGB", h.SwapUsedMB/1024)
		claudeRaw := fmt.Sprintf("%d", h.ClaudeCount)

		nameCol := fmt.Sprintf("%-22s ", nameCell)
		statusCol := onlineStyle.Render(fmt.Sprintf("%-8s", "online")) + " "
		claudeCol := fmt.Sprintf("%-5s ", claudeRaw)
		label := machine.ScoreLabel(score)
		var healthCol string
		switch label {
		case "free":
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
		b.WriteString("  ")
		b.WriteString(formatLabelList(labels[h.Name], ccPIDs[h.Name], liveSessionIDs))
		b.WriteString("\n")
	}

	return b.String()
}

// machineNameCell returns the machine name with an optional bracketed account
// suffix aggregated from live sessions.
func machineNameCell(name string, sessions []session.Session) string {
	accounts := make(map[string]struct{})
	for _, s := range sessions {
		if s.Machine == name && s.Account != "" {
			accounts[s.Account] = struct{}{}
		}
	}
	if len(accounts) == 0 {
		return name
	}
	names := make([]string, 0, len(accounts))
	for a := range accounts {
		names = append(names, a)
	}
	sort.Strings(names)
	return name + " [" + strings.Join(names, ",") + "]"
}

// formatLabelList renders labels as "live1, live2, stale1(stale)".
// A linked label (non-empty SessionID) is live iff its session still exists
// in the liveSessionIDs set. An orphan label (empty SessionID) is live iff
// its LastSeenPID matches one of the currently observed CC PIDs on the machine.
func formatLabelList(labels []session.MachineLabel, livePIDs []int, liveSessionIDs map[string]bool) string {
	if len(labels) == 0 {
		return dimStyle.Render(emDash)
	}
	parts := make([]string, 0, len(labels))
	for _, l := range labels {
		if session.IsLabelLive(l, liveSessionIDs, livePIDs) {
			parts = append(parts, l.Name)
		} else {
			parts = append(parts, dimStyle.Render(l.Name+"(stale)"))
		}
	}
	return strings.Join(parts, ", ")
}
