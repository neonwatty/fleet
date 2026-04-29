package machine

import (
	"context"

	"github.com/neonwatty/fleet/internal/config"
)

func ProbeProcessesAll(ctx context.Context, machines []config.Machine) map[string][]ProcessGroup {
	type result struct {
		name   string
		groups []ProcessGroup
	}

	ch := make(chan result, len(machines))
	for _, m := range machines {
		go func(m config.Machine) {
			ch <- result{name: m.Name, groups: ProbeProcesses(ctx, m)}
		}(m)
	}

	out := make(map[string][]ProcessGroup, len(machines))
	for range machines {
		r := <-ch
		out[r.name] = r.groups
	}
	return out
}
