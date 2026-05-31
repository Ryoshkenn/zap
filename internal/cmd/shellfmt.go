package cmd

import (
	"os"
	"strings"
)

// abbrevHome shortens an absolute path under $HOME to a leading ~.
func abbrevHome(p string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

// shellArg quotes an argument for display in --print output. It only adds
// quotes when the value contains characters a shell would treat specially, so
// the common case stays readable.
func shellArg(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return strings.ContainsRune(" \t\n\"'\\$`&|;<>(){}*?[]#~!", r)
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellJoin renders argv as a single shell-ish command line for display.
func shellJoin(parts []string) string {
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = shellArg(p)
	}
	return strings.Join(out, " ")
}
