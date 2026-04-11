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
	if cfg.Settings.SwapScanProcs == 0 {
		cfg.Settings.SwapScanProcs = 10
	}

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
