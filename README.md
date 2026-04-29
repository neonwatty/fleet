# Fleet

A CLI for distributing Claude Code instances across a local Mac fleet. Auto-picks the healthiest machine, sets up SSH tunnels for dev servers (so OAuth callbacks work), and provides a live TUI dashboard.

```
                    ┌─────────────────────────┐
                    │   MacBook Air M2 (16GB) │
                    │   "Control Plane"        │
                    │                          │
                    │   fleet launch            │
                    │   fleet status            │
                    └──────┬──────┬──────┬─────┘
                      SSH  │      │      │  SSH
              ┌────────────┘      │      └────────────┐
              │                   │                    │
        ┌─────▼──────┐    ┌──────▼─────┐    ┌────────▼────┐
        │  Mac Mini  │    │  Mac Mini  │    │  Mac Mini   │
        │  M4 (16GB) │    │  M4 (16GB) │    │  M4 (16GB)  │
        │  "mm1"     │    │  "mm2"     │    │  "mm3"      │
        └────────────┘    └────────────┘    └─────────────┘
                Local network
```

## Install

```bash
git clone https://github.com/neonwatty/fleet.git
cd fleet
make build
cp bin/fleet /opt/homebrew/bin/fleet
```

Requires Go 1.26+ and SSH access to your machines (configured in `~/.ssh/config`).

## Setup

Copy the example config and edit for your machines:

```bash
mkdir -p ~/.fleet
cp config.example.toml ~/.fleet/config.toml
```

```toml
[settings]
port_range = [4000, 4999]
poll_interval = 5
stress_threshold = 20
worktree_base = "~/fleet-work"
bare_repo_base = "~/fleet-repos"

[[machines]]
name = "local"
host = "localhost"
user = ""
enabled = true

[[machines]]
name = "mm1"
host = "mm1"
user = "neonwatty"
enabled = true
```

Each machine entry uses the SSH host alias from `~/.ssh/config` — fleet shells out to the system `ssh` binary, so it inherits your keys, ControlMaster, and aliases.

## Commands

### `fleet launch <org/repo>`

Auto-picks the healthiest machine and launches Claude Code there.

```bash
fleet launch neonwatty/my-project           # auto-pick best machine
fleet launch neonwatty/my-project -t mm2    # force a specific machine
fleet launch neonwatty/my-project -b feat   # check out a specific branch
```

What happens:
1. Probes all machines for RAM, swap, and Claude Code instance count
2. Scores and ranks them (highest available memory, lowest swap pressure)
3. If all machines are stressed, asks for confirmation
4. On the chosen machine: ensures a bare clone exists, creates a fresh git worktree
5. Sets up an SSH tunnel for the dev server port (so `localhost:4001` reaches the remote)
6. Drops you into Claude Code in your terminal
7. On exit: tears down the tunnel, removes the worktree, cleans up state

### `fleet status`

Opens a live TUI dashboard with four panels:

```
┌─────────────────────────────────────────────────┐
│ Fleet Dashboard                                 │
│                                                 │
│ ┌ Machines ───────────────────────────────────┐ │
│ │ MACHINE    STATUS   MEM AVAIL  SWAP   CC    │ │
│ │ local      online   45%        0.0GB  1  ok │ │
│ │ mm1        online   22%        9.1GB  2  .. │ │
│ │ mm2        online   31%        2.4GB  2  .. │ │
│ └─────────────────────────────────────────────┘ │
│ ┌ Sessions ───────────────────────────────────┐ │
│ │ ID       PROJECT              MACHINE       │ │
│ │ a1b2c3   neonwatty/seatify    mm2           │ │
│ └─────────────────────────────────────────────┘ │
│ ┌ Tunnels ────────────────────────────────────┐ │
│ │ localhost:4001 → mm2:3000   neonwatty/seat..│ │
│ └─────────────────────────────────────────────┘ │
│ ┌ Processes on mm1 ───────────────────────────┐ │
│ │ CATEGORY       COUNT  RSS       DETAIL      │ │
│ │ Dev Servers    2      679MB     next         │ │
│ │ Claude Code    2      616MB                  │ │
│ │ Chrome         12     900MB     12 tabs      │ │
│ │ Docker         3      264MB     3 procs      │ │
│ └─────────────────────────────────────────────┘ │
│                                                 │
│ tab: switch | j/k: navigate | d: kill | q: quit│
└─────────────────────────────────────────────────┘
```

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Switch between panels |
| `j` / `k` | Navigate rows. On Machines panel, also selects which machine's processes are shown |
| `o` | Open tunnel URL in browser (Tunnels panel) |
| `x` | Kill a fleet session and clean up its worktree/tunnel (Sessions panel) |
| `d` | Kill a process group on the selected machine (Processes panel) |
| `q` / `ctrl+c` | Quit |

