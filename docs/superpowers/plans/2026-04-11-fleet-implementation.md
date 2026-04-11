# Fleet Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI (`fleet`) that auto-distributes Claude Code instances across a local Mac fleet with SSH tunneling and a Bubble Tea TUI dashboard.

**Architecture:** Single binary on the MacBook Air. Shells out to system `ssh` for all remote operations. Config in `~/.fleet/config.toml`, runtime state in `~/.fleet/state.json`. Three commands: `launch`, `status`, `clean`.

**Tech Stack:** Go 1.26, Cobra (CLI), Bubble Tea + Lipgloss + Bubbles (TUI), BurntSushi/toml (config)

**Spec:** `docs/superpowers/specs/2026-04-11-fleet-design.md`

---

## File Map

| File | Responsibility |
|------|----------------|
| `cmd/fleet/main.go` | Entry point, cobra root + subcommands |
| `internal/config/config.go` | Parse `~/.fleet/config.toml`, validate, expand paths |
| `internal/config/config_test.go` | Config parsing tests |
| `internal/exec/exec.go` | Run commands locally or via SSH based on machine type |
| `internal/exec/exec_test.go` | Executor tests |
| `internal/machine/probe.go` | Collect RAM/CPU/swap/processes from a machine |
| `internal/machine/probe_test.go` | Probe parsing tests (unit test with fixture output) |
| `internal/machine/score.go` | Rank machines by health score, pick best |
| `internal/machine/score_test.go` | Scoring logic tests |
| `internal/session/state.go` | Read/write/lock `~/.fleet/state.json` |
| `internal/session/state_test.go` | State CRUD tests |
| `internal/tunnel/tunnel.go` | SSH `-L` tunnel lifecycle, port allocation |
| `internal/tunnel/tunnel_test.go` | Port allocation tests |
| `internal/tunnel/portdetect.go` | Detect dev server port from `.fleet.toml` / `package.json` |
| `internal/tunnel/portdetect_test.go` | Port detection tests |
| `internal/session/launch.go` | Orchestrate: bare clone → worktree → tunnel → exec claude |
| `internal/session/teardown.go` | Signal trap, cleanup worktree + tunnel + state |
| `internal/session/clean.go` | Reconcile state against reality |
| `internal/session/clean_test.go` | Clean reconciliation logic tests |
| `internal/tui/app.go` | Bubble Tea main model, layout, key bindings |
| `internal/tui/machines.go` | Machines panel (health table) |
| `internal/tui/sessions.go` | Sessions panel (active sessions table) |
| `internal/tui/tunnels.go` | Tunnels panel (port mapping table) |
| `internal/tui/actions.go` | Action handlers (kill, open browser, SSH) |
| `internal/tui/styles.go` | Lipgloss style definitions |
| `.golangci.yml` | Linter config |
| `.github/workflows/ci.yml` | CI pipeline |
| `Makefile` | Build/lint/test/fmt convenience targets |
| `scripts/pre-push.sh` | Pre-push hook script |
| `commitlint.config.cjs` | Conventional commit enforcement |
| `config.example.toml` | Example config for new users |
| `.gitignore` | Ignore bin/, coverage, etc. |

---

### Task 1: Project Scaffolding and Engineering Standards

**Files:**
- Create: `.gitignore`
- Create: `Makefile`
- Create: `.golangci.yml`
- Create: `scripts/pre-push.sh`
- Create: `commitlint.config.cjs`
- Create: `.github/workflows/ci.yml`
- Create: `config.example.toml`

- [ ] **Step 1: Create `.gitignore`**

```gitignore
# Build output
bin/
dist/

# Test artifacts
coverage.out
coverage.html

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Fleet runtime (never commit user state)
state.json
```

- [ ] **Step 2: Create `Makefile`**

```makefile
.PHONY: lint test test-coverage build fmt vet check clean install

BINARY := fleet
BUILD_DIR := bin

lint:
	golangci-lint run ./...

test:
	go test -race -count=1 ./...

test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/fleet/

fmt:
	gofmt -w .
	goimports -w .

vet:
	go vet ./...

check: fmt lint vet test build

clean:
	rm -rf $(BUILD_DIR)/ coverage.out

install: build
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY)
```

- [ ] **Step 3: Create `.golangci.yml`**

```yaml
run:
  timeout: 5m

linters:
  enable:
    - unused
    - unparam
    - ineffassign
    - gofmt
    - goimports
    - misspell
    - govet
    - staticcheck
    - errcheck
    - gosec
    - gocyclo
    - funlen
    - lll
    - goconst
    - dupl
    - prealloc

linters-settings:
  funlen:
    lines: 100
    statements: 50
  lll:
    line-length: 120
  gocyclo:
    min-complexity: 15
  dupl:
    threshold: 100

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

- [ ] **Step 4: Create `scripts/pre-push.sh`**

```bash
#!/bin/bash
set -euo pipefail

echo "=== 1/5 Format check ==="
UNFORMATTED=$(gofmt -l . 2>&1 || true)
if [ -n "$UNFORMATTED" ]; then
  echo "Unformatted files:"
  echo "$UNFORMATTED"
  echo "Run: gofmt -w ."
  exit 1
fi

echo "=== 2/5 Lint ==="
golangci-lint run ./...

echo "=== 3/5 Vet ==="
go vet ./...

echo "=== 4/5 Unit tests ==="
go test -race -count=1 ./...

echo "=== 5/5 Build ==="
go build -o /dev/null ./cmd/fleet/

echo ""
echo "All checks passed."
```

Run: `chmod +x scripts/pre-push.sh`

- [ ] **Step 5: Create `commitlint.config.cjs`**

```javascript
module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    'type-enum': [
      2,
      'always',
      ['feat', 'fix', 'perf', 'security', 'docs', 'test', 'chore', 'refactor', 'ci'],
    ],
  },
};
```

- [ ] **Step 6: Create `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  pull_request:
    branches: [main]
    types: [opened, synchronize, reopened]
  push:
    branches: [main]

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  quality-checks:
    runs-on: macos-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Check formatting
        run: test -z "$(gofmt -l .)"
      - name: Vet
        run: go vet ./...
      - name: Check go.mod tidy
        run: |
          go mod tidy
          git diff --exit-code go.mod go.sum
      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  unit-tests:
    runs-on: macos-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Run tests with coverage
        run: go test -race -coverprofile=coverage.out -covermode=atomic ./...
      - name: Coverage summary
        run: go tool cover -func=coverage.out | tail -1
      - name: Upload coverage
        uses: actions/upload-artifact@v4
        with:
          name: coverage
          path: coverage.out
          retention-days: 30

  build:
    runs-on: macos-latest
    timeout-minutes: 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Build
        run: go build -o bin/fleet ./cmd/fleet/
      - name: Verify binary
        run: bin/fleet --version

  file-size-check:
    runs-on: macos-latest
    timeout-minutes: 2
    steps:
      - uses: actions/checkout@v4
      - name: Check file sizes
        run: |
          OVERSIZED=$(find . -name '*.go' -exec awk 'END { if (NR > 300) print FILENAME ": " NR " lines" }' {} \;)
          if [ -n "$OVERSIZED" ]; then
            echo "Files exceeding 300 lines:"
            echo "$OVERSIZED"
            exit 1
          fi

  vulnerability-check:
    runs-on: macos-latest
    timeout-minutes: 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@latest
      - name: Run vulnerability check
        run: govulncheck ./...

  ci-gate:
    runs-on: macos-latest
    timeout-minutes: 2
    needs: [quality-checks, unit-tests, build, file-size-check, vulnerability-check]
    if: always()
    steps:
      - name: Check results
        run: |
          if [[ "${{ needs.quality-checks.result }}" != "success" ]] || \
             [[ "${{ needs.unit-tests.result }}" != "success" ]] || \
             [[ "${{ needs.build.result }}" != "success" ]] || \
             [[ "${{ needs.file-size-check.result }}" != "success" ]] || \
             [[ "${{ needs.vulnerability-check.result }}" != "success" ]]; then
            echo "One or more required jobs failed"
            exit 1
          fi
```

- [ ] **Step 7: Create `config.example.toml`**

```toml
# Fleet configuration
# Copy to ~/.fleet/config.toml and adjust for your setup.

[settings]
port_range = [4000, 4999]       # Local port range for tunnel auto-assignment
poll_interval = 5               # TUI refresh interval in seconds
stress_threshold = 20           # Score below this triggers confirmation prompt
worktree_base = "~/fleet-work"  # Remote directory for worktrees
bare_repo_base = "~/fleet-repos" # Remote directory for bare clones

