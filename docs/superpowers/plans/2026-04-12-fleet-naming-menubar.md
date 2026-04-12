# Fleet — Session Labels, Accounts, and SwiftBar Menu Bar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add machine-scoped session labels, per-session account tracking, and a SwiftBar-driven menu bar indicator to fleet, powered by a new `fleet status --json` output mode.

**Architecture:** All state lives on the MBA hub in `~/.fleet/state.json`. Labels are a new top-level `machine_labels` map keyed by machine name; accounts are a new field on `Session`. A new `fleet status --json` mode serializes both for consumption by a SwiftBar plugin shell script. TUI rendering is extended in place; no new TUI screens.

**Tech Stack:** Go 1.26, Cobra CLI, Bubble Tea TUI, BurntSushi TOML, SwiftBar (shell + jq).

**Spec reference:** [`docs/superpowers/specs/2026-04-12-fleet-naming-menubar-design.md`](../specs/2026-04-12-fleet-naming-menubar-design.md)

**Visual mockup:** [`docs/superpowers/mockups/2026-04-12-menu-bar.html`](../mockups/2026-04-12-menu-bar.html)

---

## Task 1: Extend state schema with `MachineLabel` type and `Session.Account` field

**Files:**
- Modify: `internal/session/state.go`
- Test: `internal/session/state_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/session/state_test.go`:

```go
func TestStateRoundTripWithLabelsAndAccounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	created := time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC)
	s := &State{
		Sessions: []Session{
			{
				ID:      "abc123",
				Project: "neonwatty/bleep",
				Machine: "mm1",
				Branch:  "main",
				Account: "personal-max",
			},
		},
		MachineLabels: map[string][]MachineLabel{
			"mm1": {
				{Name: "bleep", SessionID: "abc123", CreatedAt: created, LastSeenPID: 4242},
				{Name: "deckchecker", SessionID: "", CreatedAt: created, LastSeenPID: 0},
			},
		},
	}

	if err := Save(path, s); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}

	if loaded.Sessions[0].Account != "personal-max" {
		t.Errorf("Account = %q, want %q", loaded.Sessions[0].Account, "personal-max")
	}
	if len(loaded.MachineLabels["mm1"]) != 2 {
		t.Fatalf("len(MachineLabels[mm1]) = %d, want 2", len(loaded.MachineLabels["mm1"]))
	}
	if loaded.MachineLabels["mm1"][0].Name != "bleep" {
		t.Errorf("label[0].Name = %q, want %q", loaded.MachineLabels["mm1"][0].Name, "bleep")
	}
	if loaded.MachineLabels["mm1"][0].SessionID != "abc123" {
		t.Errorf("label[0].SessionID = %q, want %q", loaded.MachineLabels["mm1"][0].SessionID, "abc123")
	}
	if loaded.MachineLabels["mm1"][1].SessionID != "" {
		t.Errorf("label[1].SessionID = %q, want empty (orphan)", loaded.MachineLabels["mm1"][1].SessionID)
	}
}

func TestLoadStateBackCompatNoLabels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	legacy := `{"sessions":[{"id":"x","project":"p","machine":"m","branch":"main","worktree_path":"/tmp","tunnel":{"local_port":0,"remote_port":0},"started_at":"2026-04-11T08:00:00Z","pid":1}]}`
	if err := os.WriteFile(path, []byte(legacy), 0644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if len(loaded.Sessions) != 1 {
		t.Fatalf("len(Sessions) = %d, want 1", len(loaded.Sessions))
	}
	if loaded.Sessions[0].Account != "" {
		t.Errorf("legacy Account = %q, want empty", loaded.Sessions[0].Account)
	}
	if loaded.MachineLabels != nil && len(loaded.MachineLabels) > 0 {
		t.Errorf("legacy MachineLabels should be nil/empty, got %v", loaded.MachineLabels)
	}
}
```

Also add `"os"` to the imports if not already present.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./internal/session/ -run TestStateRoundTripWithLabelsAndAccounts -v`

Expected: FAIL with errors about `Account`, `MachineLabels`, or `MachineLabel` being undefined.

- [ ] **Step 3: Implement minimal schema changes**

Modify `internal/session/state.go` by changing the `State` and `Session` structs and adding the new `MachineLabel` type. The final top of the file should read:

```go
package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	Sessions      []Session                 `json:"sessions"`
	MachineLabels map[string][]MachineLabel `json:"machine_labels,omitempty"`
}

type Session struct {
	ID           string     `json:"id"`
	Project      string     `json:"project"`
	Machine      string     `json:"machine"`
	Branch       string     `json:"branch"`
	Account      string     `json:"account,omitempty"`
	WorktreePath string     `json:"worktree_path"`
	Tunnel       TunnelInfo `json:"tunnel"`
	StartedAt    time.Time  `json:"started_at"`
	PID          int        `json:"pid"`
}

