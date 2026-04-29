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
	DevPort         int    `toml:"dev_port"`
	TunnelLocalPort int    `toml:"tunnel_local_port"`
	LaunchCommand   string `toml:"launch_command"`
}

type packageJSON struct {
	Scripts map[string]string `json:"scripts"`
}

var portFlagRe = regexp.MustCompile(`(?:-p|--port)\s+(\d+)`)
var portEnvRe = regexp.MustCompile(`(?:^|\s)PORT=(\d+)(?:\s|$)`)
var viteLikeRe = regexp.MustCompile(`(^|\s)(vite|astro)(\s|$)`)
var remixRe = regexp.MustCompile(`(^|\s)remix(\s|$)`)
var nextRe = regexp.MustCompile(`(^|\s)next(\s|$)`)

type ProjectConfig struct {
	DevPort         int
	TunnelLocalPort int
	LaunchCommand   string
}

func DetectPorts(worktreePath string) (devPort int, pinnedLocalPort int) {
	cfg := DetectProjectConfig(worktreePath)
	return cfg.DevPort, cfg.TunnelLocalPort
}

func DetectProjectConfig(worktreePath string) ProjectConfig {
	// 1. Check .fleet.toml
	ft := filepath.Join(worktreePath, ".fleet.toml")
	if data, err := os.ReadFile(ft); err == nil {
		var cfg fleetToml
		if toml.Unmarshal(data, &cfg) == nil {
			if cfg.DevPort > 0 || cfg.LaunchCommand != "" {
				return ProjectConfig{
					DevPort:         portOrDefault(cfg.DevPort),
					TunnelLocalPort: cfg.TunnelLocalPort,
					LaunchCommand:   cfg.LaunchCommand,
				}
			}
		}
	}

	// 2. Check package.json
	pj := filepath.Join(worktreePath, "package.json")
	if data, err := os.ReadFile(pj); err == nil {
		var pkg packageJSON
		if json.Unmarshal(data, &pkg) == nil {
			if devScript, ok := pkg.Scripts["dev"]; ok {
				if port := detectScriptPort(devScript); port > 0 {
					return ProjectConfig{DevPort: port}
				}
				if port := defaultScriptPort(devScript); port > 0 {
					return ProjectConfig{DevPort: port}
				}
			}
		}
	}

	// 3. Fallback
	return ProjectConfig{DevPort: 3000}
}

func portOrDefault(port int) int {
	if port > 0 {
		return port
	}
	return 3000
}

func detectScriptPort(script string) int {
	for _, re := range []*regexp.Regexp{portEnvRe, portFlagRe} {
		if m := re.FindStringSubmatch(script); m != nil {
			port, _ := strconv.Atoi(m[1])
			if port > 0 {
				return port
			}
		}
	}
	return 0
}

func defaultScriptPort(script string) int {
	switch {
	case viteLikeRe.MatchString(script):
		return 5173
	case remixRe.MatchString(script):
		return 3000
	case nextRe.MatchString(script):
		return 3000
	default:
		return 0
	}
}
