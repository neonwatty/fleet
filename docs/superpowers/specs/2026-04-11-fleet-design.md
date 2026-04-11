# Fleet — Local Mac Cluster Manager for Claude Code

## Problem

Managing 10-12 concurrent Claude Code instances across a 4-machine local network (MacBook Air M2 + 3x Mac Mini M4, all 16GB RAM) requires manual SSH session management, mental load balancing, and clunky Screen Sharing to access remote dev servers. OAuth callbacks break when accessing remote dev servers via LAN IP instead of localhost.

## Solution

A single Go binary (`fleet`) that runs on the MacBook Air and automates instance placement, SSH tunnel management, and fleet monitoring. The Mac Minis remain dumb SSH targets with nothing installed.

## Architecture

```
MacBook Air (control plane + pool member)
├── ~/.fleet/config.toml    (machine list, thresholds, ports)
├── ~/.fleet/state.json     (active sessions, tunnel mappings)
├── fleet launch            (auto-place Claude Code on best machine)
└── fleet status            (Bubble Tea TUI dashboard)
    ├── SSH → mm1 (Mac Mini M4, 16GB)
    ├── SSH → mm2 (Mac Mini M4, 16GB)
    └── SSH → mm3 (Mac Mini M4, 16GB)
```

For local launches (MacBook Air chosen as target), SSH is skipped and commands run directly.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go + Bubble Tea | Single binary, no runtime deps, snappy TUI, easy to distribute |
| SSH strategy | Shell out to system `ssh` | Inherits ~/.ssh/config (ControlMaster, keys, aliases) for free |
| Repo management | Bare clone + worktrees | Clean isolation per session, cheap, matches existing workflow |
| Tunnel ports | Auto-assign from 4000-4999 | Avoids conflicts, TUI is source of truth for port mappings |
| Worktree cleanup | Auto on session exit | Tunnels and worktrees tear down together, consistent clean state |
| Overload handling | Ask before launching | "All machines stressed, launch anyway on mm2?" prompt |
| Offline machines | Skip silently | TUI shows them as offline, placement ignores them |
| MacBook Air | In the placement pool | Treated as a target like the Minis, with local-exec special case |
| Multiple instances/project | Supported | Each launch creates its own worktree, fully independent |
| Config location | ~/.fleet/config.toml | Single source of truth, separate from SSH config |

## Commands

### `fleet launch <org/repo> [--branch <name>] [--target <machine>]`

1. Read `~/.fleet/config.toml` for machine list and thresholds
2. Poll all enabled machines in parallel via SSH:
   - Memory pressure (`vm_stat` + `sysctl vm.swapusage`)
   - Running Claude Code process count (`ps aux | grep claude`)
   - Available disk space
3. Rank machines by health score:
   ```
   available_memory = (pages_free + pages_inactive) * page_size
   available_pct = available_memory / total_memory * 100
   swap_used_pct = swap_used / swap_total * 100  (0 if no swap allocated)
   score = available_pct - (swap_used_pct * 0.5) - (claude_count * 10)
   ```
   Note: macOS "inactive" pages are reclaimable, so they count as available.
4. If best score < `stress_threshold` → prompt: "All machines stressed, launch anyway on <best>? [y/n]"
5. If `--target` given, skip ranking and use that machine
6. On the chosen machine:
   a. Check if bare clone exists at `~/fleet-repos/<org>/<repo>.git`
   b. If not, `git clone --bare`
   c. `git fetch origin` (update refs)
   d. Create worktree at `~/fleet-work/<repo>-<timestamp>` from `origin/main` (or `--branch`)
7. Set up SSH tunnel: pick next available port from configured range, forward `local:<port>` → `remote:<dev_port>`
8. Write session to `~/.fleet/state.json`
9. SSH into the machine, cd to the worktree, exec `claude`
10. On exit: tear down tunnel, remove remote worktree, prune, update state

### `fleet status`

Bubble Tea TUI with four panels, live-refreshing every 5 seconds:

| Panel | Content |
|-------|---------|
| Machines | hostname, CPU%, mem%, swap%, Claude count, online/offline |
| Sessions | project, machine, branch, worktree path, uptime |
| Tunnels | local port → remote machine:port, associated project |
| Actions | kill session, open tunnel URL in browser, SSH into machine |

