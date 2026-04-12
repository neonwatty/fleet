package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
)

type statusDoc struct {
	Version   string          `json:"version"`
	Timestamp string          `json:"timestamp"`
	Machines  []machineStatus `json:"machines"`
	Sessions  []sessionStatus `json:"sessions"`
}

type machineStatus struct {
	Name        string        `json:"name"`
	Status      string        `json:"status"` // "online" | "offline"
	MemAvailPct int           `json:"mem_available_pct"`
	SwapGB      float64       `json:"swap_gb"`
	CCCount     int           `json:"cc_count"`
	Score       float64       `json:"score"`
	Health      string        `json:"health"`
	Accounts    []string      `json:"accounts"`
	Labels      []labelStatus `json:"labels"`
}

type labelStatus struct {
	Name      string `json:"name"`
	Live      bool   `json:"live"`
	SessionID string `json:"session_id,omitempty"`
}

type sessionStatus struct {
	ID               string `json:"id"`
	Project          string `json:"project"`
	Machine          string `json:"machine"`
	Branch           string `json:"branch"`
	Account          string `json:"account,omitempty"`
	Label            string `json:"label,omitempty"`
	TunnelLocalPort  int    `json:"tunnel_local_port"`
	TunnelRemotePort int    `json:"tunnel_remote_port"`
	StartedAt        string `json:"started_at"`
}

func buildStatusJSON(
	healths []machine.Health,
	sessions []session.Session,
	labels map[string][]session.MachineLabel,
	ccPIDs map[string][]int,
	now time.Time,
) statusDoc {
	doc := statusDoc{
		Version:   "1",
		Timestamp: now.UTC().Format(time.RFC3339),
		Machines:  []machineStatus{},
		Sessions:  []sessionStatus{},
	}

	liveSessionIDs := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		liveSessionIDs[s.ID] = true
	}

	for _, h := range healths {
		ms := machineStatus{
			Name:     h.Name,
			Accounts: accountsForMachine(h.Name, sessions),
			Labels:   labelStatusList(labels[h.Name], ccPIDs[h.Name], liveSessionIDs),
		}
		if !h.Online {
			ms.Status = "offline"
			doc.Machines = append(doc.Machines, ms)
			continue
		}
		ms.Status = "online"
		if h.TotalMemory > 0 {
			ms.MemAvailPct = int(float64(h.AvailMemory) / float64(h.TotalMemory) * 100)
		}
		ms.SwapGB = h.SwapUsedMB / 1024
		ms.CCCount = h.ClaudeCount
		ms.Score = machine.Score(h)
		ms.Health = machine.ScoreLabel(ms.Score)
		doc.Machines = append(doc.Machines, ms)
	}

	for _, s := range sessions {
		doc.Sessions = append(doc.Sessions, sessionStatus{
			ID:               s.ID,
			Project:          s.Project,
			Machine:          s.Machine,
			Branch:           s.Branch,
			Account:          s.Account,
			Label:            sessionLabelName(labels, s),
			TunnelLocalPort:  s.Tunnel.LocalPort,
			TunnelRemotePort: s.Tunnel.RemotePort,
			StartedAt:        s.StartedAt.UTC().Format(time.RFC3339),
		})
	}

	return doc
}

func accountsForMachine(name string, sessions []session.Session) []string {
	seen := make(map[string]struct{})
	out := []string{}
	for _, s := range sessions {
		if s.Machine != name || s.Account == "" {
			continue
		}
		if _, ok := seen[s.Account]; ok {
			continue
		}
		seen[s.Account] = struct{}{}
		out = append(out, s.Account)
	}
	return out
}

func labelStatusList(labels []session.MachineLabel, livePIDs []int, liveSessionIDs map[string]bool) []labelStatus {
	out := make([]labelStatus, 0, len(labels))
	for _, l := range labels {
		live := session.IsLabelLive(l, liveSessionIDs, livePIDs)
		out = append(out, labelStatus{Name: l.Name, Live: live, SessionID: l.SessionID})
	}
	return out
}

func sessionLabelName(labels map[string][]session.MachineLabel, s session.Session) string {
	for _, l := range labels[s.Machine] {
		if l.SessionID == s.ID {
			return l.Name
		}
	}
	return ""
}

// runStatusJSON is called from the status command when --json is set.
func runStatusJSON(cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	enabled := cfg.EnabledMachines()
	healths := machine.ProbeAll(ctx, enabled)

	state, err := session.LoadState(session.DefaultStatePath())
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	ccPIDs := make(map[string][]int)
	for _, m := range enabled {
		groups := machine.ProbeProcesses(ctx, m)
		for _, g := range groups {
			if g.Name == "Claude Code" {
				ccPIDs[m.Name] = append(ccPIDs[m.Name], g.PIDs...)
			}
		}
	}

	doc := buildStatusJSON(healths, state.Sessions, state.MachineLabels, ccPIDs, time.Now())

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