type MachineLabel struct {
	Name        string    `json:"name"`
	SessionID   string    `json:"session_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeenPID int       `json:"last_seen_pid,omitempty"`
}

type TunnelInfo struct {
	LocalPort  int `json:"local_port"`
	RemotePort int `json:"remote_port"`
}
```

Leave the rest of the file unchanged.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./internal/session/ -v`

Expected: all tests PASS, including the two new ones.

- [ ] **Step 5: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add internal/session/state.go internal/session/state_test.go
git commit -m "feat(session): add MachineLabel type and Session.Account field"
```

---

## Task 2: Label mutation helpers in `internal/session/labels.go`

**Files:**
- Create: `internal/session/labels.go`
- Create: `internal/session/labels_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/session/labels_test.go`:

```go
package session

import (
	"path/filepath"
	"testing"
)

func TestAddLabelOrphan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := AddLabel(path, "mm1", "bleep", "", 0); err != nil {
		t.Fatalf("AddLabel() error: %v", err)
	}

	s, _ := LoadState(path)
	if len(s.MachineLabels["mm1"]) != 1 {
		t.Fatalf("len = %d, want 1", len(s.MachineLabels["mm1"]))
	}
	if s.MachineLabels["mm1"][0].Name != "bleep" {
		t.Errorf("Name = %q, want %q", s.MachineLabels["mm1"][0].Name, "bleep")
	}
	if s.MachineLabels["mm1"][0].SessionID != "" {
		t.Errorf("SessionID = %q, want empty (orphan)", s.MachineLabels["mm1"][0].SessionID)
	}
}

func TestAddLabelLinkedToSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := AddLabel(path, "mm1", "bleep", "sess-123", 4242); err != nil {
		t.Fatalf("AddLabel() error: %v", err)
	}

	s, _ := LoadState(path)
	if s.MachineLabels["mm1"][0].SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want sess-123", s.MachineLabels["mm1"][0].SessionID)
	}
	if s.MachineLabels["mm1"][0].LastSeenPID != 4242 {
		t.Errorf("LastSeenPID = %d, want 4242", s.MachineLabels["mm1"][0].LastSeenPID)
	}
}

func TestAddLabelDuplicateOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	_ = AddLabel(path, "mm1", "bleep", "s1", 1)
	_ = AddLabel(path, "mm1", "bleep", "s2", 2)

	s, _ := LoadState(path)
	if len(s.MachineLabels["mm1"]) != 1 {
		t.Fatalf("len = %d, want 1 (duplicate should overwrite)", len(s.MachineLabels["mm1"]))
	}
	if s.MachineLabels["mm1"][0].SessionID != "s2" {
		t.Errorf("SessionID = %q, want s2", s.MachineLabels["mm1"][0].SessionID)
	}
}

func TestRemoveLabel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	_ = AddLabel(path, "mm1", "bleep", "", 0)
	_ = AddLabel(path, "mm1", "deckchecker", "", 0)

	if err := RemoveLabel(path, "mm1", "bleep"); err != nil {
		t.Fatalf("RemoveLabel() error: %v", err)
	}

	s, _ := LoadState(path)
	if len(s.MachineLabels["mm1"]) != 1 {
		t.Fatalf("len = %d, want 1", len(s.MachineLabels["mm1"]))
	}
	if s.MachineLabels["mm1"][0].Name != "deckchecker" {
		t.Errorf("Name = %q, want deckchecker", s.MachineLabels["mm1"][0].Name)
	}
}

func TestClearLabels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	_ = AddLabel(path, "mm1", "a", "", 0)
	_ = AddLabel(path, "mm1", "b", "", 0)

	if err := ClearLabels(path, "mm1"); err != nil {
		t.Fatalf("ClearLabels() error: %v", err)
	}

	s, _ := LoadState(path)
	if len(s.MachineLabels["mm1"]) != 0 {
		t.Errorf("len = %d, want 0 after clear", len(s.MachineLabels["mm1"]))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./internal/session/ -run TestAddLabel -v`

Expected: FAIL — `AddLabel`, `RemoveLabel`, `ClearLabels` undefined.

- [ ] **Step 3: Implement `internal/session/labels.go`**

Create `internal/session/labels.go`:

```go
package session

import (
	"fmt"
	"time"
)

// AddLabel creates or updates a label on the given machine. If a label with
// the same name already exists it is overwritten (last-write-wins). sessionID
// and lastSeenPID are optional (pass "" and 0 for an orphan label).
func AddLabel(statePath, machineName, labelName, sessionID string, lastSeenPID int) error {
	if labelName == "" {
		return fmt.Errorf("label name is required")
	}
	s, err := LoadState(statePath)
	if err != nil {
		return err
	}
	if s.MachineLabels == nil {
		s.MachineLabels = make(map[string][]MachineLabel)
	}

	label := MachineLabel{
		Name:        labelName,
		SessionID:   sessionID,
		CreatedAt:   time.Now().UTC(),
		LastSeenPID: lastSeenPID,
	}

	existing := s.MachineLabels[machineName]
	replaced := false
	for i := range existing {
		if existing[i].Name == labelName {
			existing[i] = label
			replaced = true
			break
		}
	}
	if !replaced {
		existing = append(existing, label)
	}
	s.MachineLabels[machineName] = existing

	return Save(statePath, s)
}

// RemoveLabel removes a single label by name from a machine. No error if the
// label does not exist (idempotent).
func RemoveLabel(statePath, machineName, labelName string) error {
	s, err := LoadState(statePath)
	if err != nil {
		return err
	}
	if s.MachineLabels == nil {
		return nil
	}
	existing := s.MachineLabels[machineName]
	filtered := existing[:0]
	for _, l := range existing {
		if l.Name != labelName {
			filtered = append(filtered, l)
		}
	}
	s.MachineLabels[machineName] = filtered
	return Save(statePath, s)
}

// ClearLabels removes all labels from a machine.
func ClearLabels(statePath, machineName string) error {
	s, err := LoadState(statePath)
	if err != nil {
		return err
	}
	if s.MachineLabels == nil {
		return nil
	}
	delete(s.MachineLabels, machineName)
	return Save(statePath, s)
}

// ListLabels returns a copy of the labels for a machine, or nil if none.
func ListLabels(statePath, machineName string) ([]MachineLabel, error) {
	s, err := LoadState(statePath)
	if err != nil {
		return nil, err
	}
	if s.MachineLabels == nil {
		return nil, nil
	}
	out := make([]MachineLabel, len(s.MachineLabels[machineName]))
	copy(out, s.MachineLabels[machineName])
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./internal/session/ -v`

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add internal/session/labels.go internal/session/labels_test.go
git commit -m "feat(session): add label Add/Remove/Clear/List helpers"
```

---

## Task 3: Add `DefaultAccount` to `config.Machine`

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/config/config_test.go`:

```go
func TestLoadConfigWithDefaultAccount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	toml := `
[settings]
port_range = [4000, 4999]
poll_interval = 5
stress_threshold = 20
worktree_base = "/tmp/fleet-work"
bare_repo_base = "/tmp/fleet-repos"

[[machines]]
name = "mm1"
host = "mm1"
user = "neonwatty"
enabled = true
default_account = "personal-max"

[[machines]]
name = "mm2"
host = "mm2"
user = "neonwatty"
enabled = true
`
	if err := os.WriteFile(path, []byte(toml), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Machines[0].DefaultAccount != "personal-max" {
		t.Errorf("mm1 DefaultAccount = %q, want personal-max", cfg.Machines[0].DefaultAccount)
	}
	if cfg.Machines[1].DefaultAccount != "" {
		t.Errorf("mm2 DefaultAccount = %q, want empty", cfg.Machines[1].DefaultAccount)
	}
}
```

Make sure `config_test.go` imports `"os"` and `"path/filepath"` and `"testing"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./internal/config/ -run TestLoadConfigWithDefaultAccount -v`

Expected: FAIL — `DefaultAccount` field is undefined on `Machine`.

- [ ] **Step 3: Add `DefaultAccount` to the struct**

In `internal/config/config.go`, change the `Machine` struct to:

```go
type Machine struct {
	Name           string `toml:"name"`
	Host           string `toml:"host"`
	User           string `toml:"user"`
	Enabled        bool   `toml:"enabled"`
	DefaultAccount string `toml:"default_account"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./internal/config/ -v`

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add default_account per machine"
```

---

## Task 4: Capture account on launch (`LaunchOpts.Account` with config fallback)

**Files:**
- Modify: `internal/session/launch.go`
- Create: `internal/session/account.go`
- Create: `internal/session/account_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/session/account_test.go`:

```go
package session

import (
	"testing"

	"github.com/neonwatty/fleet/internal/config"
)

func TestResolveAccountExplicitWins(t *testing.T) {
	m := config.Machine{Name: "mm1", DefaultAccount: "personal-max"}
	got := ResolveAccount("work-team", m)
	if got != "work-team" {
		t.Errorf("ResolveAccount(explicit) = %q, want work-team", got)
	}
}

func TestResolveAccountFallsBackToMachineDefault(t *testing.T) {
	m := config.Machine{Name: "mm1", DefaultAccount: "personal-max"}
	got := ResolveAccount("", m)
	if got != "personal-max" {
		t.Errorf("ResolveAccount(fallback) = %q, want personal-max", got)
	}
}

func TestResolveAccountEmptyWhenNoneSet(t *testing.T) {
	m := config.Machine{Name: "mm1"}
	got := ResolveAccount("", m)
	if got != "" {
		t.Errorf("ResolveAccount(none) = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./internal/session/ -run TestResolveAccount -v`

Expected: FAIL — `ResolveAccount` undefined.

- [ ] **Step 3: Implement `internal/session/account.go`**

Create `internal/session/account.go`:

```go
package session

import (
	"fmt"

	"github.com/neonwatty/fleet/internal/config"
)

// ResolveAccount returns the account to stamp on a new session. An explicit
// --account flag wins; otherwise the machine's DefaultAccount is used; otherwise
// the result is empty.
func ResolveAccount(explicit string, m config.Machine) string {
	if explicit != "" {
		return explicit
	}
	return m.DefaultAccount
}

// SetSessionAccount updates the Account field of an existing session by ID.
// Returns an error if the session is not found.
func SetSessionAccount(statePath, sessionID, account string) error {
	s, err := LoadState(statePath)
	if err != nil {
		return err
	}
	for i := range s.Sessions {
		if s.Sessions[i].ID == sessionID {
			s.Sessions[i].Account = account
			return Save(statePath, s)
		}
	}
	return fmt.Errorf("session %q not found", sessionID)
}
```

- [ ] **Step 4: Wire Account into `LaunchOpts` and `Launch`**

In `internal/session/launch.go`, change `LaunchOpts`:

```go
type LaunchOpts struct {
	Project   string
	Branch    string
	Account   string
	Machine   config.Machine
	Settings  config.Settings
	StatePath string
}
```

In the same file, inside `Launch`, update the session construction (currently lines ~103–112) to set the account:

```go
	sess := Session{
		ID:           GenerateID(),
		Project:      opts.Project,
		Machine:      opts.Machine.Name,
		Branch:       opts.Branch,
		Account:      ResolveAccount(opts.Account, opts.Machine),
		WorktreePath: remoteWork,
		Tunnel:       TunnelInfo{LocalPort: localPort, RemotePort: devPort},
		StartedAt:    time.Now().UTC(),
		PID:          os.Getpid(),
	}
```

- [ ] **Step 5: Run tests to verify they pass and nothing else broke**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./internal/session/ -v && go build ./...`

Expected: all tests PASS, `go build` succeeds.

- [ ] **Step 6: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add internal/session/account.go internal/session/account_test.go internal/session/launch.go
git commit -m "feat(session): capture account on launch with config fallback"
```

---

## Task 5: `fleet label` subcommand

**Files:**
- Create: `cmd/fleet/label.go`
- Modify: `cmd/fleet/main.go`

- [ ] **Step 1: Create the subcommand file**

Create `cmd/fleet/label.go`:

```go
package main

import (
	"fmt"
	"sort"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/session"
	"github.com/spf13/cobra"
)

func labelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Manage machine-scoped session labels",
	}
	cmd.AddCommand(labelSetCmd())
	cmd.AddCommand(labelListCmd())
	return cmd
}

func labelSetCmd() *cobra.Command {
	var remove bool
	var clear bool
	var sessionID string

	cmd := &cobra.Command{
		Use:   "set <machine> [name]",
		Short: "Add, remove, or clear labels on a machine",
		Long: `Add a new label, remove one, or clear all labels from a machine.

Examples:
  fleet label set mm1 bleep                    # add orphan label
  fleet label set mm1 bleep --session a1b2c3   # add label linked to a session
  fleet label set mm1 bleep --remove           # remove a single label
  fleet label set mm1 --clear                  # remove all labels on mm1
`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.DefaultPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if err := assertKnownMachine(cfg, args[0]); err != nil {
				return err
			}

			statePath := session.DefaultStatePath()
			machineName := args[0]

			if clear {
				if len(args) != 1 {
					return fmt.Errorf("--clear takes no label name")
				}
				return session.ClearLabels(statePath, machineName)
			}
			if len(args) != 2 {
				return fmt.Errorf("label name is required (or use --clear)")
			}
			name := args[1]
			if remove {
				return session.RemoveLabel(statePath, machineName, name)
			}
			return session.AddLabel(statePath, machineName, name, sessionID, 0)
		},
	}
	cmd.Flags().BoolVar(&remove, "remove", false, "Remove the given label")
	cmd.Flags().BoolVar(&clear, "clear", false, "Remove all labels on this machine")
	cmd.Flags().StringVar(&sessionID, "session", "", "Link the label to a session ID")
	return cmd
}

func labelListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [machine]",
		Short: "List labels across the fleet or on one machine",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			statePath := session.DefaultStatePath()
			state, err := session.LoadState(statePath)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}
			if state.MachineLabels == nil {
				fmt.Println("(no labels)")
				return nil
			}
			machines := make([]string, 0, len(state.MachineLabels))
			for name := range state.MachineLabels {
				if len(args) == 1 && name != args[0] {
					continue
				}
				machines = append(machines, name)
			}
			sort.Strings(machines)
			for _, name := range machines {
				fmt.Printf("%s:\n", name)
				for _, l := range state.MachineLabels[name] {
					linkage := "(orphan)"
					if l.SessionID != "" {
						linkage = "session=" + l.SessionID
					}
					fmt.Printf("  - %s  %s\n", l.Name, linkage)
				}
			}
			return nil
		},
	}
}

func assertKnownMachine(cfg *config.Config, name string) error {
	for _, m := range cfg.Machines {
		if m.Name == name {
			return nil
		}
	}
	known := make([]string, 0, len(cfg.Machines))
	for _, m := range cfg.Machines {
		known = append(known, m.Name)
	}
	return fmt.Errorf("unknown machine %q; known: %v", name, known)
}
```

- [ ] **Step 2: Register the subcommand**

In `cmd/fleet/main.go`, add to the `main` function after the existing `AddCommand` lines:

```go
	root.AddCommand(labelCmd())
```

- [ ] **Step 3: Build and smoke test**

Run:

```bash
cd /Users/jeremywatt/Desktop/fleet && go build -o /tmp/fleet ./cmd/fleet && /tmp/fleet label --help
```

Expected: help output lists `set` and `list` subcommands with no build errors.

- [ ] **Step 4: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add cmd/fleet/label.go cmd/fleet/main.go
git commit -m "feat(cli): add fleet label set/list subcommands"
```

---

## Task 6: `fleet account` subcommand

**Files:**
- Create: `cmd/fleet/account.go`
- Modify: `cmd/fleet/main.go`

- [ ] **Step 1: Create the subcommand file**

Create `cmd/fleet/account.go`:

```go
package main

import (
	"fmt"

	"github.com/neonwatty/fleet/internal/session"
	"github.com/spf13/cobra"
)

func accountCmd() *cobra.Command {
	var clear bool

	cmd := &cobra.Command{
		Use:   "account <session-id> [name]",
		Short: "Set or clear the Claude account assigned to a session",
		Long: `Set or clear the Claude account label on an existing session.

Session IDs are matched exactly.

Examples:
  fleet account a1b2c3d4 personal-max
  fleet account a1b2c3d4 --clear
`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]
			statePath := session.DefaultStatePath()
			if clear {
				if len(args) != 1 {
					return fmt.Errorf("--clear takes no account name")
				}
				return session.SetSessionAccount(statePath, sessionID, "")
			}
			if len(args) != 2 {
				return fmt.Errorf("account name is required (or use --clear)")
			}
			return session.SetSessionAccount(statePath, sessionID, args[1])
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "Unset the account on this session")
	return cmd
}
```

- [ ] **Step 2: Register the subcommand**

In `cmd/fleet/main.go`, add to the `main` function:

```go
	root.AddCommand(accountCmd())
