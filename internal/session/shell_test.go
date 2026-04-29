package session

import "testing"

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: "''"},
		{input: "main", want: "'main'"},
		{input: "feature/with space", want: "'feature/with space'"},
		{input: "repo'; rm -rf /", want: "'repo'\\''; rm -rf /'"},
	}

	for _, tt := range tests {
		if got := shellQuote(tt.input); got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestShellQuotePathPreservesRemoteHomeExpansion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "~", want: "~"},
		{input: "~/fleet work/repo.git", want: "~/'fleet work/repo.git'"},
		{input: "/Users/me/fleet work/repo.git", want: "'/Users/me/fleet work/repo.git'"},
	}

	for _, tt := range tests {
		if got := shellQuotePath(tt.input); got != tt.want {
			t.Errorf("shellQuotePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
