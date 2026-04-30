# Screenshot guide

Use these shots to replace the starter mock visuals when the app is ready for a public launch pass.

## Dashboard

Target file:

```text
site/assets/fleet-dashboard.png
```

Current status: captured from the live CLI on April 30, 2026. Recapture after
meaningful visual changes to the TUI.

Capture:

1. Open a terminal at roughly 1200x800.
2. Run `fleet status` with a representative config.
3. Show at least three machines, one active session, one tunnel, and one process group.
4. Avoid personal project names or account labels unless they are intentional.

## Menu bar

Target file:

```text
site/assets/fleet-menubar.png
```

Current status: captured from the live app on April 30, 2026. Recapture after
meaningful visual changes to the menu bar app.

Capture:

1. Launch `FleetMenuBar.app`.
2. Open the popover.
3. Show online count, health bands, labels, accounts, and copyable SSH target rows.
4. Crop enough surrounding menu bar to make the surface recognizable.

## Demo loop

Target file:

```text
site/assets/demo-loop.gif
```

Suggested flow:

1. `fleet status --json | jq '.machines[] | {name, health, score, swap_gb}'`
2. `fleet launch neonwatty/my-project`
3. Show selected machine and tunnel.
4. Open the local dev URL.
5. Exit and show cleanup.

Keep the final clip under 20 seconds for social posts and README embedding.
