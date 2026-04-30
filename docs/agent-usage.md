# Agent Usage

Fleet exposes the current state of the fleet as JSON so Claude Code, Codex, or
another agent can make a placement decision without Fleet needing a separate
scheduler API.

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
  "labels": []
}
```

A simple agent instruction can stay high level:

> Run `fleet status --json`, choose an online machine with good capacity, then
> SSH to its `ssh_target` and start work there.

For manual inspection:

```sh
fleet status --json |
  jq '.machines[] | {name, ssh_target, status, health, score, mem_available_pct, swap_gb, cc_count}'
```

Fleet intentionally does not need to pick the machine for the agent in this
workflow. The JSON gives the agent enough context to choose, explain the choice,
and use the correct SSH alias.