[[machines]]
name = "local"
host = "localhost"
user = ""
enabled = true

# [[machines]]
# name = "mm1"
# host = "mm1"        # Must match an SSH config Host entry
# user = "youruser"
# enabled = true
```

- [ ] **Step 8: Install git hooks and commit**

```bash
git config core.hooksPath scripts
```

Note: This makes git look for hooks in `scripts/`. Rename `scripts/pre-push.sh` to `scripts/pre-push` (no extension) for git to find it.

Run: `mv scripts/pre-push.sh scripts/pre-push`

```bash
git add -A
git commit -m "chore: add engineering standards scaffolding

Makefile, golangci-lint config, GitHub Actions CI pipeline,
pre-push hook, commitlint config, and example config."
```

---

### Task 2: Config Package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write config parsing tests**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
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
enabled = false
`
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Settings.PortRange[0] != 4000 || cfg.Settings.PortRange[1] != 4999 {
		t.Errorf("PortRange = %v, want [4000, 4999]", cfg.Settings.PortRange)
	}
	if cfg.Settings.PollInterval != 5 {
		t.Errorf("PollInterval = %d, want 5", cfg.Settings.PollInterval)
	}
	if cfg.Settings.StressThreshold != 20 {
		t.Errorf("StressThreshold = %d, want 20", cfg.Settings.StressThreshold)
	}
	if len(cfg.Machines) != 3 {
		t.Fatalf("len(Machines) = %d, want 3", len(cfg.Machines))
	}
	if cfg.Machines[1].Name != "mm1" {
		t.Errorf("Machines[1].Name = %q, want %q", cfg.Machines[1].Name, "mm1")
	}
}

func TestEnabledMachines(t *testing.T) {
	cfg := &Config{
		Machines: []Machine{
			{Name: "local", Host: "localhost", Enabled: true},
			{Name: "mm1", Host: "mm1", Enabled: true},
			{Name: "mm2", Host: "mm2", Enabled: false},
		},
	}

	enabled := cfg.EnabledMachines()
	if len(enabled) != 2 {
		t.Fatalf("len(EnabledMachines) = %d, want 2", len(enabled))
	}
	if enabled[1].Name != "mm1" {
		t.Errorf("enabled[1].Name = %q, want %q", enabled[1].Name, "mm1")
	}
}

func TestIsLocal(t *testing.T) {
	local := Machine{Name: "local", Host: "localhost"}
	remote := Machine{Name: "mm1", Host: "mm1"}

	if !local.IsLocal() {
		t.Error("expected localhost to be local")
	}
	if remote.IsLocal() {
		t.Error("expected mm1 to not be local")
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := ExpandPath("~/fleet-work")
	expected := filepath.Join(home, "fleet-work")
	if result != expected {
		t.Errorf("ExpandPath(~/fleet-work) = %q, want %q", result, expected)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.toml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v`
Expected: Compilation error — `config` package doesn't exist yet.

- [ ] **Step 3: Implement config package**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Settings Settings  `toml:"settings"`
	Machines []Machine `toml:"machines"`
}

type Settings struct {
	PortRange       [2]int `toml:"port_range"`
	PollInterval    int    `toml:"poll_interval"`
	StressThreshold int    `toml:"stress_threshold"`
	WorktreeBase    string `toml:"worktree_base"`
	BareRepoBase    string `toml:"bare_repo_base"`
}

type Machine struct {
	Name    string `toml:"name"`
	Host    string `toml:"host"`
	User    string `toml:"user"`
	Enabled bool   `toml:"enabled"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.Settings.WorktreeBase = ExpandPath(cfg.Settings.WorktreeBase)
	cfg.Settings.BareRepoBase = ExpandPath(cfg.Settings.BareRepoBase)

	return &cfg, nil
}

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".fleet", "config.toml")
}

func (c *Config) EnabledMachines() []Machine {
	var result []Machine
	for _, m := range c.Machines {
		if m.Enabled {
			result = append(result, m)
		}
	}
	return result
}

func (m *Machine) IsLocal() bool {
	return m.Host == "localhost" || m.Host == "127.0.0.1"
}

func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
```

- [ ] **Step 4: Install dependency and run tests**

Run: `go get github.com/BurntSushi/toml && go test -race ./internal/config/ -v`
Expected: All 5 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat: add config package for parsing ~/.fleet/config.toml"
```

---

### Task 3: Command Executor (Local + SSH)

**Files:**
- Create: `internal/exec/exec.go`
- Create: `internal/exec/exec_test.go`

- [ ] **Step 1: Write executor tests**

Create `internal/exec/exec_test.go`:

```go
package exec

import (
	"context"
	"strings"
	"testing"

	"github.com/neonwatty/fleet/internal/config"
)

func TestRunLocal(t *testing.T) {
	local := config.Machine{Name: "local", Host: "localhost"}
	out, err := Run(context.Background(), local, "echo hello")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Errorf("Run() = %q, want %q", strings.TrimSpace(out), "hello")
	}
}

func TestRunLocalFailure(t *testing.T) {
	local := config.Machine{Name: "local", Host: "localhost"}
	_, err := Run(context.Background(), local, "false")
	if err == nil {
		t.Error("expected error from failing command")
	}
}

func TestBuildSSHCommand(t *testing.T) {
	m := config.Machine{Name: "mm1", Host: "mm1", User: "neonwatty"}
	args := buildSSHArgs(m, "uname -a")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "mm1") {
		t.Errorf("expected host in args, got: %v", args)
	}
	if !strings.Contains(joined, "uname -a") {
		t.Errorf("expected command in args, got: %v", args)
	}
}

func TestRunWithTimeout(t *testing.T) {
	local := config.Machine{Name: "local", Host: "localhost"}
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	_, err := Run(ctx, local, "sleep 10")
	if err == nil {
		t.Error("expected timeout error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/exec/ -v`
Expected: Compilation error — package doesn't exist.

- [ ] **Step 3: Implement executor**

Create `internal/exec/exec.go`:

```go
package exec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/neonwatty/fleet/internal/config"
)

func Run(ctx context.Context, m config.Machine, command string) (string, error) {
	var cmd *exec.Cmd

	if m.IsLocal() {
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	} else {
		args := buildSSHArgs(m, command)
		cmd = exec.CommandContext(ctx, "ssh", args...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w (stderr: %s)", m.Name, err, stderr.String())
	}

	return stdout.String(), nil
}

func buildSSHArgs(m config.Machine, command string) []string {
	args := []string{
		"-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=accept-new",
	}
	args = append(args, m.Host, command)
	return args
}
```

- [ ] **Step 4: Run tests**

Run: `go test -race ./internal/exec/ -v`
Expected: All 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/exec/
git commit -m "feat: add exec package for local and SSH command execution"
```

---

### Task 4: Machine Health Probing

**Files:**
- Create: `internal/machine/probe.go`
- Create: `internal/machine/probe_test.go`

- [ ] **Step 1: Write probe parsing tests with fixture data**

Create `internal/machine/probe_test.go`:

```go
package machine

import (
	"testing"
)

const fixtureVMStat = `Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               73352.
Pages active:                            302556.
Pages inactive:                          291440.
Pages speculative:                        27036.
Pages throttled:                              0.
Pages wired down:                        107561.
Pages purgeable:                           3429.
"Translation faults":               30465219410.
Pages copy-on-write:                  187575813.
`

const fixtureSwap = `vm.swapusage: total = 4096.00M  used = 2471.94M  free = 1624.06M  (encrypted)`

const fixtureMemsize = `17179869184`

func TestParseVMStat(t *testing.T) {
	free, inactive, pageSize, err := parseVMStat(fixtureVMStat)
	if err != nil {
		t.Fatalf("parseVMStat() error: %v", err)
	}
	if pageSize != 16384 {
		t.Errorf("pageSize = %d, want 16384", pageSize)
	}
	if free != 73352 {
		t.Errorf("free = %d, want 73352", free)
	}
	if inactive != 291440 {
		t.Errorf("inactive = %d, want 291440", inactive)
	}
}

func TestParseSwap(t *testing.T) {
	total, used, err := parseSwap(fixtureSwap)
	if err != nil {
		t.Fatalf("parseSwap() error: %v", err)
	}
	if total != 4096.0 {
		t.Errorf("total = %f, want 4096.0", total)
	}
	if used != 2471.94 {
		t.Errorf("used = %f, want 2471.94", used)
	}
}