```

- [ ] **Step 3: Build and smoke test**

Run: `cd /Users/jeremywatt/Desktop/fleet && go build -o /tmp/fleet ./cmd/fleet && /tmp/fleet account --help`

Expected: help output shows the account command with `--clear` flag.

- [ ] **Step 4: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add cmd/fleet/account.go cmd/fleet/main.go
git commit -m "feat(cli): add fleet account subcommand"
```

---

## Task 7: Add `--account` and `--name` flags to `fleet launch`

**Files:**
- Modify: `cmd/fleet/main.go`

- [ ] **Step 1: Add flags and wire them through**

In `cmd/fleet/main.go`, update `launchCmd` to read like this (adding the two new `var` lines, two new `StringVar` calls, passing `Account` into `session.LaunchOpts`, and creating the label after a successful launch):

```go
func launchCmd() *cobra.Command {
	var branch string
	var target string
	var account string
	var label string

	cmd := &cobra.Command{
		Use:   "launch <org/repo>",
		Short: "Launch Claude Code on the best available machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			ctx := context.Background()

			cfg, err := config.Load(config.DefaultPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			enabled := cfg.EnabledMachines()
			if len(enabled) == 0 {
				return fmt.Errorf("no enabled machines in config")
			}

			var chosen config.Machine

			if target != "" {
				found := false
				for _, m := range enabled {
					if m.Name == target {
						chosen = m
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("machine %q not found or not enabled", target)
				}
				fmt.Printf("Using specified target: %s\n", chosen.Name)
			} else {
				fmt.Println("Probing machines...")
				healths := machine.ProbeAll(ctx, enabled)

				for _, h := range healths {
					if h.Online {
						availPct := float64(h.AvailMemory) / float64(h.TotalMemory) * 100
						fmt.Printf("  %s: %.0f%% mem avail, %.0fMB swap, %d claude instances (score: %.1f)\n",
							h.Name, availPct, h.SwapUsedMB, h.ClaudeCount, machine.Score(h))
					} else {
						fmt.Printf("  %s: offline\n", h.Name)
					}
				}

				best, score := machine.PickBest(healths)
				if !best.Online {
					return fmt.Errorf("no machines are reachable")
				}

				if score < float64(cfg.Settings.StressThreshold) {
					fmt.Printf("\nAll machines are stressed. Best: %s (score: %.1f)\n", best.Name, score)
					fmt.Print("Launch anyway? [y/n]: ")
					var answer string
					_, _ = fmt.Scanln(&answer)
					if answer != "y" && answer != "Y" {
						fmt.Println("Aborted.")
						return nil
					}
				}

				chosen = findMachine(enabled, best.Name)
				fmt.Printf("\nSelected: %s (score: %.1f)\n", chosen.Name, score)
			}

			fmt.Printf("Setting up %s on %s...\n", project, chosen.Name)

			result, err := session.Launch(ctx, session.LaunchOpts{
				Project:   project,
				Branch:    branch,
				Account:   account,
				Machine:   chosen,
				Settings:  cfg.Settings,
				StatePath: session.DefaultStatePath(),
			})
			if err != nil {
				return fmt.Errorf("launch: %w", err)
			}

			if label != "" {
				if err := session.AddLabel(
					session.DefaultStatePath(),
					chosen.Name,
					label,
					result.Session.ID,
					result.Session.PID,
				); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to add label: %v\n", err)
				}
			}

			if result.Session.Tunnel.LocalPort > 0 && !chosen.IsLocal() {
				fmt.Printf("Tunnel: localhost:%d → %s:%d\n",
					result.Session.Tunnel.LocalPort, chosen.Name, result.Session.Tunnel.RemotePort)
			}

			fmt.Printf("Session %s started. Launching Claude Code...\n\n", result.Session.ID)

			return session.WithSignalCleanup(
				ctx, chosen, result.Session, result.Tunnel,
				session.DefaultStatePath(),
				func() error {
					return session.ExecClaude(chosen, result.Session.WorktreePath)
				},
			)
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "main", "Branch to check out")
	cmd.Flags().StringVarP(&target, "target", "t", "", "Force a specific machine")
	cmd.Flags().StringVar(&account, "account", "", "Claude account label for this session (falls back to machine default)")
	cmd.Flags().StringVar(&label, "name", "", "Nickname to attach to the machine (creates a linked label)")
	return cmd
}
```

