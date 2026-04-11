package tui

import (
	"fmt"
	"strings"

	"github.com/neonwatty/fleet/internal/machine"
)

func renderProcessesPanel(machineName string, groups []machine.ProcessGroup, selectedRow int) string {
	if machineName == "" {
		return dimStyle.Render("Select a machine to view processes")
	}

	if len(groups) == 0 {
		return dimStyle.Render(fmt.Sprintf("No significant processes on %s", machineName))
	}

	var b strings.Builder

	header := fmt.Sprintf("%-14s %-6s %-10s %-10s %-20s",
		"CATEGORY", "COUNT", "RSS", "SWAP", "DETAIL")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for i, g := range groups {
		category := g.Name
		count := fmt.Sprintf("%d", g.Count)
		rss := formatRSS(g.TotalRSS)

		detail := g.Detail
		if g.Name == "Chrome" {
			detail = fmt.Sprintf("%d tabs/procs", g.Count)
		}
		if g.Name == "Docker" {
			detail = fmt.Sprintf("%d procs", g.Count)
		}
		if g.Name == "System" {
			detail = fmt.Sprintf("%d services", g.Count)
		}

		categoryCol := fmt.Sprintf("%-14s ", category)
		countCol := fmt.Sprintf("%-6s ", count)
		detailCol := fmt.Sprintf("%-20s", detail)

		var rssCol string
		if g.TotalRSS > 500*1024 {
			rssCol = warnStyle.Render(fmt.Sprintf("%-10s", rss)) + " "
		} else {
			rssCol = fmt.Sprintf("%-10s ", rss)
		}

		var swapCol string
		if g.TotalSwap < 0 {
			swapCol = dimStyle.Render(fmt.Sprintf("%-10s", "—")) + " "
		} else if g.TotalSwap > 500*1024 {
			swapCol = warnStyle.Render(fmt.Sprintf("%-10s", formatRSS(g.TotalSwap))) + " "
		} else {
			swapCol = fmt.Sprintf("%-10s ", formatRSS(g.TotalSwap))
		}

		line := categoryCol + countCol + rssCol + swapCol + detailCol

		if i == selectedRow {
			if !g.Killable {
				line = dimStyle.Render("> " + line)
			} else {
				line = "> " + line
			}
		} else {
			if !g.Killable {
				line = dimStyle.Render("  " + line)
			} else {
				line = "  " + line
			}
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func formatRSS(kb int) string {
	if kb >= 1024*1024 {
		return fmt.Sprintf("%.1fGB", float64(kb)/(1024*1024))
	}
	return fmt.Sprintf("%dMB", kb/1024)
}
