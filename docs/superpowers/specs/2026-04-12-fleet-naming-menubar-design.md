# Fleet — Session Labels, Per-Session Accounts, and SwiftBar Menu Bar

## Problem

Fleet currently makes it easy to launch and monitor Claude Code instances across a local Mac fleet, but two gaps hurt day-to-day usability:

1. **No way to name CC instances in fleet itself.** When multiple projects are spread across machines, it's hard to remember at a glance which project is running where — especially after a remote machine restart, when the process is gone but the memory of what was running is still valuable.
2. **No persistent at-a-glance view.** The TUI dashboard is great, but it only exists while `fleet status` is running in a terminal. There's no always-visible indicator of fleet state, which means either keeping a dedicated terminal tab open or running `fleet status` repeatedly.

A third gap surfaced during brainstorming:

3. **No visibility into which Claude subscription a session is burning.** When juggling multiple CC subscriptions ("personal-max", "work-team", etc.) across machines, there's no way to see which account a given live session is consuming quota from. This creates real risk of unintentionally burning one account's rate limit.

## Solution

Three coordinated additions to fleet, designed to share a single data pipeline:

1. **Machine-scoped session labels** — user-assigned nicknames that live on the hub (`~/.fleet/state.json`), survive remote machine restarts, and render in both the TUI and the menu bar. Labels are explicit-only (no auto-derivation), multiple per machine, and kept-but-dimmed when their matching CC process is gone ("stale").
2. **Per-session account tracking** — a free-text account name captured on each session at launch time, optionally defaulted from `config.toml` per machine. Displayed in the Sessions panel, aggregated into the Machines panel, and surfaced in the menu bar.
3. **SwiftBar menu bar plugin** — a native-looking menu bar indicator driven by a new `fleet status --json` output mode. Zero additional Go dependencies; the mac-specific rendering is owned entirely by [SwiftBar](https://github.com/swiftbar/SwiftBar).

A visual mockup of the menu bar lives at [`docs/superpowers/mockups/2026-04-12-menu-bar.html`](../mockups/2026-04-12-menu-bar.html).

## Architecture

```
~/.fleet/state.json   (hub-only; survives remote restarts)
├── sessions[]             (existing; gains Account field)
└── machine_labels{}       (new; map[machineName][]MachineLabel)

cmd/fleet/
├── label.go               (new subcommand)
├── account.go             (new subcommand)
└── status_json.go         (new --json mode on status)

internal/session/
├── labels.go + _test.go   (new; pure state mutation)
└── account.go + _test.go  (new; account resolution + mutation)

scripts/swiftbar/
├── fleet.10s.sh           (SwiftBar plugin; shell + jq)
├── fixtures/
│   ├── status.json        (test fixture)
│   └── status.expected.txt (golden output)
└── README.md              (install instructions)
```

The hub (MacBook Air) is the single source of truth. The Mac Minis remain dumb SSH targets. Because labels and accounts live in `state.json` on the hub, they are unaffected by remote machine reboots — which is the precise "remember what was running before the restart" property originally requested.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Label scope | Machine-scoped, multiple per machine | Matches user mental model ("mm1 is running bleep and deckchecker"); survives restart; aggregates naturally in the Machines panel |
| Label lifetime | Kept-but-dimmed when stale | User explicitly wants to remember past state; deleting on process exit would defeat the feature |
| Label assignment | Explicit only | User rejected auto-derivation from repo name; wants deliberate naming |
| Account scope | Session-level, not machine-level | Usage is consumed per session; a session launched under one account keeps burning that quota even if the machine's default changes later |
| Account default | Optional `default_account` in `config.toml` | Set-once, forget ergonomics; still captured to session at launch time |
| Menu bar host | SwiftBar plugin | Native NSMenu rendering for free; fleet stays pure Go; ~half day of work vs ~weeks for a standalone Swift app |
| Menu bar data source | `fleet status --json` subprocess | Reuses existing probe code path; no daemon, no IPC, no shared state |
| Refresh cadence | 10 seconds (SwiftBar filename `fleet.10s.sh`) | Matches fleet's existing 5s TUI poll tolerance; user can rename the file to change cadence |
| Back-compat | Additive JSON fields only | Existing `state.json` files parse with nil `machine_labels`; unset `Account` is empty string |

## Data Model

### State additions (`internal/session/state.go`)

```go
type State struct {
    Sessions      []Session                  `json:"sessions"`
    MachineLabels map[string][]MachineLabel  `json:"machine_labels"` // key = machine name
}

type MachineLabel struct {
    Name        string    `json:"name"`           // "bleep", "deckchecker-auth"
    SessionID   string    `json:"session_id"`     // "" if orphan (no linked session)
    CreatedAt   time.Time `json:"created_at"`
    LastSeenPID int       `json:"last_seen_pid"`  // 0 if never observed
}

type Session struct {
    ID           string     `json:"id"`
    Project      string     `json:"project"`
    Machine      string     `json:"machine"`
    Branch       string     `json:"branch"`
    Account      string     `json:"account"`       // NEW — free-text account label
    WorktreePath string     `json:"worktree_path"`
    Tunnel       TunnelInfo `json:"tunnel"`
    StartedAt    time.Time  `json:"started_at"`
    PID          int        `json:"pid"`
}
```

### Config additions (`internal/config/config.go`)

```go
type Machine struct {
    Name           string `toml:"name"`
    Host           string `toml:"host"`
    User           string `toml:"user"`
    Enabled        bool   `toml:"enabled"`
    DefaultAccount string `toml:"default_account"` // NEW — optional
}
```

### Liveness computation (not stored)

A label is **live** if either of these is true:

1. `SessionID` is non-empty AND a matching `Session` exists in state with that ID AND the session's PID is running on the machine (from the probe).
2. `SessionID` is empty AND `LastSeenPID` matches a CC process currently visible on the machine via the existing `isClaudeCode` scan in `internal/machine/processes.go`.

Otherwise the label is **stale** and rendered dimmed. Liveness is recomputed on each probe cycle and never persisted, so there is no possibility of the persisted state and the displayed state diverging.

## CLI Surface

### New subcommands

```bash
# Labels
fleet label mm1 "bleep-auth"                  # add label on mm1 (orphan, no session link)
fleet label mm1 "bleep-auth" --session a1b2c3 # add label linked to a specific session
fleet label mm1 --remove "bleep-auth"         # remove one label
fleet label mm1 --clear                       # remove all labels on mm1
fleet label list                              # list all labels across the fleet
fleet label list mm1                          # list labels for one machine

# Accounts
fleet account <session-id> "personal-max"     # set/change a session's account
fleet account <session-id> --clear            # unset
```

Session IDs are matched exactly (no prefix matching in v1) to keep the parse path trivial; prefix matching can be added later without a schema change.

### New flags on existing commands

```bash
# fleet launch
--account <name>  # set session.Account at launch; falls back to config default
--name <label>    # create a MachineLabel linked to the new session in one atomic write
```

```bash
# fleet status
--json  # emit status as JSON and exit (no TUI, no polling loop)
```

### TUI keybinding additions (`internal/tui/app.go`)

| Panel | Key | Action |
|-------|-----|--------|
| Sessions | `n` | Enter label-edit mode for the selected session; submit calls `fleet label` code path |

The rename keybinding lives on the Sessions panel (not Machines) because labels belong to specific CC instances. The Machines panel aggregates labels for display but does not own mutation.

### Error handling

| Case | Behavior |
|------|----------|
| `fleet label <unknown_machine>` | Error with list of known machine names from config |
| Duplicate label name on same machine | Last-write-wins, silent overwrite (frictionless rename) |
| `fleet account <unknown_session_id>` | Error with list of live session IDs |
| `--session` flag references unknown session | Error; label is not created |
| Launch with `--account` when no config default is set | Allowed; `--account` is always the final word |
| `fleet status --json` when some machines are offline | Include offline machines with `"status": "offline"` and zeroed metrics; never omit |

## `fleet status --json` Contract

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
        {"name": "bleep",       "live": true,  "session_id": "a1b2c3"},
        {"name": "deckchecker", "live": false, "session_id": ""}
      ]
    }
  ],
  "sessions": [
    {
      "id": "a1b2c3",
      "project": "neonwatty/bleep",
      "machine": "mm1",
      "branch": "main",
      "account": "personal-max",
      "label": "bleep",
      "tunnel_local_port": 3000,
      "tunnel_remote_port": 3000,
      "started_at": "2026-04-12T09:15:00Z"
    }
  ]
}
```

The emitter is a pure function over the existing probe result struct — no new probing logic, no parallel code paths. `cmd/fleet/status_json.go` wraps the probe call, serializes, and prints; everything else is shared with the TUI.

## Rendering

### Machines panel (existing TUI, extended)

```
MACHINE             STATUS    MEM    SWAP    CC   SCORE    LABELS
mm1 [personal-max]  busy      22%    9.1GB   2    5        bleep · deckchecker
mm2 [work-team]     ok        45%    2.4GB   1    20       seatify
mm3                 offline   —      —       —    —        bugdrop
local [personal-max] free     67%    0.0GB   0    67       —
```

Account appears as a bracketed suffix on the machine name. Labels are rendered as a comma-separated list; stale labels are dimmed. Offline machines retain their stale labels.

### Sessions panel (existing TUI, extended)

```
ID       PROJECT              MACHINE  BRANCH  ACCOUNT        LABEL
a1b2c3   neonwatty/bleep      mm1      main    personal-max   bleep
d4e5f6   neonwatty/seatify    mm2      dev     work-team      seatify
```

New columns: `ACCOUNT`, `LABEL`. Press `n` on a row to rename its label.

### Menu bar (SwiftBar)

Menu bar title format:

```
3/4 · 2 CC                    # normal
⚠ 4/4 · 5 CC                  # one machine stressed (amber or red)
```

Dropdown per-machine row:

```
mm1 [personal-max]  busy · 22% mem · 9.1GB swap · 2 CC
  ● bleep       live
  ○ deckchecker stale
