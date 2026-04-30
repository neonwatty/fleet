# Agent Usage

Fleet exposes the current state of the fleet as JSON so Claude Code, Codex, or
another agent can make a placement decision without Fleet needing a separate
scheduler API or a Fleet-specific SSH command. Fleet reports the machine state;
you or the agent connect with the normal SSH alias/target from your SSH config.

```sh
fleet status --json
```

The response includes one object per enabled machine. The most useful fields for
agent allocation are:

| Field | Meaning |
|-------|---------|
| `name` | Fleet machine name from `config.toml` |
| `ssh_target` | Direct SSH target, including configured user when present |
| `status` | `online` or `offline` |
| `health` | Human-friendly score band: `free`, `ok`, `busy`, `stressed` |
| `score` | Numeric capacity score; higher is better |
| `mem_available_pct` | Available memory percentage |
| `swap_gb` | Current swap usage in GiB |
| `cc_count` | Detected Claude Code process count |
| `accounts` | Account labels from active sessions |
| `agent_processes` | Observed Claude Code/Codex process groups |
| `labels` | User labels for live or remembered work on that machine |

Example:

```json
{
  "name": "mm2",
  "ssh_target": "jeremywatt@mm2",
  "status": "online",
  "mem_available_pct": 42,
  "swap_gb": 0,
  "cc_count": 0,
  "score": 42.3,
  "health": "free",
  "accounts": [],
  "agent_processes": [
    {"kind": "codex", "count": 1, "rss_mb": 512, "pids": [7777]}
  ],
  "labels": []
}
```

`agent_processes` is intentionally observational. It summarizes matching
processes discovered during Fleet's normal remote process scan, including work
started manually over SSH. It does not mean Fleet launched or owns those
processes.

## Recommended Workflow

Use Fleet before starting substantial work on a repo:

1. From the control machine, run `fleet status --json`.
2. Choose an `online` machine with acceptable `health`, high `score`, low
   `swap_gb`, and a manageable `agent_processes` count.
3. Connect with the returned `ssh_target` using normal SSH:

   ```sh
   ssh jeremywatt@mm2
   ```

4. On that machine, `cd` into the repo or clone it, then start Claude Code,
   Codex, or the development process there.

If the machine has a host alias in `~/.ssh/config`, that alias is enough:

```sh
ssh mm2
```

Use `ssh_target` when you want the exact target Fleet is configured to use,
including the user.

A simple agent instruction can stay high level:

> Run `fleet status --json`, choose an online machine with good capacity, explain
> the choice, then SSH to its `ssh_target` and start work there.

For a repo-specific instruction in `AGENTS.md` or `CLAUDE.md`:

```text
Before starting long-running development work, use Fleet from the control
machine. Run `fleet status --json`, inspect `.machines[]`, choose an online
machine with good health, high score, low swap_gb, and few agent_processes, then
connect using the returned ssh_target with normal SSH. Once connected, work from
that remote shell.
```

## Mid-Session Use

Using Fleet in the middle of an existing agent chat is fine for choosing a
machine, but automatic "all future tool calls happen over SSH" behavior depends
on the agent environment. Many agents do not transparently rebind their working
directory, shell, filesystem tools, and test commands to a remote host after a
plain-language instruction.

The reliable order of operations is:

1. Ask the local agent to run `fleet status --json`.
2. Pick a machine and SSH target.
3. Start or attach a shell/session on that target.
4. Start the remote Claude Code or Codex session from that SSH shell.

Mid-session, prefer explicit remote commands:

```sh
ssh jeremywatt@mm2 'cd ~/fleet-work/my-app && git status --short'
```

For sustained development, open a dedicated SSH session and start the agent
there. That keeps command execution, filesystem access, tests, and long-running
processes on the chosen machine instead of splitting state between local and
remote contexts.

For manual inspection:

```sh
fleet status --json |
  jq '.machines[] | {name, ssh_target, status, health, score, mem_available_pct, swap_gb, cc_count, agent_processes}'
```

Fleet intentionally does not need to pick the machine for the agent in this
workflow. The JSON gives the agent enough context to choose, explain the choice,
and use the correct SSH alias.