func TestParseMemsize(t *testing.T) {
	total, err := parseMemsize(fixtureMemsize)
	if err != nil {
		t.Fatalf("parseMemsize() error: %v", err)
	}
	if total != 17179869184 {
		t.Errorf("total = %d, want 17179869184", total)
	}
}

func TestParseClaudeCount(t *testing.T) {
	output := `  501 12345  0.5  1.2 claude
  501 12346  0.3  0.8 claude --resume
  501 99999  0.1  0.2 grep claude
`
	count := parseClaudeCount(output)
	if count != 2 {
		t.Errorf("count = %d, want 2 (should exclude grep)", count)
	}
}

func TestParseClaudeCountEmpty(t *testing.T) {
	count := parseClaudeCount("")
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/machine/ -v`
Expected: Compilation error.

- [ ] **Step 3: Implement probe with parsers**

Create `internal/machine/probe.go`:

```go
package machine

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	fleetexec "github.com/neonwatty/fleet/internal/exec"
)

type Health struct {
	Name         string
	Online       bool
	TotalMemory  uint64
	AvailMemory  uint64
	SwapTotalMB  float64
	SwapUsedMB   float64
	ClaudeCount  int
	Error        string
}

func Probe(ctx context.Context, m config.Machine) Health {
	h := Health{Name: m.Name}

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := "vm_stat; echo '===SWAP==='; sysctl vm.swapusage; echo '===MEM==='; sysctl -n hw.memsize; echo '===CLAUDE==='; ps aux | grep '[c]laude' || true"
	out, err := fleetexec.Run(probeCtx, m, cmd)
	if err != nil {
		h.Error = err.Error()
		return h
	}

	h.Online = true

	parts := strings.Split(out, "===SWAP===")
	if len(parts) < 2 {
		h.Error = "unexpected probe output format"
		return h
	}
	vmstatOut := parts[0]

	rest := strings.Split(parts[1], "===MEM===")
	if len(rest) < 2 {
		h.Error = "unexpected probe output format"
		return h
	}
	swapOut := strings.TrimSpace(rest[0])

	rest2 := strings.Split(rest[1], "===CLAUDE===")
	if len(rest2) < 2 {
		h.Error = "unexpected probe output format"
		return h
	}
	memsizeOut := strings.TrimSpace(rest2[0])
	claudeOut := rest2[1]

	free, inactive, pageSize, err := parseVMStat(vmstatOut)
	if err != nil {
		h.Error = fmt.Sprintf("parse vm_stat: %v", err)
		return h
	}

	totalMem, err := parseMemsize(memsizeOut)
	if err != nil {
		h.Error = fmt.Sprintf("parse memsize: %v", err)
		return h
	}

	swapTotal, swapUsed, err := parseSwap(swapOut)
	if err != nil {
		h.Error = fmt.Sprintf("parse swap: %v", err)
		return h
	}

	h.TotalMemory = totalMem
	h.AvailMemory = (free + inactive) * uint64(pageSize)
	h.SwapTotalMB = swapTotal
	h.SwapUsedMB = swapUsed
	h.ClaudeCount = parseClaudeCount(claudeOut)

	return h
}

func ProbeAll(ctx context.Context, machines []config.Machine) []Health {
	results := make([]Health, len(machines))
	ch := make(chan struct{ idx int; h Health }, len(machines))

	for i, m := range machines {
		go func(idx int, m config.Machine) {
			ch <- struct{ idx int; h Health }{idx, Probe(ctx, m)}
		}(i, m)
	}

	for range machines {
		r := <-ch
		results[r.idx] = r.h
	}
	return results
}

var pagesSizeRe = regexp.MustCompile(`page size of (\d+) bytes`)
var pagesFreeRe = regexp.MustCompile(`Pages free:\s+(\d+)`)
var pagesInactiveRe = regexp.MustCompile(`Pages inactive:\s+(\d+)`)

func parseVMStat(out string) (free, inactive uint64, pageSize int, err error) {
	m := pagesSizeRe.FindStringSubmatch(out)
	if m == nil {
		return 0, 0, 0, fmt.Errorf("page size not found")
	}
	pageSize, _ = strconv.Atoi(m[1])

	m = pagesFreeRe.FindStringSubmatch(out)
	if m == nil {
		return 0, 0, 0, fmt.Errorf("pages free not found")
	}
	free, _ = strconv.ParseUint(m[1], 10, 64)

	m = pagesInactiveRe.FindStringSubmatch(out)
	if m == nil {
		return 0, 0, 0, fmt.Errorf("pages inactive not found")
	}
	inactive, _ = strconv.ParseUint(m[1], 10, 64)

	return free, inactive, pageSize, nil
}

var swapRe = regexp.MustCompile(`total = ([\d.]+)M\s+used = ([\d.]+)M`)

func parseSwap(out string) (totalMB, usedMB float64, err error) {
	m := swapRe.FindStringSubmatch(out)
	if m == nil {
		return 0, 0, fmt.Errorf("swap info not found in: %q", out)
	}
	totalMB, _ = strconv.ParseFloat(m[1], 64)
	usedMB, _ = strconv.ParseFloat(m[2], 64)
	return totalMB, usedMB, nil
}

func parseMemsize(out string) (uint64, error) {
	s := strings.TrimSpace(out)
	return strconv.ParseUint(s, 10, 64)
}

func parseClaudeCount(out string) int {
	count := 0
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "grep") {
			continue
		}
		if strings.Contains(line, "claude") {
			count++
		}
	}
	return count
}
```

- [ ] **Step 4: Run tests**

Run: `go test -race ./internal/machine/ -v`
Expected: All 5 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/machine/probe.go internal/machine/probe_test.go
git commit -m "feat: add machine health probing via SSH"
```

---

### Task 5: Machine Scoring and Selection

**Files:**
- Create: `internal/machine/score.go`
- Create: `internal/machine/score_test.go`

- [ ] **Step 1: Write scoring tests**

Create `internal/machine/score_test.go`:

```go
package machine

import (
	"testing"
)

func TestScore(t *testing.T) {
	tests := []struct {
		name     string
		health   Health
		wantSign string // "positive" or "negative" or "zero"
	}{
		{
			name: "healthy machine",
			health: Health{
				Online:      true,
				TotalMemory: 16 * 1024 * 1024 * 1024, // 16GB
				AvailMemory: 12 * 1024 * 1024 * 1024, // 12GB free
				SwapTotalMB: 4096,
				SwapUsedMB:  0,
				ClaudeCount: 0,
			},
			wantSign: "positive",
		},
		{
			name: "stressed machine",
			health: Health{
				Online:      true,
				TotalMemory: 16 * 1024 * 1024 * 1024,
				AvailMemory: 1 * 1024 * 1024 * 1024, // 1GB free
				SwapTotalMB: 7168,
				SwapUsedMB:  6614,
				ClaudeCount: 5,
			},
			wantSign: "negative",
		},
		{
			name: "offline machine",
			health: Health{
				Online: false,
			},
			wantSign: "negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := Score(tt.health)
			switch tt.wantSign {
			case "positive":
				if score <= 0 {
					t.Errorf("Score() = %f, want positive", score)
				}
			case "negative":
				if score >= 0 {
					t.Errorf("Score() = %f, want negative", score)
				}
			}
		})
	}
}

func TestPickBest(t *testing.T) {
	healths := []Health{
		{Name: "mm1", Online: true, TotalMemory: 16e9, AvailMemory: 4e9, SwapTotalMB: 4096, SwapUsedMB: 2000, ClaudeCount: 3},
		{Name: "mm2", Online: true, TotalMemory: 16e9, AvailMemory: 12e9, SwapTotalMB: 4096, SwapUsedMB: 100, ClaudeCount: 0},
		{Name: "mm3", Online: false},
	}

	best, score := PickBest(healths)
	if best.Name != "mm2" {
		t.Errorf("PickBest() = %q, want mm2", best.Name)
	}
	if score <= 0 {
		t.Errorf("score = %f, want positive", score)
	}
}

func TestPickBestAllOffline(t *testing.T) {
	healths := []Health{
		{Name: "mm1", Online: false},
		{Name: "mm2", Online: false},
	}

	_, score := PickBest(healths)
	if score > -999 {
		t.Errorf("score = %f, want <= -999 (all offline)", score)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/machine/ -run TestScore -v`
Expected: Compilation error.

- [ ] **Step 3: Implement scoring**

Create `internal/machine/score.go`:

```go
package machine

const offlineScore = -1000.0

func Score(h Health) float64 {
	if !h.Online {
		return offlineScore
	}

	if h.TotalMemory == 0 {
		return offlineScore
	}

	availPct := float64(h.AvailMemory) / float64(h.TotalMemory) * 100

	var swapPenalty float64
	if h.SwapTotalMB > 0 {
		swapPenalty = (h.SwapUsedMB / h.SwapTotalMB) * 100 * 0.5
	}

	claudePenalty := float64(h.ClaudeCount) * 10

	return availPct - swapPenalty - claudePenalty
}

func PickBest(healths []Health) (Health, float64) {
	bestScore := offlineScore - 1
	bestIdx := 0

	for i, h := range healths {
		s := Score(h)
		if s > bestScore {
			bestScore = s
			bestIdx = i
		}
	}

	return healths[bestIdx], bestScore
}
```

- [ ] **Step 4: Run tests**

Run: `go test -race ./internal/machine/ -v`
Expected: All tests pass (probe + score tests).

- [ ] **Step 5: Commit**

```bash
git add internal/machine/score.go internal/machine/score_test.go
git commit -m "feat: add machine scoring and selection logic"
```

---

### Task 6: State Management

**Files:**
- Create: `internal/session/state.go`
- Create: `internal/session/state_test.go`

- [ ] **Step 1: Write state tests**

Create `internal/session/state_test.go`:

```go
package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := &State{
		Sessions: []Session{
			{
				ID:           "abc123",
				Project:      "neonwatty/seatify",
				Machine:      "mm2",
				Branch:       "main",
				WorktreePath: "/Users/jeremywatt/fleet-work/seatify-1234",
				Tunnel:       TunnelInfo{LocalPort: 4001, RemotePort: 3000},
				StartedAt:    time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC),
				PID:          12345,
			},
		},
	}

	if err := Save(path, s); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}

	if len(loaded.Sessions) != 1 {
		t.Fatalf("len(Sessions) = %d, want 1", len(loaded.Sessions))
	}
	if loaded.Sessions[0].ID != "abc123" {
		t.Errorf("ID = %q, want %q", loaded.Sessions[0].ID, "abc123")
	}
	if loaded.Sessions[0].Tunnel.LocalPort != 4001 {
		t.Errorf("LocalPort = %d, want 4001", loaded.Sessions[0].Tunnel.LocalPort)
	}
}

func TestLoadStateMissingFile(t *testing.T) {
	s, err := LoadState("/nonexistent/state.json")
	if err != nil {
		t.Fatalf("LoadState() should return empty state, got error: %v", err)
	}
	if len(s.Sessions) != 0 {
		t.Errorf("expected empty sessions for missing file")
	}
}

func TestAddAndRemoveSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	sess := Session{
		ID:      "test1",
		Project: "org/repo",
		Machine: "mm1",
	}

	if err := AddSession(path, sess); err != nil {
		t.Fatalf("AddSession() error: %v", err)
	}

	s, _ := LoadState(path)
	if len(s.Sessions) != 1 {
		t.Fatalf("expected 1 session after add")
	}

	if err := RemoveSession(path, "test1"); err != nil {
		t.Fatalf("RemoveSession() error: %v", err)
	}

	s, _ = LoadState(path)
	if len(s.Sessions) != 0 {
		t.Errorf("expected 0 sessions after remove, got %d", len(s.Sessions))
	}
}

func TestUsedPorts(t *testing.T) {
	s := &State{
		Sessions: []Session{
			{ID: "a", Tunnel: TunnelInfo{LocalPort: 4001}},
			{ID: "b", Tunnel: TunnelInfo{LocalPort: 4005}},
		},
	}

	ports := s.UsedPorts()
	if len(ports) != 2 {
		t.Fatalf("len(UsedPorts) = %d, want 2", len(ports))
	}
	if !ports[4001] || !ports[4005] {
		t.Errorf("expected ports 4001 and 4005 in set")
	}
}

func TestDefaultPath(t *testing.T) {
	path := DefaultStatePath()
	if !filepath.IsAbs(path) {
		t.Errorf("DefaultStatePath() = %q, want absolute path", path)
	}
	if !contains(path, ".fleet") {
		t.Errorf("DefaultStatePath() = %q, want to contain .fleet", path)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && containsAt(s, sub)
}

func containsAt(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestGenerateID(t *testing.T) {
	id := GenerateID()
	if len(id) != 8 {
		t.Errorf("GenerateID() len = %d, want 8", len(id))
	}

	// Should be unique
	id2 := GenerateID()
	if id == id2 {
		t.Error("two GenerateID() calls returned same value")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/session/ -v`
Expected: Compilation error.

- [ ] **Step 3: Implement state management**

Create `internal/session/state.go`:

```go
package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	Sessions []Session `json:"sessions"`
}

type Session struct {
	ID           string     `json:"id"`
	Project      string     `json:"project"`
	Machine      string     `json:"machine"`
	Branch       string     `json:"branch"`
	WorktreePath string     `json:"worktree_path"`
	Tunnel       TunnelInfo `json:"tunnel"`
	StartedAt    time.Time  `json:"started_at"`
	PID          int        `json:"pid"`
}

type TunnelInfo struct {
	LocalPort  int `json:"local_port"`
	RemotePort int `json:"remote_port"`
}

func DefaultStatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".fleet", "state.json")
}

func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{}, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &s, nil
}

func Save(path string, s *State) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func AddSession(path string, sess Session) error {
	s, err := LoadState(path)
	if err != nil {
		return err
	}
	s.Sessions = append(s.Sessions, sess)
	return Save(path, s)
}

func RemoveSession(path string, id string) error {
	s, err := LoadState(path)
	if err != nil {
		return err
	}

	filtered := make([]Session, 0, len(s.Sessions))
	for _, sess := range s.Sessions {
		if sess.ID != id {
			filtered = append(filtered, sess)
		}
	}
	s.Sessions = filtered
	return Save(path, s)
}

func (s *State) UsedPorts() map[int]bool {
	ports := make(map[int]bool)
	for _, sess := range s.Sessions {
		if sess.Tunnel.LocalPort > 0 {
			ports[sess.Tunnel.LocalPort] = true
		}
	}
	return ports
}

func GenerateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 4: Run tests**

Run: `go test -race ./internal/session/ -v`
Expected: All 6 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/session/state.go internal/session/state_test.go
git commit -m "feat: add session state management (read/write/add/remove)"
```

---

### Task 7: Tunnel Management and Port Detection

**Files:**
- Create: `internal/tunnel/tunnel.go`
- Create: `internal/tunnel/tunnel_test.go`
- Create: `internal/tunnel/portdetect.go`
- Create: `internal/tunnel/portdetect_test.go`

- [ ] **Step 1: Write port allocation tests**

Create `internal/tunnel/tunnel_test.go`:

```go
package tunnel

import (
	"testing"
)

func TestAllocatePort(t *testing.T) {
	used := map[int]bool{4000: true, 4001: true}
	port, err := AllocatePort(4000, 4999, used)
	if err != nil {
		t.Fatalf("AllocatePort() error: %v", err)
	}
	if port != 4002 {
		t.Errorf("AllocatePort() = %d, want 4002", port)
	}
}

func TestAllocatePortPinned(t *testing.T) {
	used := map[int]bool{}
	port, err := AllocatePortPinned(3000, used)
	if err != nil {
		t.Fatalf("AllocatePortPinned() error: %v", err)
	}
	if port != 3000 {
		t.Errorf("AllocatePortPinned() = %d, want 3000", port)
	}
}

func TestAllocatePortPinnedConflict(t *testing.T) {
	used := map[int]bool{3000: true}
	_, err := AllocatePortPinned(3000, used)
	if err == nil {
		t.Error("expected error for pinned port conflict")
	}
}

func TestAllocatePortExhausted(t *testing.T) {
	used := map[int]bool{4000: true, 4001: true, 4002: true}
	_, err := AllocatePort(4000, 4002, used)
	if err == nil {
		t.Error("expected error when all ports used")
	}
}
```

- [ ] **Step 2: Write port detection tests**

Create `internal/tunnel/portdetect_test.go`:

