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
	SwapScanProcs   int    `toml:"swap_scan_procs"`
	SwapWarnMB      int    `toml:"swap_warn_mb"`
	SwapHighMB      int    `toml:"swap_high_mb"`
}

type Machine struct {
	Name           string `toml:"name"`
	Host           string `toml:"host"`
	User           string `toml:"user"`
	Enabled        bool   `toml:"enabled"`
	DefaultAccount string `toml:"default_account"`
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
	if cfg.Settings.SwapScanProcs == 0 {
		cfg.Settings.SwapScanProcs = 10
	}
	if cfg.Settings.SwapWarnMB == 0 {
		cfg.Settings.SwapWarnMB = 1024
	}
	if cfg.Settings.SwapHighMB == 0 {
		cfg.Settings.SwapHighMB = 4096
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if err := validateSettings(c.Settings); err != nil {
		return err
	}
	return validateMachines(c.Machines)
}

func validateSettings(settings Settings) error {
	if settings.PortRange[0] <= 0 || settings.PortRange[1] <= 0 {
		return fmt.Errorf("settings.port_range must contain positive ports")
	}
	if settings.PortRange[0] > settings.PortRange[1] {
		return fmt.Errorf("settings.port_range start must be less than or equal to end")
	}
	if settings.PortRange[1] > 65535 {
		return fmt.Errorf("settings.port_range cannot exceed 65535")
	}
	if settings.PollInterval <= 0 {
		return fmt.Errorf("settings.poll_interval must be greater than 0")
	}
	if settings.WorktreeBase == "" {
		return fmt.Errorf("settings.worktree_base is required")
	}
	if settings.BareRepoBase == "" {
		return fmt.Errorf("settings.bare_repo_base is required")
	}
	if settings.SwapScanProcs <= 0 {
		return fmt.Errorf("settings.swap_scan_procs must be greater than 0")
	}
	if settings.SwapWarnMB < 0 || settings.SwapHighMB < 0 {
		return fmt.Errorf("swap thresholds cannot be negative")
	}
	if settings.SwapWarnMB > settings.SwapHighMB {
		return fmt.Errorf("settings.swap_warn_mb must be less than or equal to settings.swap_high_mb")
	}
	return nil
}

func validateMachines(machines []Machine) error {
	seen := make(map[string]bool, len(machines))
	for i, m := range machines {
		if strings.TrimSpace(m.Name) == "" {
			return fmt.Errorf("machines[%d].name is required", i)
		}
		if seen[m.Name] {
			return fmt.Errorf("duplicate machine name %q", m.Name)
		}
		seen[m.Name] = true
		if strings.TrimSpace(m.Host) == "" {
			return fmt.Errorf("machines[%d].host is required", i)
		}
	}
	return nil
}

const DefaultConfig = `# Fleet configuration
# Copy to ~/.fleet/config.toml and adjust for your setup.

[settings]
port_range = [4000, 4999]       # Local port range for tunnel auto-assignment
poll_interval = 5               # TUI refresh interval in seconds
stress_threshold = 20           # Score below this triggers confirmation prompt
worktree_base = "~/fleet-work"  # Remote directory for worktrees
bare_repo_base = "~/fleet-repos" # Remote directory for bare clones
swap_scan_procs = 10             # Max processes to scan for swap (via vmmap, ~1-2s each)
swap_warn_mb = 1024              # Swap threshold for amber warning in TUI + menu bar
swap_high_mb = 4096              # Swap threshold for red warning in TUI + menu bar

[[machines]]
name = "local"
host = "localhost"
user = ""
enabled = true
# default_account = "personal-max"

# [[machines]]
# name = "mm1"
# host = "mm1"        # Must match an SSH config Host entry
# user = "youruser"
# enabled = true
`

func WriteDefault(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config already exists at %s", path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat config: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(DefaultConfig), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
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

func (m Machine) SSHTarget() string {
	if strings.TrimSpace(m.User) == "" {
		return m.Host
	}
	return m.User + "@" + m.Host
}

func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