- [ ] **Step 2: Build to verify no syntax errors**

Run: `cd /Users/jeremywatt/Desktop/fleet && go build -o /tmp/fleet ./cmd/fleet && /tmp/fleet launch --help`

Expected: `launch` help lists `--account` and `--name` flags.

- [ ] **Step 3: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add cmd/fleet/main.go
git commit -m "feat(cli): add --account and --name flags to launch"
```

---

## Task 8: TUI Sessions panel — add `ACCOUNT` and `LABEL` columns

**Files:**
- Modify: `internal/tui/sessions.go`
- Create: `internal/tui/sessions_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/sessions_test.go`:

```go
package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/neonwatty/fleet/internal/session"
)

func TestSessionsPanelRendersAccountAndLabel(t *testing.T) {
	sessions := []session.Session{
		{
			ID:        "a1b2c3d4",
			Project:   "neonwatty/bleep",
			Machine:   "mm1",
			Branch:    "main",
			Account:   "personal-max",
			StartedAt: time.Now().Add(-10 * time.Minute),
		},
	}
	labels := map[string][]session.MachineLabel{
		"mm1": {{Name: "bleep", SessionID: "a1b2c3d4"}},
	}

	out := renderSessionsPanel(sessions, labels)
	if !strings.Contains(out, "personal-max") {
		t.Errorf("output missing account:\n%s", out)
	}
	if !strings.Contains(out, "bleep") {
		t.Errorf("output missing label:\n%s", out)
	}
	if !strings.Contains(out, "ACCOUNT") || !strings.Contains(out, "LABEL") {
		t.Errorf("output missing headers:\n%s", out)
	}
}