```go
package tunnel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPortFromFleetToml(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".fleet.toml"), []byte(`
dev_port = 3001
tunnel_local_port = 3001
`), 0644)

	devPort, localPort := DetectPorts(dir)
	if devPort != 3001 {
		t.Errorf("devPort = %d, want 3001", devPort)
	}
	if localPort != 3001 {
		t.Errorf("localPort = %d, want 3001 (pinned)", localPort)
	}
}

func TestDetectPortFromFleetTomlNoPinning(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".fleet.toml"), []byte(`
dev_port = 5173
`), 0644)

	devPort, localPort := DetectPorts(dir)
	if devPort != 5173 {
		t.Errorf("devPort = %d, want 5173", devPort)
	}
	if localPort != 0 {
		t.Errorf("localPort = %d, want 0 (no pin)", localPort)
	}
}

func TestDetectPortFromPackageJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "scripts": {
    "dev": "next dev -p 3002"
  }
}`), 0644)

	devPort, localPort := DetectPorts(dir)
	if devPort != 3002 {
		t.Errorf("devPort = %d, want 3002", devPort)
	}
	if localPort != 0 {
		t.Errorf("localPort = %d, want 0", localPort)
	}
}

func TestDetectPortFromPackageJSONVite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "scripts": {
    "dev": "vite --port 5173"
  }
}`), 0644)

	devPort, _ := DetectPorts(dir)
	if devPort != 5173 {
		t.Errorf("devPort = %d, want 5173", devPort)
	}
}

func TestDetectPortFallback(t *testing.T) {
	dir := t.TempDir()
	devPort, _ := DetectPorts(dir)
	if devPort != 3000 {
		t.Errorf("devPort = %d, want 3000 (fallback)", devPort)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/tunnel/ -v`
Expected: Compilation error.

- [ ] **Step 4: Implement port detection**

Create `internal/tunnel/portdetect.go`:

```go
package tunnel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/BurntSushi/toml"
)

type fleetToml struct {
	DevPort        int `toml:"dev_port"`
	TunnelLocalPort int `toml:"tunnel_local_port"`
}

type packageJSON struct {
	Scripts map[string]string `json:"scripts"`
}

var portFlagRe = regexp.MustCompile(`(?:-p|--port)\s+(\d+)`)

func DetectPorts(worktreePath string) (devPort int, pinnedLocalPort int) {
	// 1. Check .fleet.toml
	ft := filepath.Join(worktreePath, ".fleet.toml")
	if data, err := os.ReadFile(ft); err == nil {
		var cfg fleetToml
		if toml.Unmarshal(data, &cfg) == nil {
			if cfg.DevPort > 0 {
				return cfg.DevPort, cfg.TunnelLocalPort
			}
		}
	}

	// 2. Check package.json
	pj := filepath.Join(worktreePath, "package.json")
	if data, err := os.ReadFile(pj); err == nil {
		var pkg packageJSON
		if json.Unmarshal(data, &pkg) == nil {
			if devScript, ok := pkg.Scripts["dev"]; ok {
				if m := portFlagRe.FindStringSubmatch(devScript); m != nil {
					port, _ := strconv.Atoi(m[1])
					if port > 0 {
						return port, 0
					}
				}
			}
		}
	}

	// 3. Fallback
	return 3000, 0
}
```

- [ ] **Step 5: Implement tunnel management**

Create `internal/tunnel/tunnel.go`:

```go
package tunnel

import (
	"fmt"
	"os/exec"

	"github.com/neonwatty/fleet/internal/config"
)

func AllocatePort(rangeStart, rangeEnd int, used map[int]bool) (int, error) {
	for p := rangeStart; p <= rangeEnd; p++ {
		if !used[p] {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", rangeStart, rangeEnd)
}

func AllocatePortPinned(port int, used map[int]bool) (int, error) {
	if used[port] {
		return 0, fmt.Errorf("pinned port %d is already in use by another session", port)
	}
	return port, nil
}

type Tunnel struct {
	LocalPort  int
	RemotePort int
	Machine    config.Machine
	Cmd        *exec.Cmd
}

func Start(m config.Machine, localPort, remotePort int) (*Tunnel, error) {
	if m.IsLocal() {
		return &Tunnel{
			LocalPort:  remotePort,
			RemotePort: remotePort,
			Machine:    m,
		}, nil
	}

	arg := fmt.Sprintf("%d:localhost:%d", localPort, remotePort)
	cmd := exec.Command("ssh",
		"-N",
		"-L", arg,
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ConnectTimeout=5",
		m.Host,
	)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start tunnel: %w", err)
	}

	return &Tunnel{
		LocalPort:  localPort,
		RemotePort: remotePort,
		Machine:    m,
		Cmd:        cmd,
	}, nil
}

func (t *Tunnel) Stop() error {
	if t.Cmd == nil || t.Cmd.Process == nil {
		return nil
	}
	return t.Cmd.Process.Kill()
}
```

- [ ] **Step 6: Run all tunnel tests**

Run: `go test -race ./internal/tunnel/ -v`
Expected: All 9 tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/tunnel/
git commit -m "feat: add tunnel management and dev server port detection"
```

---

### Task 8: Session Launch and Teardown

**Files:**
- Create: `internal/session/launch.go`
- Create: `internal/session/teardown.go`

- [ ] **Step 1: Implement launch orchestration**

Create `internal/session/launch.go`:

```go
package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	fleetexec "github.com/neonwatty/fleet/internal/exec"
	"github.com/neonwatty/fleet/internal/tunnel"
)

type LaunchOpts struct {
	Project    string // "org/repo"
	Branch     string
	Machine    config.Machine
	Settings   config.Settings
	StatePath  string
}

type LaunchResult struct {
	Session Session
	Tunnel  *tunnel.Tunnel
}

func Launch(ctx context.Context, opts LaunchOpts) (*LaunchResult, error) {
	if opts.Branch == "" {
		opts.Branch = "main"
	}

	org, repo := splitProject(opts.Project)
	bareDir := filepath.Join(opts.Settings.BareRepoBase, org, repo+".git")
	timestamp := time.Now().Unix()
	worktreeDir := filepath.Join(opts.Settings.WorktreeBase, fmt.Sprintf("%s-%d", repo, timestamp))

	// Expand paths for remote machine
	remoteBare := expandRemotePath(bareDir, opts.Machine)
	remoteWork := expandRemotePath(worktreeDir, opts.Machine)

	// Step 1: Ensure bare clone exists
	checkCmd := fmt.Sprintf("test -d %s", remoteBare)
	if _, err := fleetexec.Run(ctx, opts.Machine, checkCmd); err != nil {
		cloneURL := fmt.Sprintf("https://github.com/%s.git", opts.Project)
		mkdirCmd := fmt.Sprintf("mkdir -p %s && git clone --bare %s %s",
			filepath.Dir(remoteBare), cloneURL, remoteBare)
		if _, err := fleetexec.Run(ctx, opts.Machine, mkdirCmd); err != nil {
			return nil, fmt.Errorf("bare clone: %w", err)
		}
	}

	// Step 2: Fetch latest
	fetchCmd := fmt.Sprintf("git -C %s fetch origin", remoteBare)
	if _, err := fleetexec.Run(ctx, opts.Machine, fetchCmd); err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	// Step 3: Create worktree
	worktreeCmd := fmt.Sprintf("git -C %s worktree add %s origin/%s",
		remoteBare, remoteWork, opts.Branch)
	if _, err := fleetexec.Run(ctx, opts.Machine, worktreeCmd); err != nil {
		return nil, fmt.Errorf("worktree: %w", err)
	}

	// Step 4: Detect dev server port
	devPort, pinnedLocal := detectRemotePorts(ctx, opts.Machine, remoteWork)

	// Step 5: Set up tunnel
	state, err := LoadState(opts.StatePath)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}
	usedPorts := state.UsedPorts()

	var localPort int
	if pinnedLocal > 0 {
		localPort, err = tunnel.AllocatePortPinned(pinnedLocal, usedPorts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v. Auto-assigning port.\n", err)
			localPort, err = tunnel.AllocatePort(
				opts.Settings.PortRange[0], opts.Settings.PortRange[1], usedPorts)
		}
	} else if !opts.Machine.IsLocal() {
		localPort, err = tunnel.AllocatePort(
			opts.Settings.PortRange[0], opts.Settings.PortRange[1], usedPorts)
	}
	if err != nil {
		return nil, fmt.Errorf("allocate port: %w", err)
	}

	var tun *tunnel.Tunnel
	if !opts.Machine.IsLocal() && localPort > 0 {
		tun, err = tunnel.Start(opts.Machine, localPort, devPort)
		if err != nil {
			return nil, fmt.Errorf("tunnel: %w", err)
		}
	}

	// Step 6: Record session
	sess := Session{
		ID:           GenerateID(),
		Project:      opts.Project,
		Machine:      opts.Machine.Name,
		Branch:       opts.Branch,
		WorktreePath: remoteWork,
		Tunnel:       TunnelInfo{LocalPort: localPort, RemotePort: devPort},
		StartedAt:    time.Now().UTC(),
		PID:          os.Getpid(),
	}

	if err := AddSession(opts.StatePath, sess); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	return &LaunchResult{Session: sess, Tunnel: tun}, nil
}

