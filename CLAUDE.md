# Fleet

Local Mac fleet manager for distributing Claude Code instances.

## Quick Reference

- **Language**: Go 1.26
- **Build**: `make build` → `bin/fleet`
- **Test**: `make test` (or `go test -race ./...`)
- **Lint**: `make lint` (requires `golangci-lint`)
- **All checks**: `make check`

## Architecture

Single binary, runs on MacBook Air. Shells out to `ssh` for remote ops.

- `internal/config/` — TOML config parsing
- `internal/exec/` — Local + SSH command execution
- `internal/machine/` — Health probing + scoring
- `internal/session/` — State, launch, teardown, clean
- `internal/tunnel/` — SSH port forwarding + port detection
- `internal/tui/` — Bubble Tea dashboard

## Testing

```bash
go test -race ./...                    # All tests
go test -race ./internal/machine/ -v   # Single package
```

## Config

`~/.fleet/config.toml` — see `config.example.toml` for format.
