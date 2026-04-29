package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/session"
)

func TestStatusJSONWithFakeSSH(t *testing.T) {
	dir := t.TempDir()
	installFakeSSH(t, dir, `#!/bin/bash
set -euo pipefail
cmd="${@: -1}"
if [[ "$cmd" == *"vm_stat"* ]]; then
  cat <<'OUT'
Mach Virtual Memory Statistics: (page size of 4096 bytes)
Pages free:                               1000.
Pages inactive:                           3000.
===SWAP===
vm.swapusage: total = 1024.00M  used = 128.00M  free = 896.00M  (encrypted)
===MEM===
17179869184
===CLAUDE===
2
OUT
  exit 0
fi
if [[ "$cmd" == *"ps -eo rss,pid,command"* ]]; then
  printf '2048 4242 claude\n1024 5000 node vite\n'
  exit 0
fi
exit 0
`)

	statePath := filepath.Join(dir, "state.json")
	if err := session.Save(statePath, &session.State{
		Sessions: []session.Session{{
			ID:           "s1",
			Project:      "org/repo",
			Machine:      "mm1",
			Branch:       "main",
			Account:      "personal",
			WorktreePath: "/tmp/fleet-work/repo-1",
			Tunnel:       session.TunnelInfo{LocalPort: 4001, RemotePort: 3000},
			StartedAt:    time.Date(2026, 4, 29, 20, 0, 0, 0, time.UTC),
		}},
		MachineLabels: map[string][]session.MachineLabel{
			"mm1": {{Name: "repo", SessionID: "s1", CreatedAt: time.Now().UTC()}},
		},
	}); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	cfg := &config.Config{
		Settings: config.Settings{
			SwapWarnMB: 1024,
			SwapHighMB: 4096,
		},
		Machines: []config.Machine{{Name: "mm1", Host: "mm1", User: "me", Enabled: true}},
	}

	out := captureStdout(t, func() {
		if err := runStatusJSON(cfg, statePath); err != nil {
			t.Fatalf("runStatusJSON() error: %v", err)
		}
	})

	var doc statusDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("decode status JSON: %v\n%s", err, out)
	}
	if len(doc.Machines) != 1 {
		t.Fatalf("len(Machines) = %d, want 1", len(doc.Machines))
	}
	m := doc.Machines[0]
	if m.Name != "mm1" || m.Status != "online" || m.CCCount != 2 {
		t.Fatalf("machine status = %+v, want online mm1 with 2 CC", m)
	}
	if len(m.Labels) != 1 || !m.Labels[0].Live {
		t.Fatalf("labels = %+v, want live linked label", m.Labels)
	}
}

func installFakeSSH(t *testing.T, dir, script string) {
	t.Helper()
	path := filepath.Join(dir, "ssh")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return strings.TrimSpace(buf.String())
}