func ExecClaude(m config.Machine, worktreePath string) error {
	if m.IsLocal() {
		cmd := exec.Command("claude")
		cmd.Dir = worktreePath
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	sshCmd := fmt.Sprintf("cd %s && claude", worktreePath)
	cmd := exec.Command("ssh", "-t", m.Host, sshCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func splitProject(project string) (org, repo string) {
	parts := strings.SplitN(project, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}

func expandRemotePath(path string, m config.Machine) string {
	if m.IsLocal() {
		return config.ExpandPath(path)
	}
	if strings.HasPrefix(path, "~/") {
		return path // SSH expands ~ on the remote side
	}
	return path
}

func detectRemotePorts(ctx context.Context, m config.Machine, worktree string) (int, int) {
	// Try to read .fleet.toml from remote worktree
	catCmd := fmt.Sprintf("cat %s/.fleet.toml 2>/dev/null || true", worktree)
	fleetToml, _ := fleetexec.Run(ctx, m, catCmd)

	if strings.Contains(fleetToml, "dev_port") {
		// Write to temp dir and use DetectPorts
		tmpDir, err := os.MkdirTemp("", "fleet-detect-*")
		if err != nil {
			return 3000, 0
		}
		defer os.RemoveAll(tmpDir)
		os.WriteFile(filepath.Join(tmpDir, ".fleet.toml"), []byte(fleetToml), 0644)
		return tunnel.DetectPorts(tmpDir)
	}

	// Try package.json
	catCmd = fmt.Sprintf("cat %s/package.json 2>/dev/null || true", worktree)
	pkgJSON, _ := fleetexec.Run(ctx, m, catCmd)
	if strings.Contains(pkgJSON, "scripts") {
		tmpDir, err := os.MkdirTemp("", "fleet-detect-*")
		if err != nil {
			return 3000, 0
		}
		defer os.RemoveAll(tmpDir)
		os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644)
		return tunnel.DetectPorts(tmpDir)
	}

	return 3000, 0
}
```

- [ ] **Step 2: Implement teardown with signal handling**

Create `internal/session/teardown.go`:

```go
package session

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/neonwatty/fleet/internal/config"
	fleetexec "github.com/neonwatty/fleet/internal/exec"
	"github.com/neonwatty/fleet/internal/tunnel"
)

func Teardown(ctx context.Context, m config.Machine, sess Session, tun *tunnel.Tunnel, statePath string) {
	// 1. Stop tunnel
	if tun != nil {
		tun.Stop()
	}

	// 2. Remove remote worktree
	if sess.WorktreePath != "" {
		rmCmd := fmt.Sprintf("rm -rf %s", sess.WorktreePath)
		fleetexec.Run(ctx, m, rmCmd)

		// Prune worktrees on the bare repo
		org, repo := splitProject(sess.Project)
		home := "~"
		bareDir := filepath.Join(home, "fleet-repos", org, repo+".git")
		pruneCmd := fmt.Sprintf("git -C %s worktree prune", bareDir)
		fleetexec.Run(ctx, m, pruneCmd)
	}

	// 3. Remove session from state
	RemoveSession(statePath, sess.ID)
}

func WithSignalCleanup(
	ctx context.Context,
	m config.Machine,
	sess Session,
	tun *tunnel.Tunnel,
	statePath string,
	fn func() error,
) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- fn()
	}()

	select {
	case sig := <-sigCh:
		fmt.Fprintf(os.Stderr, "\nReceived %v, cleaning up...\n", sig)
		Teardown(ctx, m, sess, tun, statePath)
		return fmt.Errorf("interrupted by %v", sig)
	case err := <-doneCh:
		Teardown(ctx, m, sess, tun, statePath)
		return err
	}
}
```

- [ ] **Step 3: Run tests to verify compilation**

Run: `go build ./internal/session/`
Expected: Compiles without error.

- [ ] **Step 4: Commit**

```bash
git add internal/session/launch.go internal/session/teardown.go
git commit -m "feat: add session launch orchestration and signal-safe teardown"
```

---

### Task 9: Fleet Clean Command

**Files:**
- Create: `internal/session/clean.go`
- Create: `internal/session/clean_test.go`

- [ ] **Step 1: Write clean reconciliation tests**

Create `internal/session/clean_test.go`:

```go
package session

import (
	"testing"
)

func TestClassifySessions(t *testing.T) {
	sessions := []Session{
		{ID: "alive", Machine: "mm1", WorktreePath: "/exists"},
		{ID: "orphan", Machine: "mm1", WorktreePath: "/exists-no-proc"},
		{ID: "stale", Machine: "mm1", WorktreePath: "/gone"},
	}

	// Simulate remote checks
	checker := func(sess Session) SessionStatus {
		switch sess.ID {
		case "alive":
			return StatusAlive
		case "orphan":
			return StatusOrphan
		case "stale":
			return StatusStale
		default:
			return StatusStale
		}
	}

	alive, orphan, stale := ClassifySessions(sessions, checker)
	if len(alive) != 1 || alive[0].ID != "alive" {
		t.Errorf("alive = %v, want [alive]", ids(alive))
	}
	if len(orphan) != 1 || orphan[0].ID != "orphan" {
		t.Errorf("orphan = %v, want [orphan]", ids(orphan))
	}
	if len(stale) != 1 || stale[0].ID != "stale" {
		t.Errorf("stale = %v, want [stale]", ids(stale))
	}
}

func ids(sessions []Session) []string {
	out := make([]string, len(sessions))
	for i, s := range sessions {
		out[i] = s.ID
	}
	return out
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/session/ -run TestClassify -v`
Expected: Compilation error.

- [ ] **Step 3: Implement clean logic**

Create `internal/session/clean.go`:

```go
package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neonwatty/fleet/internal/config"
	fleetexec "github.com/neonwatty/fleet/internal/exec"
)

type SessionStatus int

const (
	StatusAlive  SessionStatus = iota
	StatusOrphan               // worktree exists but no claude process
	StatusStale                // nothing on disk
)

type StatusChecker func(Session) SessionStatus

func ClassifySessions(sessions []Session, check StatusChecker) (alive, orphan, stale []Session) {
	for _, s := range sessions {
		switch check(s) {
		case StatusAlive:
			alive = append(alive, s)
		case StatusOrphan:
			orphan = append(orphan, s)
		case StatusStale:
			stale = append(stale, s)
		}
	}
	return
}

func MakeRemoteChecker(ctx context.Context, machines []config.Machine) StatusChecker {
	machineMap := make(map[string]config.Machine)
	for _, m := range machines {
		machineMap[m.Name] = m
	}

	return func(sess Session) SessionStatus {
		m, ok := machineMap[sess.Machine]
		if !ok {
			return StatusStale
		}

		// Check if worktree exists
		checkDir := fmt.Sprintf("test -d %s", sess.WorktreePath)
		if _, err := fleetexec.Run(ctx, m, checkDir); err != nil {
			return StatusStale
		}

		// Check if claude process is running in that worktree
		checkProc := fmt.Sprintf("ps aux | grep '[c]laude' | grep -q %s",
			filepath.Base(sess.WorktreePath))
		if _, err := fleetexec.Run(ctx, m, checkProc); err != nil {
			return StatusOrphan
		}

		return StatusAlive
	}
}

