# Social copy drafts

## Short

Fleet is a local Mac fleet control plane for coding agents.

It auto-picks the healthiest Mac, launches isolated worktrees, keeps remote dev servers reachable through localhost SSH tunnels, and shows the whole fleet in a TUI + native menu bar app.

https://github.com/neonwatty/fleet

## Thread

I built Fleet to run more coding-agent sessions across my Macs without manually juggling SSH.

The setup is a MacBook control plane plus Mac minis as simple SSH targets.

`fleet launch org/repo` probes the machines, picks the healthiest target, creates a fresh worktree, opens a localhost-safe tunnel, and starts the agent command.

The tunnel part matters for web apps. If OAuth expects `localhost:3000`, Fleet can pin that local port while the dev server runs on a remote Mac.

`fleet status` gives a live TUI for machines, sessions, tunnels, and process groups.

There is also a native macOS menu bar app for quick health, swap, labels, accounts, and SSH targets.

No daemon on target machines. Just SSH.

https://github.com/neonwatty/fleet
