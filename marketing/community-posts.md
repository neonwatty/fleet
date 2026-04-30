# Community post drafts

## Hacker News

Title:

```text
Show HN: Fleet - run coding agents across a local Mac mini pool
```

Post:

```text
I built Fleet after my coding-agent workflow outgrew one Mac.

The setup is intentionally small: a MacBook as the control plane and a few Mac minis as SSH targets. Fleet probes memory/swap/agent process counts, picks a healthy machine, creates a fresh worktree, opens SSH tunnels back to localhost, and starts the agent command.

The localhost tunnel piece is the thing that made this worth building for me. OAuth callbacks and web app dev servers usually expect localhost ports, even when the actual session is running remotely.

There is also a Bubble Tea TUI, a native macOS menu bar app, and `fleet status --json` so agents/scripts can make their own placement decision.

It is not trying to be Kubernetes for Macs. It is a narrow tool for local Mac capacity management when you are running several Claude Code/Codex-style sessions.

Repo: https://github.com/neonwatty/fleet
```

## Reddit

Title:

```text
I built a small local Mac fleet manager for coding-agent sessions
```

Post:

```text
I have been running more Claude Code/Codex-style sessions than one Mac can comfortably handle, so I built Fleet.

It treats nearby Macs as a small SSH-backed pool:

- probes memory, swap, online status, and active agent process counts
- picks the healthiest Mac for a launch
- creates isolated git worktrees
- opens local SSH tunnels for remote dev servers
- keeps OAuth callbacks working on localhost
- shows state in a TUI and native macOS menu bar app

The remote machines do not run a daemon. Fleet uses your normal SSH config.

This is deliberately narrow: local Mac capacity for coding agents, not a general cluster manager.

Repo: https://github.com/neonwatty/fleet
```