func Clean(ctx context.Context, cfg *config.Config, statePath string) error {
	state, err := LoadState(statePath)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	if len(state.Sessions) == 0 {
		fmt.Println("No sessions in state. Nothing to clean.")
		return nil
	}

	checker := MakeRemoteChecker(ctx, cfg.Machines)
	alive, orphans, stales := ClassifySessions(state.Sessions, checker)

	fmt.Printf("Sessions: %d alive, %d orphaned, %d stale\n",
		len(alive), len(orphans), len(stales))

	machineMap := make(map[string]config.Machine)
	for _, m := range cfg.Machines {
		machineMap[m.Name] = m
	}

	// Clean orphaned worktrees
	for _, sess := range orphans {
		m := machineMap[sess.Machine]
		fmt.Printf("  Cleaning orphan: %s on %s (%s)\n", sess.Project, sess.Machine, sess.WorktreePath)

		rmCmd := fmt.Sprintf("rm -rf %s", sess.WorktreePath)
		fleetexec.Run(ctx, m, rmCmd)

		org, repo := splitProject(sess.Project)
		bareDir := filepath.Join("~", "fleet-repos", org, repo+".git")
		pruneCmd := fmt.Sprintf("git -C %s worktree prune 2>/dev/null || true", bareDir)
		fleetexec.Run(ctx, m, pruneCmd)
	}

	// Update state to only keep alive sessions
	state.Sessions = alive
	if err := Save(statePath, state); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	cleaned := len(orphans) + len(stales)
	fmt.Printf("Cleaned %d sessions.\n", cleaned)

	// Kill orphaned tunnel processes
	killOrphanTunnels(alive)

	return nil
}

func killOrphanTunnels(aliveSessions []Session) {
	aliveLocalPorts := make(map[int]bool)
	for _, s := range aliveSessions {
		if s.Tunnel.LocalPort > 0 {
			aliveLocalPorts[s.Tunnel.LocalPort] = true
		}
	}

	// List local ssh tunnel processes
	out, err := fleetexec.Run(context.Background(),
		config.Machine{Host: "localhost"},
		"ps aux | grep 'ssh -N -L' | grep -v grep || true")
	if err != nil {
		return
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Check if this tunnel's port is still needed
		isNeeded := false
		for port := range aliveLocalPorts {
			if strings.Contains(line, fmt.Sprintf("%d:localhost:", port)) {
				isNeeded = true
				break
			}
		}
		if !isNeeded {
			fmt.Printf("  Found orphaned tunnel process: %s\n", truncate(line, 80))
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
```

- [ ] **Step 4: Run tests**

Run: `go test -race ./internal/session/ -v`
Expected: All tests pass (state + clean tests).

- [ ] **Step 5: Commit**

```bash
git add internal/session/clean.go internal/session/clean_test.go
git commit -m "feat: add fleet clean command for state reconciliation"
```

---

### Task 10: CLI Wiring with Cobra

**Files:**
- Create: `cmd/fleet/main.go`

- [ ] **Step 1: Install cobra dependency**

Run: `go get github.com/spf13/cobra`

- [ ] **Step 2: Implement CLI entry point**

Create `cmd/fleet/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "fleet",
		Short:   "Distribute Claude Code instances across your local Mac fleet",
		Version: version,
	}

	root.AddCommand(launchCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(cleanCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func launchCmd() *cobra.Command {
	var branch string
	var target string

	cmd := &cobra.Command{
		Use:   "launch <org/repo>",
		Short: "Launch Claude Code on the best available machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			ctx := context.Background()

			cfg, err := config.Load(config.DefaultPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			enabled := cfg.EnabledMachines()
			if len(enabled) == 0 {
				return fmt.Errorf("no enabled machines in config")
			}

			var chosen config.Machine

			if target != "" {
				found := false
				for _, m := range enabled {
					if m.Name == target {
						chosen = m
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("machine %q not found or not enabled", target)
				}
				fmt.Printf("Using specified target: %s\n", chosen.Name)
			} else {
				fmt.Println("Probing machines...")
				healths := machine.ProbeAll(ctx, enabled)

				for _, h := range healths {
					if h.Online {
						availPct := float64(h.AvailMemory) / float64(h.TotalMemory) * 100
						fmt.Printf("  %s: %.0f%% mem avail, %.0fMB swap, %d claude instances (score: %.1f)\n",
							h.Name, availPct, h.SwapUsedMB, h.ClaudeCount, machine.Score(h))
					} else {
						fmt.Printf("  %s: offline\n", h.Name)
					}
				}

				best, score := machine.PickBest(healths)
				if !best.Online {
					return fmt.Errorf("no machines are reachable")
				}

				if score < float64(cfg.Settings.StressThreshold) {
					fmt.Printf("\nAll machines are stressed. Best: %s (score: %.1f)\n", best.Name, score)
					fmt.Print("Launch anyway? [y/n]: ")
					var answer string
					fmt.Scanln(&answer)
					if answer != "y" && answer != "Y" {
						fmt.Println("Aborted.")
						return nil
					}
				}

				chosen = findMachine(enabled, best.Name)
				fmt.Printf("\nSelected: %s (score: %.1f)\n", chosen.Name, score)
			}

			fmt.Printf("Setting up %s on %s...\n", project, chosen.Name)

			result, err := session.Launch(ctx, session.LaunchOpts{
				Project:  project,
				Branch:   branch,
				Machine:  chosen,
				Settings: cfg.Settings,
				StatePath: session.DefaultStatePath(),
			})
			if err != nil {
				return fmt.Errorf("launch: %w", err)
			}

			if result.Session.Tunnel.LocalPort > 0 && !chosen.IsLocal() {
				fmt.Printf("Tunnel: localhost:%d → %s:%d\n",
					result.Session.Tunnel.LocalPort, chosen.Name, result.Session.Tunnel.RemotePort)
			}

			fmt.Printf("Session %s started. Launching Claude Code...\n\n", result.Session.ID)

			return session.WithSignalCleanup(
				ctx, chosen, result.Session, result.Tunnel,
				session.DefaultStatePath(),
				func() error {
					return session.ExecClaude(chosen, result.Session.WorktreePath)
				},
			)
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "main", "Branch to check out")
	cmd.Flags().StringVarP(&target, "target", "t", "", "Force a specific machine")
	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show fleet dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TUI implementation in Task 11
			fmt.Println("TUI dashboard — not yet implemented")
			return nil
		},
	}
}

func cleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Clean up orphaned worktrees and stale sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.DefaultPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return session.Clean(context.Background(), cfg, session.DefaultStatePath())
		},
	}
}

func findMachine(machines []config.Machine, name string) config.Machine {
	for _, m := range machines {
		if m.Name == name {
			return m
		}
	}
	return machines[0]
}
```

- [ ] **Step 3: Build and verify**

Run: `go mod tidy && go build -o bin/fleet ./cmd/fleet/`
Expected: Binary compiles.

Run: `bin/fleet --version`
Expected: `fleet version dev`

Run: `bin/fleet launch --help`
Expected: Shows usage with `--branch` and `--target` flags.

- [ ] **Step 4: Commit**

```bash
git add cmd/ go.mod go.sum
git commit -m "feat: add cobra CLI with launch, status, and clean commands"
```

---

### Task 11: TUI Dashboard

**Files:**
- Create: `internal/tui/styles.go`
- Create: `internal/tui/machines.go`
- Create: `internal/tui/sessions.go`
- Create: `internal/tui/tunnels.go`
- Create: `internal/tui/actions.go`
- Create: `internal/tui/app.go`
- Modify: `cmd/fleet/main.go` (wire up status command)

- [ ] **Step 1: Install Bubble Tea dependencies**

Run:
```bash
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss
go get github.com/charmbracelet/bubbles
```

- [ ] **Step 2: Create styles**

Create `internal/tui/styles.go`:

```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			PaddingLeft(1)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("39")).
				Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("252"))

	onlineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	offlineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			PaddingLeft(1)
)
```

- [ ] **Step 3: Create machines panel**

Create `internal/tui/machines.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/neonwatty/fleet/internal/machine"
)

