package stream

import (
	"os"
	"path/filepath"
	"strings"
)

// ParseSSHConfigHosts extracts top-level Host entries from ~/.ssh/config.
// Wildcard entries (* / ?) are skipped; complex directives (Include/Match) are ignored.
func ParseSSHConfigHosts() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	if err != nil {
		return nil
	}
	var hosts []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "host ") {
			continue
		}
		for _, tok := range strings.Fields(line[len("Host"):]) {
			if tok == "" || strings.ContainsAny(tok, "*?") || seen[tok] {
				continue
			}
			seen[tok] = true
			hosts = append(hosts, tok)
		}
	}
	return hosts
}

// MergeSSHHosts merges explicit configured hosts with discovered ~/.ssh/config hosts,
// deduplicated, configured-first.
func MergeSSHHosts(configured []string, discovered []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, h := range append(append([]string{}, configured...), discovered...) {
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}
