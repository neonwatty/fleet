package machine

import (
	"testing"
)

const fixturePS = `631 19648 next-server (v15.5.14)
518 45760 claude --dangerously-skip-permissions
298 15083 /Applications/Google Chrome.app/Contents/Frameworks/Google Chrome Helper
185 15066 /Applications/Google Chrome.app/Contents/MacOS/Google Chrome
185 55964 /Applications/Docker.app/Contents/MacOS/com.docker.backend services
139 21077 /System/Library/PrivateFrameworks/MediaAnalysis.framework/Versions/A/mediaanalysisd
128 9406 /Applications/Google Chrome.app/Contents/MacOS/Google Chrome
98 16831 claude --dangerously-skip-permissions --resume
79 55983 /Applications/Docker.app/Contents/MacOS/com.docker.build
72 45850 node /Users/neonwatty/.npm/_npx/node_modules/.bin/playwright-mcp
67 849 /System/Library/Frameworks/CoreServices.framework/mds_stores
48 19642 node /Users/neonwatty/Desktop/issuectl/node_modules/.bin/../next/dist/bin/next dev
`

func TestParseProcesses(t *testing.T) {
	procs := parseProcesses(fixturePS)
	if len(procs) < 10 {
		t.Fatalf("parseProcesses() returned %d procs, want >= 10", len(procs))
	}
	if procs[0].RSSKB != 631 {
		t.Errorf("first proc RSS = %d KB, want 631", procs[0].RSSKB)
	}
}

func TestClassifyProcesses(t *testing.T) {
	procs := parseProcesses(fixturePS)
	groups := ClassifyProcesses(procs)

	claude := findGroup(groups, "Claude Code")
	if claude == nil {
		t.Fatal("expected Claude Code group")
	}
	if claude.Count != 2 {
		t.Errorf("Claude Code count = %d, want 2", claude.Count)
	}

	chrome := findGroup(groups, "Chrome")
	if chrome == nil {
		t.Fatal("expected Chrome group")
	}
	if chrome.Count != 3 {
		t.Errorf("Chrome count = %d, want 3", chrome.Count)
	}

	docker := findGroup(groups, "Docker")
	if docker == nil {
		t.Fatal("expected Docker group")
	}

	devServers := findGroup(groups, "Dev Servers")
	if devServers == nil {
		t.Fatal("expected Dev Servers group")
	}
}

func TestClassifyEmpty(t *testing.T) {
	groups := ClassifyProcesses(nil)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for nil input, got %d", len(groups))
	}
}

func TestParseVmmapSwap(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		wantKB int
	}{
		{
			name:   "megabytes",
			line:   "TOTAL                             10.7G   536.6M   311.3M   204.5M       0K      32K       0K     3628",
			wantKB: 209408, // 204.5 * 1024
		},
		{
			name:   "gigabytes",
			line:   "TOTAL                             10.7G   536.6M   311.3M   1.2G       0K      32K       0K     3628",
			wantKB: 1258291, // 1.2 * 1024 * 1024
		},
		{
			name:   "kilobytes",
			line:   "TOTAL                             10.7G   536.6M   311.3M   512K       0K      32K       0K     3628",
			wantKB: 512,
		},
		{
			name:   "zero",
			line:   "TOTAL                             10.7G   536.6M   311.3M   0K       0K      32K       0K     3628",
			wantKB: 0,
		},
		{
			name:   "empty",
			line:   "",
			wantKB: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVmmapSwap(tt.line)
			if got != tt.wantKB {
				t.Errorf("parseVmmapSwap() = %d, want %d", got, tt.wantKB)
			}
		})
	}
}

func TestParseSizeToKB(t *testing.T) {
	tests := []struct {
		input  string
		wantKB int
	}{
		{"204.5M", 209408},
		{"1.2G", 1258291},
		{"512K", 512},
		{"0K", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseSizeToKB(tt.input)
		if got != tt.wantKB {
			t.Errorf("parseSizeToKB(%q) = %d, want %d", tt.input, got, tt.wantKB)
		}
	}
}

func TestClassifyInitializesSwapToNegOne(t *testing.T) {
	procs := parseProcesses(fixturePS)
	groups := ClassifyProcesses(procs)
	for _, g := range groups {
		if g.TotalSwap != -1 {
			t.Errorf("group %q TotalSwap = %d, want -1 (not scanned)", g.Name, g.TotalSwap)
		}
	}
}

func findGroup(groups []ProcessGroup, name string) *ProcessGroup {
	for i := range groups {
		if groups[i].Name == name {
			return &groups[i]
		}
	}
	return nil
}
