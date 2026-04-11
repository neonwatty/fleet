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
	Name        string
	Online      bool
	TotalMemory uint64
	AvailMemory uint64
	SwapTotalMB float64
	SwapUsedMB  float64
	ClaudeCount int
	Error       string
}

const errUnexpectedFormat = "unexpected probe output format"

func Probe(ctx context.Context, m config.Machine) Health {
	h := Health{Name: m.Name}

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := "vm_stat; echo '===SWAP==='; sysctl vm.swapusage; " +
		"echo '===MEM==='; sysctl -n hw.memsize; " +
		"echo '===CLAUDE==='; ps -eo comm | grep -c '^claude$' || echo 0"
	out, err := fleetexec.Run(probeCtx, m, cmd)
	if err != nil {
		h.Error = err.Error()
		return h
	}

	h.Online = true

	parts := strings.Split(out, "===SWAP===")
	if len(parts) < 2 {
		h.Error = errUnexpectedFormat
		return h
	}
	vmstatOut := parts[0]

	rest := strings.Split(parts[1], "===MEM===")
	if len(rest) < 2 {
		h.Error = errUnexpectedFormat
		return h
	}
	swapOut := strings.TrimSpace(rest[0])

	rest2 := strings.Split(rest[1], "===CLAUDE===")
	if len(rest2) < 2 {
		h.Error = errUnexpectedFormat
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
	ch := make(chan struct {
		idx int
		h   Health
	}, len(machines))

	for i, m := range machines {
		go func(idx int, m config.Machine) {
			ch <- struct {
				idx int
				h   Health
			}{idx, Probe(ctx, m)}
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
	s := strings.TrimSpace(out)
	n, _ := strconv.Atoi(s)
	return n
}