func renderMachinesPanel(healths []machine.Health, width int) string {
	var b strings.Builder

	header := fmt.Sprintf("%-10s %-8s %-10s %-10s %-8s %-6s",
		"MACHINE", "STATUS", "MEM AVAIL", "SWAP USED", "CLAUDE", "SCORE")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for _, h := range healths {
		if !h.Online {
			line := fmt.Sprintf("%-10s %-8s",
				h.Name, offlineStyle.Render("offline"))
			b.WriteString(line)
			b.WriteString("\n")
			continue
		}

		availPct := float64(h.AvailMemory) / float64(h.TotalMemory) * 100
		score := machine.Score(h)

		status := onlineStyle.Render("online")
		memStr := fmt.Sprintf("%.0f%%", availPct)
		swapStr := fmt.Sprintf("%.0fMB", h.SwapUsedMB)
		claudeStr := fmt.Sprintf("%d", h.ClaudeCount)
		scoreStr := fmt.Sprintf("%.1f", score)

		if availPct < 25 {
			memStr = warnStyle.Render(memStr)
		}
		if h.SwapUsedMB > 4000 {
			swapStr = warnStyle.Render(swapStr)
		}

		line := fmt.Sprintf("%-10s %-8s %-10s %-10s %-8s %-6s",
			h.Name, status, memStr, swapStr, claudeStr, scoreStr)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}
```

- [ ] **Step 4: Create sessions panel**

Create `internal/tui/sessions.go`:

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/neonwatty/fleet/internal/session"
)

func renderSessionsPanel(sessions []session.Session) string {
	if len(sessions) == 0 {
		return dimStyle.Render("No active sessions")
	}

	var b strings.Builder

	header := fmt.Sprintf("%-8s %-20s %-8s %-10s %-10s",
		"ID", "PROJECT", "MACHINE", "BRANCH", "UPTIME")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for _, s := range sessions {
		uptime := time.Since(s.StartedAt).Truncate(time.Second)
		line := fmt.Sprintf("%-8s %-20s %-8s %-10s %-10s",
			s.ID, truncateStr(s.Project, 20), s.Machine, s.Branch, uptime)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
```

- [ ] **Step 5: Create tunnels panel**

Create `internal/tui/tunnels.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/neonwatty/fleet/internal/session"
)

func renderTunnelsPanel(sessions []session.Session) string {
	tunneled := make([]session.Session, 0)
	for _, s := range sessions {
		if s.Tunnel.LocalPort > 0 {
			tunneled = append(tunneled, s)
		}
	}

	if len(tunneled) == 0 {
		return dimStyle.Render("No active tunnels")
	}

	var b strings.Builder

	header := fmt.Sprintf("%-22s %-12s %-20s",
		"LOCAL", "MACHINE", "PROJECT")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for _, s := range tunneled {
		local := fmt.Sprintf("localhost:%d", s.Tunnel.LocalPort)
		remote := fmt.Sprintf("→ %s:%d", s.Machine, s.Tunnel.RemotePort)
		line := fmt.Sprintf("%-22s %-12s %-20s",
			local+" "+remote, s.Machine, truncateStr(s.Project, 20))
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}
```

- [ ] **Step 6: Create actions panel**

Create `internal/tui/actions.go`:

```go
package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/session"
)

func killSession(ctx context.Context, cfg *config.Config, sess session.Session, statePath string) error {
	machineMap := make(map[string]config.Machine)
	for _, m := range cfg.Machines {
		machineMap[m.Name] = m
	}

	m, ok := machineMap[sess.Machine]
	if !ok {
		return fmt.Errorf("machine %q not found", sess.Machine)
	}

	session.Teardown(ctx, m, sess, nil, statePath)
	return nil
}

func openInBrowser(port int) error {
	url := fmt.Sprintf("http://localhost:%d", port)
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", url)
	} else {
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Run()
}
```

- [ ] **Step 7: Create main TUI app**

Create `internal/tui/app.go`:

```go
package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/neonwatty/fleet/internal/config"
	"github.com/neonwatty/fleet/internal/machine"
	"github.com/neonwatty/fleet/internal/session"
)

type panel int

const (
	panelMachines panel = iota
	panelSessions
	panelTunnels
	panelCount
)

type model struct {
	cfg          *config.Config
	statePath    string
	healths      []machine.Health
	state        *session.State
	activePanel  panel
	selectedRow  int
	width        int
	height       int
	pollInterval time.Duration
	err          error
}

type tickMsg time.Time
type refreshMsg struct {
	healths []machine.Health
	state   *session.State
}

func NewModel(cfg *config.Config, statePath string) model {
	return model{
		cfg:          cfg,
		statePath:    statePath,
		pollInterval: time.Duration(cfg.Settings.PollInterval) * time.Second,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(refresh(m.cfg, m.statePath), tick(m.pollInterval))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activePanel = (m.activePanel + 1) % panelCount
			m.selectedRow = 0
		case "shift+tab":
			m.activePanel = (m.activePanel - 1 + panelCount) % panelCount
			m.selectedRow = 0
		case "j", "down":
			m.selectedRow++
		case "k", "up":
			if m.selectedRow > 0 {
				m.selectedRow--
			}
		case "o":
			if m.activePanel == panelTunnels && m.state != nil {
				tunneled := tunneledSessions(m.state.Sessions)
				if m.selectedRow < len(tunneled) {
					openInBrowser(tunneled[m.selectedRow].Tunnel.LocalPort)
				}
			}
		case "x":
			if m.activePanel == panelSessions && m.state != nil {
				if m.selectedRow < len(m.state.Sessions) {
					sess := m.state.Sessions[m.selectedRow]
					killSession(context.Background(), m.cfg, sess, m.statePath)
					return m, refresh(m.cfg, m.statePath)
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		return m, tea.Batch(refresh(m.cfg, m.statePath), tick(m.pollInterval))
	case refreshMsg:
		m.healths = msg.healths
		m.state = msg.state
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	title := titleStyle.Render("Fleet Dashboard")
	panelWidth := m.width - 4

	// Machines
	machinesContent := renderMachinesPanel(m.healths, panelWidth)
	machinesPanel := wrapPanel("Machines", machinesContent, panelWidth, m.activePanel == panelMachines)

	// Sessions
	var sessions []session.Session
	if m.state != nil {
		sessions = m.state.Sessions
	}
	sessionsContent := renderSessionsPanel(sessions)
	sessionsPanel := wrapPanel("Sessions", sessionsContent, panelWidth, m.activePanel == panelSessions)

	// Tunnels
	tunnelsContent := renderTunnelsPanel(sessions)
	tunnelsPanel := wrapPanel("Tunnels", tunnelsContent, panelWidth, m.activePanel == panelTunnels)

	help := helpStyle.Render("tab: switch panel | j/k: navigate | o: open in browser | x: kill session | q: quit")

	return fmt.Sprintf("%s\n\n%s\n%s\n%s\n\n%s",
		title, machinesPanel, sessionsPanel, tunnelsPanel, help)
}

func wrapPanel(title, content string, width int, active bool) string {
	style := panelStyle
	if active {
		style = activePanelStyle
	}

	header := titleStyle.Render(title)
	inner := lipgloss.JoinVertical(lipgloss.Left, header, content)
	return style.Width(width).Render(inner)
}

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func refresh(cfg *config.Config, statePath string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		healths := machine.ProbeAll(ctx, cfg.EnabledMachines())
		state, _ := session.LoadState(statePath)

		return refreshMsg{healths: healths, state: state}
	}
}

func tunneledSessions(sessions []session.Session) []session.Session {
	var out []session.Session
	for _, s := range sessions {
		if s.Tunnel.LocalPort > 0 {
			out = append(out, s)
		}
	}
	return out
}

func Run(cfg *config.Config, statePath string) error {
	p := tea.NewProgram(NewModel(cfg, statePath), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
```

- [ ] **Step 8: Wire up status command in main.go**

Modify `cmd/fleet/main.go` — replace the `statusCmd` function:

```go
func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show fleet dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.DefaultPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return tui.Run(cfg, session.DefaultStatePath())
		},
	}
}
```

Add import: `"github.com/neonwatty/fleet/internal/tui"`

- [ ] **Step 9: Build and verify**

Run: `go mod tidy && go build -o bin/fleet ./cmd/fleet/`
Expected: Compiles.

- [ ] **Step 10: Commit**

```bash
git add internal/tui/ cmd/fleet/main.go go.mod go.sum
git commit -m "feat: add Bubble Tea TUI dashboard with machines, sessions, tunnels panels"
```

---

### Task 12: Integration Test and Final Polish

**Files:**
- Modify: `cmd/fleet/main.go` (add version flag properly)
- Create: `CLAUDE.md`

- [ ] **Step 1: Manual integration test**

Set up config:
```bash
mkdir -p ~/.fleet
cp config.example.toml ~/.fleet/config.toml
```

Edit `~/.fleet/config.toml` to match your actual machine setup (mm1, mm2, mm3).

Run: `bin/fleet launch --help`
Expected: Shows usage with flags.

Run: `bin/fleet status`
Expected: TUI opens showing machine health for all configured machines. Press `q` to exit.

Run: `bin/fleet clean`
Expected: "No sessions in state. Nothing to clean."

- [ ] **Step 2: Create CLAUDE.md**

Create `CLAUDE.md`:

```markdown
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
```

- [ ] **Step 3: Run full check**

Run: `make check`
Expected: fmt, lint, vet, test, build all pass.

- [ ] **Step 4: Commit and push**

```bash
git add -A
git commit -m "chore: add CLAUDE.md and integration test setup"
git push origin main
```
