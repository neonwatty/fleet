package session

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/neonwatty/fleet/internal/config"
	fleetexec "github.com/neonwatty/fleet/internal/exec"
)

func killOrphanTunnels(aliveSessions []Session) {
	aliveLocalPorts := make(map[int]bool)
	for _, s := range aliveSessions {
		if s.Tunnel.LocalPort > 0 {
			aliveLocalPorts[s.Tunnel.LocalPort] = true
		}
	}

	out, err := fleetexec.RunWithTimeout(context.Background(),
		config.Machine{Host: "localhost"},
		"ps ax -o pid=,command= | grep 'ssh' | grep -- ' -L ' | grep -v grep || true",
		5*time.Second)
	if err != nil {
		return
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		port, ok := tunnelLocalPortFromPSLine(line)
		if !ok {
			continue
		}
		if !aliveLocalPorts[port] {
			fmt.Printf("  Found orphaned tunnel process: %s\n", truncateString(line, 80))
		}
	}
}

func tunnelLocalPortFromPSLine(line string) (int, bool) {
	fields := strings.Fields(line)
	for i, field := range fields {
		if field == "-L" && i+1 < len(fields) {
			return tunnelLocalPortFromForwardSpec(fields[i+1])
		}
		if strings.HasPrefix(field, "-L") && len(field) > len("-L") {
			return tunnelLocalPortFromForwardSpec(strings.TrimPrefix(field, "-L"))
		}
	}
	return 0, false
}

func tunnelLocalPortFromForwardSpec(spec string) (int, bool) {
	parts := strings.Split(spec, ":")
	if len(parts) < 3 {
		return 0, false
	}
	port, err := strconv.Atoi(parts[0])
	if err != nil || port <= 0 {
		return 0, false
	}
	return port, true
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