### `fleet clean`

Reconciles state against reality:
1. Read `state.json` for all recorded sessions
2. For each, SSH into the machine and check if worktree and Claude process still exist
3. Orphaned worktrees (no Claude process) → remove
4. Stale state entries (nothing on disk) → remove
5. Orphaned local tunnel processes → kill

## Configuration

### `~/.fleet/config.toml`

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

[[machines]]
name = "mm2"
host = "mm2"
user = "jeremywatt"
enabled = true

[[machines]]
name = "mm3"
host = "mm3"
user = "jeremywatt"
enabled = true
```

### `~/.fleet/state.json`

```json
{
  "sessions": [
    {
      "id": "a1b2c3",
      "project": "neonwatty/seatify",
      "machine": "mm2",
      "branch": "main",
      "worktree_path": "/Users/jeremywatt/fleet-work/seatify-1744355200",
      "tunnel": {
        "local_port": 4001,
        "remote_port": 3000
      },
      "started_at": "2026-04-11T08:40:00Z",
      "pid": 12345
    }
  ]
}
```

## Project Structure

```
fleet/
├── cmd/
│   └── fleet/
│       └── main.go              # Entry point, cobra CLI
├── internal/
│   ├── config/
│   │   └── config.go            # Parse ~/.fleet/config.toml
│   ├── machine/
│   │   ├── probe.go             # SSH health checks (RAM, CPU, swap, processes)
│   │   └── score.go             # Rank machines by health score
│   ├── session/
│   │   ├── launch.go            # Worktree setup, tunnel, exec claude
│   │   ├── teardown.go          # Cleanup worktree + tunnel on exit
│   │   └── state.go             # Read/write state.json
│   ├── tunnel/
│   │   └── tunnel.go            # SSH local port forwarding lifecycle
│   └── tui/
│       ├── app.go               # Bubble Tea main model
│       ├── machines.go          # Machines panel
│       ├── sessions.go          # Sessions panel
│       ├── tunnels.go           # Tunnels panel
│       └── actions.go           # Action handlers
├── go.mod
├── go.sum
├── config.example.toml
└── README.md
```

### Dependencies

| Library | Purpose |
|---------|---------|
| cobra | CLI subcommands |
| bubbletea | TUI framework |
| lipgloss | TUI styling |
| bubbles | TUI components (tables, spinners) |
| BurntSushi/toml | Config parsing |

## Signal Handling and Edge Cases

### Clean exit

1. Fleet traps `SIGINT` and `SIGTERM`
2. On signal or normal Claude Code exit:
   - Kill SSH tunnel subprocess
   - SSH into machine, `rm -rf` worktree, `git worktree prune`
   - Remove session from `state.json`
3. If fleet crashes (SIGKILL, power loss) → `fleet clean` reconciles

### Dev server port detection and OAuth tunnel pinning

Most projects use OAuth callbacks registered to `localhost:<port>` (e.g., Supabase auth
callbacks to `localhost:3000/api/auth/callback`). If the SSH tunnel's local port doesn't
match the registered callback port, OAuth breaks. Therefore:

1. Check `.fleet.toml` in repo root for explicit config:
   ```toml
   dev_port = 3000          # Port the dev server listens on remotely
   tunnel_local_port = 3000 # Pin local tunnel to this port (for OAuth callbacks)
   ```
2. If `tunnel_local_port` is set, use it as the local side of the SSH tunnel
   (`ssh -L 3000:localhost:3000 <machine>`) — this ensures `localhost:3000` on the
   MacBook Air reaches the remote dev server and OAuth callbacks work unchanged
3. If `tunnel_local_port` is not set, auto-assign from the 4000-4999 range
4. If no `.fleet.toml`, check `package.json` → parse `"dev"` script for port flags
5. Fall back to remote port `3000`

**Constraint**: Only one project with a pinned `tunnel_local_port` of a given value can be
tunneled at a time. If a second project tries to pin the same local port, fleet warns and
falls back to auto-assign.

### Local machine special case

When MacBook Air is chosen:
- No SSH — run git and claude directly
- No tunnel — dev server already on localhost
- Same worktree/bare-repo paths
- Teardown still cleans up worktree and state