Pass `--json` to emit fleet state as JSON instead of opening the TUI — used by the menu bar app and any other scripted consumer:

```bash
fleet status --json
```

### `fleet label`

Manage user-chosen nicknames attached to Claude Code sessions on each machine. Labels survive remote restarts and render as "stale" in both the TUI and the menu bar when their linked session is gone.

```bash
fleet label set mm1 bleep                    # add orphan label on mm1
fleet label set mm1 bleep --session a1b2c3   # link a label to an existing session
fleet label set mm1 bleep --remove           # remove a single label
fleet label set mm1 --clear                  # remove all labels on mm1
fleet label list                             # list labels across the fleet
fleet label list mm1                         # list labels on one machine
```

### `fleet account`

Set or clear the Claude subscription label on an existing session. Use to track which subscription each session is burning when juggling multiple accounts.

```bash
fleet account a1b2c3 personal-max
fleet account a1b2c3 --clear
```

Per-machine defaults live in `config.toml` via `default_account` — sessions launched with `fleet launch -t mm1` inherit that default unless overridden with `--account`.

### `fleet clean`

Reconciles state against reality. Finds orphaned worktrees (no Claude process running), stale state entries, and orphaned tunnel processes — cleans them all up. Also clears dangling `SessionID` references on labels whose sessions no longer exist, converting them to orphan labels.

```bash
fleet clean
fleet clean --dry-run
fleet doctor
fleet doctor --machine mm1
```

`fleet doctor` is read-only. It checks config loading, state readability, SSH reachability, and configured remote base directories, then prints the same cleanup reconciliation plan as `fleet clean --dry-run`.

## Menu Bar

Fleet ships a native macOS menu bar app that shows a compact fleet indicator
and a popover with per-machine health, swap, and labels. See
[`menubar/README.md`](menubar/README.md) for install instructions.

At a glance: `3/4 · 2 CC` means 3 of 4 machines are online with 2 live Claude
Code instances. Click the indicator for a per-machine popover with accounts,
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

## Health Score

Fleet ranks machines using a simple formula:

```
score = available_memory_% - (swap_used_% × 0.5)
```

| Component | What it measures |
|-----------|-----------------|
| `available_memory_%` | (free + inactive pages) / total RAM. Inactive pages are reclaimable on macOS |
| `swap_used_% × 0.5` | Penalty for swap pressure, halved so it doesn't dominate |

Claude Code instance count is still probed and displayed for informational purposes but does not affect the score — its memory footprint is already captured in `available_memory_%`.

The score maps to a label:

| Label | Score | Meaning |
|-------|-------|---------|
| free | >= 30 | Plenty of headroom (may still be running CC instances) |
| ok | >= 10 | Running workloads but has room |
| busy | >= -20 | Under load, will work but slower |
| stressed | < -20 | Heavy swap, likely sluggish |

The `free` label means *spare capacity*, not *no activity* — a machine actively running CC instances can land in `free` as long as memory headroom is sufficient.

## OAuth Tunnel Pinning

Most projects register OAuth callbacks to `localhost:3000`. If fleet auto-assigns a different local port, OAuth breaks. To pin the local tunnel port, add a `.fleet.toml` to your project root:

```toml
dev_port = 3000
tunnel_local_port = 3000
```

This tells fleet to forward `localhost:3000` on the MacBook to port `3000` on the remote machine, so OAuth callbacks work unchanged.

## Development

```bash
make test           # Run all tests with race detector
make lint           # Run golangci-lint
make build          # Build to bin/fleet
make check          # Run all checks (fmt, lint, vet, test, build)
```

## License

MIT
