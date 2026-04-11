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
	DevPort         int `toml:"dev_port"`
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
