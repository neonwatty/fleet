# Fleet launch post draft

I built Fleet because running many coding-agent sessions across a MacBook and a few Mac minis turned into manual scheduling work.

Before Fleet, I was checking which machine had memory headroom, SSH-ing into the right box, creating worktrees, wiring dev-server ports back to localhost, and cleaning up stale sessions by hand. OAuth-heavy web apps made this especially annoying because callbacks usually expect `localhost:3000`.

Fleet turns that into one workflow:

```sh
fleet launch neonwatty/my-project
```

It probes the configured Macs, picks the healthiest target, prepares a fresh worktree, opens an SSH tunnel for the dev server, and starts the coding-agent command. `fleet status` gives a live terminal dashboard, and the native menu bar app shows the same fleet health at a glance.

The target machines stay simple. They only need SSH access. Fleet shells out to the system `ssh`, so it uses the same keys, aliases, and ControlMaster setup you already have.

The narrow use case is deliberate: local Macs as a small pool for Claude Code, Codex-style agents, and related dev workloads.

Repo: https://github.com/neonwatty/fleet
Releases: https://github.com/neonwatty/fleet/releases/latest
