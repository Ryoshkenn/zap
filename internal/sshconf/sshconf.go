// Package sshconf reads host aliases from the user's ~/.ssh/config so zap can
// offer them in the remote picker without the user re-typing connection details.
package sshconf

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Hosts returns the concrete Host aliases declared in ~/.ssh/config, sorted and
// deduplicated. Wildcard patterns (containing * or ?) are skipped since they
// aren't directly connectable. A missing config file yields an empty slice.
func Hosts() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return hostsFromFile(filepath.Join(home, ".ssh", "config"))
}

func hostsFromFile(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	seen := map[string]bool{}
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// "Host" keyword is case-insensitive; a line may list several patterns.
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "Host") {
			continue
		}
		for _, pat := range fields[1:] {
			if strings.ContainsAny(pat, "*?!") {
				continue
			}
			if !seen[pat] {
				seen[pat] = true
				out = append(out, pat)
			}
		}
	}
	sort.Strings(out)
	return out
}