func TestSessionsPanelEmptyWithoutAccountOrLabel(t *testing.T) {
	sessions := []session.Session{
		{ID: "x", Project: "p", Machine: "mm2", Branch: "main", StartedAt: time.Now()},
	}
	out := renderSessionsPanel(sessions, nil)
	if !strings.Contains(out, "—") {
		t.Errorf("expected em-dash placeholder for empty account/label:\n%s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./internal/tui/ -run TestSessionsPanel -v`

Expected: FAIL — `renderSessionsPanel` signature doesn't accept labels, or `ACCOUNT`/`LABEL` headers missing.

- [ ] **Step 3: Rewrite `renderSessionsPanel`**

Replace the body of `internal/tui/sessions.go` with:

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/neonwatty/fleet/internal/session"
)

func renderSessionsPanel(sessions []session.Session, labels map[string][]session.MachineLabel) string {
	if len(sessions) == 0 {
		return dimStyle.Render("No active sessions")
	}

	var b strings.Builder

	header := fmt.Sprintf("%-8s %-20s %-8s %-10s %-10s %-14s %-14s",
		"ID", "PROJECT", "MACHINE", "BRANCH", "UPTIME", "ACCOUNT", "LABEL")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for _, s := range sessions {
		uptime := time.Since(s.StartedAt).Truncate(time.Second)
		account := s.Account
		if account == "" {
			account = "—"
		}
		label := labelForSession(labels, s)
		line := fmt.Sprintf("%-8s %-20s %-8s %-10s %-10s %-14s %-14s",
			s.ID, truncateStr(s.Project, 20), s.Machine, s.Branch, uptime,
			truncateStr(account, 14), truncateStr(label, 14))
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func labelForSession(labels map[string][]session.MachineLabel, s session.Session) string {
	if labels == nil {
		return "—"
	}
	for _, l := range labels[s.Machine] {
		if l.SessionID == s.ID {
			return l.Name
		}
	}
	return "—"
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
```

- [ ] **Step 4: Update the caller in `app.go`**

In `internal/tui/app.go`, change the `renderSessionsPanel` call inside the `View` method (around line 162) to:

```go
	var labels map[string][]session.MachineLabel
	if m.state != nil {
		labels = m.state.MachineLabels
	}
	sessionsContent := renderSessionsPanel(sessions, labels)
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./internal/tui/ -v && go build ./...`

Expected: all tests PASS and build succeeds.

- [ ] **Step 6: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add internal/tui/sessions.go internal/tui/sessions_test.go internal/tui/app.go
git commit -m "feat(tui): render ACCOUNT and LABEL columns in sessions panel"
```

---

## Task 9: TUI Machines panel — render `[account]` and labels list (live vs stale)

**Files:**
- Modify: `internal/tui/machines.go`
- Modify: `internal/tui/app.go`
- Create: `internal/tui/machines_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/machines_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
)

func TestMachinesPanelRendersAccountSuffixAndLabels(t *testing.T) {
	healths := []machine.Health{
		{
			Name:        "mm1",
			Online:      true,
			TotalMemory: 16 * 1024 * 1024 * 1024,
			AvailMemory: 8 * 1024 * 1024 * 1024,
			SwapUsedMB:  0,
			SwapTotalMB: 4096,
			ClaudeCount: 2,
		},
	}
	sessions := []session.Session{
		{ID: "s1", Machine: "mm1", Account: "personal-max"},
	}
	labels := map[string][]session.MachineLabel{
		"mm1": {
			{Name: "bleep", SessionID: "s1"},            // live
			{Name: "deckchecker", SessionID: ""},        // stale
		},
	}
	ccPIDs := map[string][]int{"mm1": {}}

	out := renderMachinesPanel(healths, sessions, labels, ccPIDs, 80)
	if !strings.Contains(out, "[personal-max]") {
		t.Errorf("expected [personal-max] suffix:\n%s", out)
	}
	if !strings.Contains(out, "bleep") {
		t.Errorf("expected live label 'bleep':\n%s", out)
	}
	if !strings.Contains(out, "deckchecker") {
		t.Errorf("expected stale label 'deckchecker':\n%s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./internal/tui/ -run TestMachinesPanel -v`

Expected: FAIL — `renderMachinesPanel` signature mismatch.

- [ ] **Step 3: Rewrite `renderMachinesPanel`**

Replace `internal/tui/machines.go` with:

```go
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
			b.WriteString(formatLabelList(labels[h.Name], nil))
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
		b.WriteString(formatLabelList(labels[h.Name], ccPIDs[h.Name]))
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
// A label is live when its SessionID is non-empty OR its LastSeenPID matches
// one of the currently observed CC PIDs on the machine.
func formatLabelList(labels []session.MachineLabel, livePIDs []int) string {
	if len(labels) == 0 {
		return dimStyle.Render("—")
	}
	livePIDset := make(map[int]struct{}, len(livePIDs))
	for _, p := range livePIDs {
		livePIDset[p] = struct{}{}
	}

	parts := make([]string, 0, len(labels))
	for _, l := range labels {
		live := false
		if l.SessionID != "" {
			live = true
		} else if l.LastSeenPID != 0 {
			if _, ok := livePIDset[l.LastSeenPID]; ok {
				live = true
			}
		}
		if live {
			parts = append(parts, l.Name)
		} else {
			parts = append(parts, dimStyle.Render(l.Name+"(stale)"))
		}
	}
	return strings.Join(parts, ", ")
}
```

- [ ] **Step 4: Update the caller in `app.go`**

In `internal/tui/app.go`, update the `View` method where `renderMachinesPanel` is called (around line 155) to:

```go
	var labels map[string][]session.MachineLabel
	var sessionsForPanel []session.Session
	if m.state != nil {
		labels = m.state.MachineLabels
		sessionsForPanel = m.state.Sessions
	}
	ccPIDs := ccPIDsFromProcesses(m.processes)
	machinesContent := renderMachinesPanel(m.healths, sessionsForPanel, labels, ccPIDs, panelWidth)
	machinesPanel := wrapPanel("Machines", machinesContent, panelWidth, m.activePanel == panelMachines)
```

And add a helper at the bottom of `app.go`:

```go
func ccPIDsFromProcesses(procs map[string][]machine.ProcessGroup) map[string][]int {
	out := make(map[string][]int, len(procs))
	for name, groups := range procs {
		for _, g := range groups {
			if g.Name == "Claude Code" {
				out[name] = append(out[name], g.PIDs...)
			}
		}
	}
	return out
}
```

- [ ] **Step 5: Run tests and build**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./internal/tui/ -v && go build ./...`

Expected: all tests PASS and build succeeds.

- [ ] **Step 6: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add internal/tui/machines.go internal/tui/machines_test.go internal/tui/app.go
git commit -m "feat(tui): render [account] suffix and labels list on machines panel"
```

---

## Task 10: TUI `n` keybinding to rename a session's label

**Files:**
- Modify: `internal/tui/app.go`

- [ ] **Step 1: Add rename-mode state and helpers to the model**

At the top of `internal/tui/app.go` (after the existing field declarations in `type model struct`), add two new fields:

```go
type model struct {
	cfg             *config.Config
	statePath       string
	healths         []machine.Health
	state           *session.State
	processes       map[string][]machine.ProcessGroup
	activePanel     panel
	selectedRow     int
	selectedMachine int
	width           int
	height          int
	pollInterval    time.Duration
	swapScanning    bool
	swapScanTarget  string
	renaming        bool
	renameBuffer    string
}
```

- [ ] **Step 2: Add rename handling to `Update`**

Inside the `Update` method, replace the `tea.KeyMsg` branch so it first checks the renaming state and then falls through to the existing keymap. Insert at the top of `case tea.KeyMsg:`:

```go
		if m.renaming {
			switch msg.String() {
			case "esc":
				m.renaming = false
				m.renameBuffer = ""
			case "enter":
				if m.state != nil && m.selectedRow < len(m.state.Sessions) {
					sess := m.state.Sessions[m.selectedRow]
					if strings.TrimSpace(m.renameBuffer) != "" {
						_ = session.AddLabel(
							m.statePath,
							sess.Machine,
							strings.TrimSpace(m.renameBuffer),
							sess.ID,
							sess.PID,
						)
					}
				}
				m.renaming = false
				m.renameBuffer = ""
				return m, refresh(m.cfg, m.statePath)
			case "backspace":
				if len(m.renameBuffer) > 0 {
					m.renameBuffer = m.renameBuffer[:len(m.renameBuffer)-1]
				}
			default:
				if len(msg.Runes) == 1 {
					m.renameBuffer += string(msg.Runes)
				}
			}
			return m, nil
		}
```

In the same switch, add a new case for `"n"` alongside the other panel-specific keybindings:

```go
		case "n":
			if m.activePanel == panelSessions && m.state != nil && m.selectedRow < len(m.state.Sessions) {
				m.renaming = true
				m.renameBuffer = ""
			}
```

Add `"strings"` to the import list if not already present.

- [ ] **Step 3: Surface the rename prompt in the help line**

Near the bottom of the `View` method, replace the `helpParts` assignment with:

```go
	helpParts := "tab: switch panel | j/k: navigate | o: open in browser | x: kill session | n: rename label | s: scan swap | d: kill process group | q: quit"
	if m.renaming {
		helpParts = fmt.Sprintf("rename label: %s▌  (enter: save, esc: cancel)", m.renameBuffer)
	} else if m.swapScanning {
		helpParts = fmt.Sprintf("Scanning swap on %s... | q: quit", m.swapScanTarget)
	}
```

- [ ] **Step 4: Build to verify no syntax errors**

Run: `cd /Users/jeremywatt/Desktop/fleet && go build ./... && go test ./internal/tui/ -v`

Expected: build succeeds, existing tests still pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add internal/tui/app.go
git commit -m "feat(tui): add n keybinding to rename session labels"
```

---

## Task 11: `fleet status --json` output mode

**Files:**
- Create: `cmd/fleet/status_json.go`
- Create: `cmd/fleet/status_json_test.go`
- Modify: `cmd/fleet/main.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/fleet/status_json_test.go`:

```go
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
)

func TestBuildStatusJSON(t *testing.T) {
	healths := []machine.Health{
		{
			Name:        "mm1",
			Online:      true,
			TotalMemory: 16 * 1024 * 1024 * 1024,
			AvailMemory: 4 * 1024 * 1024 * 1024,
			SwapUsedMB:  2048,
			SwapTotalMB: 4096,
			ClaudeCount: 2,
		},
		{Name: "mm2", Online: false},
	}
	sessions := []session.Session{
		{
			ID:        "a1b2c3",
			Project:   "neonwatty/bleep",
			Machine:   "mm1",
			Branch:    "main",
			Account:   "personal-max",
			Tunnel:    session.TunnelInfo{LocalPort: 3000, RemotePort: 3000},
			StartedAt: time.Date(2026, 4, 12, 9, 15, 0, 0, time.UTC),
			PID:       4242,
		},
	}
	labels := map[string][]session.MachineLabel{
		"mm1": {
			{Name: "bleep", SessionID: "a1b2c3", LastSeenPID: 4242},
			{Name: "deckchecker", SessionID: ""},
		},
	}
	ccPIDs := map[string][]int{"mm1": {4242}}
	cfg := &config.Config{Machines: []config.Machine{{Name: "mm1"}, {Name: "mm2"}}}

	doc := buildStatusJSON(cfg, healths, sessions, labels, ccPIDs, time.Date(2026, 4, 12, 14, 32, 10, 0, time.UTC))
	blob, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(blob)

	for _, want := range []string{
		`"timestamp":"2026-04-12T14:32:10Z"`,
		`"name":"mm1"`,
		`"status":"online"`,
		`"accounts":["personal-max"]`,
		`"name":"bleep"`,
		`"live":true`,
		`"name":"deckchecker"`,
		`"live":false`,
		`"name":"mm2"`,
		`"status":"offline"`,
		`"project":"neonwatty/bleep"`,
		`"account":"personal-max"`,
		`"label":"bleep"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("json missing %q:\n%s", want, s)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./cmd/fleet/ -run TestBuildStatusJSON -v`

Expected: FAIL — `buildStatusJSON` undefined.

- [ ] **Step 3: Implement `cmd/fleet/status_json.go`**

Create `cmd/fleet/status_json.go`:

```go
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
	Timestamp string          `json:"timestamp"`
	Machines  []machineStatus `json:"machines"`
	Sessions  []sessionStatus `json:"sessions"`
}

type machineStatus struct {
	Name         string        `json:"name"`
	Status       string        `json:"status"` // "online" | "offline"
	MemAvailPct  int           `json:"mem_available_pct"`
	SwapGB       float64       `json:"swap_gb"`
	CCCount      int           `json:"cc_count"`
	Score        float64       `json:"score"`
	Label        string        `json:"label"`
	Accounts     []string      `json:"accounts"`
	Labels       []labelStatus `json:"labels"`
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
	cfg *config.Config,
	healths []machine.Health,
	sessions []session.Session,
	labels map[string][]session.MachineLabel,
	ccPIDs map[string][]int,
	now time.Time,
) statusDoc {
	doc := statusDoc{Timestamp: now.UTC().Format(time.RFC3339)}

	for _, h := range healths {
		ms := machineStatus{
			Name:     h.Name,
			Accounts: accountsForMachine(h.Name, sessions),
			Labels:   labelStatusList(labels[h.Name], ccPIDs[h.Name]),
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
		ms.Label = machine.ScoreLabel(ms.Score)
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
	var out []string
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

func labelStatusList(labels []session.MachineLabel, livePIDs []int) []labelStatus {
	livePIDset := make(map[int]struct{}, len(livePIDs))
	for _, p := range livePIDs {
		livePIDset[p] = struct{}{}
	}
	out := make([]labelStatus, 0, len(labels))
	for _, l := range labels {
		live := false
		if l.SessionID != "" {
			live = true
		} else if l.LastSeenPID != 0 {
			if _, ok := livePIDset[l.LastSeenPID]; ok {
				live = true
			}
		}
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

	doc := buildStatusJSON(cfg, healths, state.Sessions, state.MachineLabels, ccPIDs, time.Now())

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
```

- [ ] **Step 4: Add `--json` flag to `statusCmd`**

In `cmd/fleet/main.go`, change `statusCmd` to:

```go
func statusCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show fleet dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.DefaultPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if jsonOut {
				return runStatusJSON(cfg)
			}
			return tui.Run(cfg, session.DefaultStatePath())
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit fleet status as JSON and exit (no TUI)")
	return cmd
}
```

- [ ] **Step 5: Run tests and build**

Run: `cd /Users/jeremywatt/Desktop/fleet && go test ./cmd/fleet/ -v && go build -o /tmp/fleet ./cmd/fleet && /tmp/fleet status --help`

Expected: tests PASS, `--json` flag listed in help.

- [ ] **Step 6: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add cmd/fleet/status_json.go cmd/fleet/status_json_test.go cmd/fleet/main.go
git commit -m "feat(cli): add fleet status --json output mode"
```

---

## Task 12: SwiftBar plugin script and fixture-based test

**Files:**
- Create: `scripts/swiftbar/fleet.10s.sh`
- Create: `scripts/swiftbar/fixtures/status.json`
- Create: `scripts/swiftbar/fixtures/status.expected.txt`
- Create: `scripts/swiftbar/README.md`
- Modify: `Makefile`

- [ ] **Step 1: Create the fixture JSON**

Create `scripts/swiftbar/fixtures/status.json`:

```json
{
  "timestamp": "2026-04-12T14:32:10Z",
  "machines": [
    {
      "name": "mm1",
      "status": "online",
      "mem_available_pct": 22,
      "swap_gb": 9.1,
      "cc_count": 2,
      "score": 5,
      "label": "busy",
      "accounts": ["personal-max"],
      "labels": [
        {"name": "bleep", "live": true, "session_id": "a1b2c3"},
        {"name": "deckchecker", "live": false, "session_id": ""}
      ]
    },
    {
      "name": "mm2",
      "status": "online",
      "mem_available_pct": 45,
      "swap_gb": 2.4,
      "cc_count": 1,
      "score": 32,
      "label": "free",
      "accounts": ["work-team"],
      "labels": [
        {"name": "seatify", "live": true, "session_id": "d4e5f6"}
      ]
    },
    {
      "name": "mm3",
      "status": "offline",
      "mem_available_pct": 0,
      "swap_gb": 0,
      "cc_count": 0,
      "score": -1000,
      "label": "offline",
      "accounts": [],
      "labels": []
    }
  ],
  "sessions": [
    {"id": "a1b2c3", "project": "neonwatty/bleep", "machine": "mm1", "branch": "main", "account": "personal-max", "label": "bleep", "tunnel_local_port": 3000, "tunnel_remote_port": 3000, "started_at": "2026-04-12T09:15:00Z"},
    {"id": "d4e5f6", "project": "neonwatty/seatify", "machine": "mm2", "branch": "main", "account": "work-team", "label": "seatify", "tunnel_local_port": 4001, "tunnel_remote_port": 3000, "started_at": "2026-04-12T10:00:00Z"}
  ]
}
```

- [ ] **Step 2: Create the plugin script**

Create `scripts/swiftbar/fleet.10s.sh` (mark executable in Step 3):

```bash
#!/bin/bash
# <bitbar.title>Fleet</bitbar.title>
# <bitbar.version>v1.0</bitbar.version>
# <bitbar.author>neonwatty</bitbar.author>
# <bitbar.desc>Fleet status in the macOS menu bar (reads fleet status --json).</bitbar.desc>
# <bitbar.dependencies>jq, fleet</bitbar.dependencies>
#
# SwiftBar plugin for fleet. Install: copy this file to your SwiftBar plugin
# directory and enable it. Requires `fleet` on $PATH and `jq` installed.
#
# The script accepts FLEET_BIN and FLEET_STATUS_FIXTURE env vars for testing.

set -eu

FLEET_BIN="${FLEET_BIN:-fleet}"

if ! command -v jq >/dev/null 2>&1; then
  echo "fleet: install jq"
  echo "---"
  echo "brew install jq | bash=/opt/homebrew/bin/brew param1=install param2=jq terminal=true"
  exit 0
fi

if [ -n "${FLEET_STATUS_FIXTURE:-}" ]; then
  JSON=$(cat "$FLEET_STATUS_FIXTURE")
elif ! JSON=$("$FLEET_BIN" status --json 2>/dev/null); then
  echo "fleet ⚠ | color=red"
  echo "---"
  echo "fleet status --json failed"
  echo "Check fleet binary on PATH | bash=which param1=fleet terminal=true"
  exit 0
fi

total=$(echo "$JSON" | jq '.machines | length')
online=$(echo "$JSON" | jq '[.machines[] | select(.status == "online")] | length')
cc=$(echo "$JSON" | jq '[.machines[].cc_count] | add // 0')
stressed=$(echo "$JSON" | jq '[.machines[] | select(.label == "stressed")] | length')
busy=$(echo "$JSON" | jq '[.machines[] | select(.label == "busy")] | length')

prefix=""
color=""
if [ "$stressed" -gt 0 ]; then
  prefix="⚠ "
  color=" | color=red"
elif [ "$busy" -gt 0 ]; then
  prefix="⚠ "
  color=" | color=orange"
fi

echo "${prefix}${online}/${total} · ${cc} CC${color}"
echo "---"
echo "FLEET | size=11 color=gray"
echo "---"

echo "$JSON" | jq -r '
  .machines[] |
  (if .status == "offline" then
    "\(.name) — offline | color=gray"
  else
    "\(.name) \(if (.accounts | length) > 0 then "[" + (.accounts | join(",")) + "]" else "" end)  \(.label) · \(.mem_available_pct)% mem · \(.swap_gb)GB swap · \(.cc_count) CC"
  end),
  (.labels[] |
    (if .live then "  ● " + .name + " | color=white"
    else "  ○ " + .name + "(stale) | color=gray"
    end))
'

echo "---"
echo "Open full dashboard | bash=$FLEET_BIN param1=status terminal=true"
echo "Refresh | refresh=true"
```

- [ ] **Step 3: Mark the script executable and add golden output**

Run:

```bash
chmod +x /Users/jeremywatt/Desktop/fleet/scripts/swiftbar/fleet.10s.sh
```

Then generate the golden output by running the script against the fixture once:

```bash
cd /Users/jeremywatt/Desktop/fleet && FLEET_STATUS_FIXTURE=scripts/swiftbar/fixtures/status.json ./scripts/swiftbar/fleet.10s.sh > scripts/swiftbar/fixtures/status.expected.txt
cat scripts/swiftbar/fixtures/status.expected.txt
```

Inspect the output. It should begin with `⚠ 2/3 · 3 CC | color=orange` (because one machine is `busy`) and include per-machine lines for `mm1`, `mm2`, and `mm3`, plus label bullet lines. If the output looks wrong, fix the script before continuing.

- [ ] **Step 4: Add a Makefile target and self-check**

Append to `Makefile`:

```makefile
.PHONY: test-swiftbar
test-swiftbar:
	@diff -u scripts/swiftbar/fixtures/status.expected.txt \
		<(FLEET_STATUS_FIXTURE=scripts/swiftbar/fixtures/status.json \
			./scripts/swiftbar/fleet.10s.sh)
	@echo "swiftbar plugin output matches golden."
```

Then fold it into `make check` — find the existing `check:` target and append `test-swiftbar` to its dependency list.

Run: `cd /Users/jeremywatt/Desktop/fleet && make test-swiftbar`

Expected: `swiftbar plugin output matches golden.` (no diff).

- [ ] **Step 5: Create the plugin README**

Create `scripts/swiftbar/README.md`:

```markdown
# Fleet SwiftBar plugin

Compact menu bar indicator for fleet. Shows online/total machines, live CC
count, and per-machine details in the dropdown.

## Install

1. Install [SwiftBar](https://github.com/swiftbar/SwiftBar).
2. Install `jq`: `brew install jq`.
3. Make sure `fleet` is on your `PATH`.
4. Copy the plugin to your SwiftBar plugin directory:

   ```bash
   mkdir -p ~/Library/Application\ Support/SwiftBar/Plugins
   cp fleet.10s.sh ~/Library/Application\ Support/SwiftBar/Plugins/
   ```

5. Open SwiftBar. The `fleet` plugin should appear in the menu bar within 10s.

The `.10s.` in the filename controls the refresh cadence. Rename to
`fleet.30s.sh` for a slower refresh or `fleet.5s.sh` for a faster one.

## Customizing

Set `FLEET_BIN` if `fleet` is not on `PATH`:

```bash
export FLEET_BIN=/opt/homebrew/bin/fleet
```

## Testing

Run the fixture test from the repo root:

```bash
make test-swiftbar
```

This diffs the script's output against a committed golden file so regressions
are caught in CI without needing SwiftBar installed.
```

- [ ] **Step 6: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add scripts/swiftbar/ Makefile
git commit -m "feat(swiftbar): ship menu bar plugin and fixture test"
```

---

## Task 13: Update top-level docs and example config

**Files:**
- Modify: `README.md`
- Modify: `config.example.toml`

- [ ] **Step 1: Add menu bar section to `README.md`**

In `README.md`, after the `## Commands` section and before `## Health Score`, insert:

```markdown
## Menu Bar (SwiftBar)

Fleet ships a SwiftBar plugin that shows a compact fleet indicator in the
macOS menu bar. See [`scripts/swiftbar/README.md`](scripts/swiftbar/README.md)
for install instructions.

At a glance: `3/4 · 2 CC` means 3 of 4 machines are online with 2 live Claude
Code instances. Click the indicator for a per-machine dropdown with accounts,
labels, memory, and swap.

## Session Labels and Accounts

Fleet lets you attach user-chosen nicknames ("labels") to Claude Code sessions
on each machine, and record which Claude subscription each session is burning.

```bash
# At launch
fleet launch neonwatty/bleep -t mm1 --account personal-max --name bleep

# After launch
fleet label set mm1 bleep --session a1b2c3
fleet account a1b2c3 personal-max
```

Labels survive remote machine restarts (they live in `~/.fleet/state.json` on
the hub, not on the remote machine). When a label's matching CC process is
gone, the TUI and menu bar render it dimmed as "stale" — useful for remembering
what was running before a reboot.

Per-machine default accounts can be set in `config.toml` so you don't have to
pass `--account` every time:

```toml
[[machines]]
name = "mm1"
host = "mm1"
user = "neonwatty"
enabled = true
default_account = "personal-max"
```
```

- [ ] **Step 2: Update `config.example.toml`**

In `config.example.toml`, find one of the `[[machines]]` entries and add a commented example line:

```toml
[[machines]]
name = "mm1"
host = "mm1"
user = "neonwatty"
enabled = true
# default_account = "personal-max"  # optional; stamps new sessions launched on mm1
```

- [ ] **Step 3: Run the full check**

Run: `cd /Users/jeremywatt/Desktop/fleet && make check`

Expected: all checks pass (fmt, lint, vet, test, build, test-swiftbar).

- [ ] **Step 4: Commit**

```bash
cd /Users/jeremywatt/Desktop/fleet
git add README.md config.example.toml
git commit -m "docs: document labels, accounts, and SwiftBar menu bar"
```

---

## Self-Review Checklist

Before declaring the plan complete, the implementer should verify:

- [ ] `~/.fleet/state.json` with no `machine_labels` field still loads cleanly (Task 1 covers this).
- [ ] `fleet launch ... --account foo --name bar -t mm1` produces a session with `Account: "foo"` and a linked `MachineLabel{Name: "bar", SessionID: <new-id>}` on `mm1`.
- [ ] `fleet status --json` includes both live and stale labels with accurate `live` flags.
- [ ] TUI Sessions panel shows `ACCOUNT` and `LABEL` columns; pressing `n` on a row enters rename mode and commits on Enter.
- [ ] TUI Machines panel shows the `[account]` suffix on the machine name and a comma-separated labels list with stale labels dimmed.
- [ ] SwiftBar plugin script output diffs cleanly against the golden fixture.
- [ ] `README.md` documents all new commands and the menu bar install path.
- [ ] `make check` passes end-to-end.