```

Swap coloring in the menu bar:
- **Default** when swap is under ~1GB
- **Amber** when swap crosses ~1GB (early pressure signal)
- **Red** when swap crosses ~4GB (actively thrashing)

Thresholds live in `config.toml` alongside existing `stress_threshold`.

Full visual: see [`docs/superpowers/mockups/2026-04-12-menu-bar.html`](../mockups/2026-04-12-menu-bar.html) — open in a browser for the primary state, a stressed variant, and a mixed-accounts variant.

## SwiftBar Plugin

Shipped as `scripts/swiftbar/fleet.10s.sh`. The `10s` suffix tells SwiftBar to refresh every 10 seconds.

Script responsibilities:
1. Run `fleet status --json`.
2. Parse with `jq`.
3. Emit menu bar title line.
4. Emit `---` separator.
5. Emit one dropdown section per machine.
6. Emit footer actions: "Open full dashboard" (launches `fleet status` in a terminal via SwiftBar's `bash=` + `terminal=true`), "Refresh" (SwiftBar's `refresh=true`).

Dependencies: `jq` (standard, documented in plugin README), `fleet` (on `$PATH`), SwiftBar.

Install: user copies `fleet.10s.sh` to their SwiftBar plugin directory (default `~/Library/Application Support/SwiftBar/Plugins/`). No package, no installer. One `cp` command documented in the main `README.md`.

Failure modes:
- `fleet status --json` fails (binary missing, state corrupted) → plugin emits `fleet ⚠` title and an error row in the dropdown.
- `jq` missing → plugin emits `fleet: install jq` title; install instructions in the dropdown.

## Testing Strategy

### Unit tests

| File | Coverage |
|------|----------|
| `internal/session/state_test.go` (extend) | Round-trip serialization with and without `machine_labels`; empty/nil map back-compat; `Session.Account` round-trip |
| `internal/session/labels_test.go` (new) | `AddLabel`, `RemoveLabel`, `ClearLabels`, `ListLabels`, duplicate overwrite, unknown machine error |
| `internal/session/account_test.go` (new) | `SetSessionAccount`; launch-time resolution order (`--account` > `config.default_account` > empty) |
| `cmd/fleet/status_json_test.go` (new) | Feed fake probe result into JSON renderer, assert structure against committed schema fixture |
| `internal/tui/sessions_test.go` (extend or add) | Renders new ACCOUNT/LABEL columns; `n` keybinding triggers label-edit action |
| `internal/tui/machines_test.go` (extend or add) | Machine name renders `[account]` suffix; labels dim when stale |

### Plugin test

A shell-level test pipes a committed `scripts/swiftbar/fixtures/status.json` through the plugin script and diffs the output against `scripts/swiftbar/fixtures/status.expected.txt`. Runs via `make test-swiftbar` (new target) folded into `make check`. No SwiftBar install required in CI.

### Out of scope for this spec

- Live probing of real remote machines (already covered by existing `probe_test.go` with mocked runners).
- SwiftBar's own rendering behavior (upstream).
- Any form of usage tracking, quota enforcement, or API calls to Claude — accounts are purely user-assigned labels.

## File-Level Change Summary

### Modified

- `internal/session/state.go` — add `MachineLabels`, `MachineLabel`, `Session.Account`
- `internal/session/launch.go` — accept `Account` in `LaunchOpts`; resolve flag → config default → empty
- `internal/config/config.go` — add `DefaultAccount` to `Machine`
- `internal/tui/machines.go` — render `[account]` suffix and labels list, dim stale labels
- `internal/tui/sessions.go` — new `ACCOUNT` and `LABEL` columns; `n` keybinding
- `internal/tui/app.go` — wire `n` keybinding to label-edit action
- `cmd/fleet/main.go` — register `label` and `account` subcommands; add `--json` flag on `status`; add `--account` and `--name` flags on `launch`
- `README.md` — new "Menu bar" section with install instructions
- `config.example.toml` — add commented `default_account = "personal-max"` example

### New

- `internal/session/labels.go` + `labels_test.go`
- `internal/session/account.go` + `account_test.go`
- `cmd/fleet/label.go`
- `cmd/fleet/account.go`
- `cmd/fleet/status_json.go` + `status_json_test.go`
- `scripts/swiftbar/fleet.10s.sh`
- `scripts/swiftbar/fixtures/status.json`
- `scripts/swiftbar/fixtures/status.expected.txt`
- `scripts/swiftbar/README.md`

## Scope Estimate

- Data model, label/account CLI subcommands, and their unit tests: ~1 day
- TUI rendering and keybinding: ~half day
- `fleet status --json`, SwiftBar plugin, fixture test, docs: ~half day
- **Total: ~2 days of focused work.**

## Non-Goals

- Automatic label derivation from repo name.
- Machine-level account tracking as the source of truth (only as a default that feeds session-level at launch time).
- A standalone Swift menu bar app (explicitly rejected in favor of SwiftBar).
- Duplicate-account warnings when the same account is labelled on multiple live sessions (rejected during brainstorming as scope creep).
- Any integration with Claude's usage API.
- Auto-discovery of labels from Ghostty window titles (rejected as fragile).
