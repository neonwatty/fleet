package session

import "strings"

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shellQuotePath(path string) string {
	switch {
	case path == "~":
		return path
	case strings.HasPrefix(path, "~/"):
		return "~/" + shellQuote(path[2:])
	default:
		return shellQuote(path)
	}
}
