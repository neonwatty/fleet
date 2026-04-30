# Demo script

## Goal

Show that Fleet turns several local Macs into practical coding-agent capacity without a cloud scheduler.

## Flow

1. Show the Mac menu bar indicator: `3/4 · 2 CC`.
2. Open the popover and point out machine health, swap, labels, accounts, and copyable SSH targets.
3. Run:

   ```sh
   fleet status
   ```

4. Show the TUI panels: Machines, Sessions, Tunnels, Processes.
5. Launch a project:

   ```sh
   fleet launch neonwatty/my-project
   ```

6. Narrate the placement decision: Fleet picked the machine with the best available memory and lowest swap pressure.
7. Start a dev server on the remote machine.
8. Open the local tunnel URL and show that the app is reachable from the control Mac.
9. Exit the session and show cleanup.
10. Run:

    ```sh
    fleet clean --dry-run
    fleet doctor
    ```

## Closing line

Fleet is for the small but increasingly common setup where your local Macs are the compute pool for coding agents.
