// Package sshcfg reads OpenSSH client config and lists the concrete Host entries. Only the
// displayed keys are parsed; patterns and Match blocks are skipped.
package sshcfg

import (
	"bufio"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/brohd11/goutil/strutil"
)

// DefaultPort is what ssh uses when a Host block declares no Port.
const DefaultPort = "22"

// maxIncludeDepth caps Include recursion. ssh itself allows deep nesting, but a cycle
// (a file including its own directory glob) would otherwise never terminate.
const maxIncludeDepth = 5

// Host is one connectable entry. Alias is what ssh/scp receive, so every option in the
// block still applies; the other fields are for display.
type Host struct {
	Alias        string
	HostName     string
	User         string
	Port         string
	IdentityFile string
	ProxyJump    string
}

// Target is the alias, so ssh resolves the block's options itself.
func (h Host) Target() string { return h.Alias }

// Display is the human-readable address for a list row: user@hostname, with the port
// appended only when it is non-default (the common case stays uncluttered).
func (h Host) Display() string {
	addr := h.HostName
	if h.User != "" {
		addr = h.User + "@" + addr
	}
	if h.Port != "" && h.Port != DefaultPort {
		addr += ":" + h.Port
	}
	return addr
}

// DefaultPath is ~/.ssh/config, or "" when the home directory can't be determined.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", "config")
}

// Load reads the default config and returns its hosts and path. A missing file yields no
// hosts, not an error.
func Load() ([]Host, string, error) {
	path := DefaultPath()
	if path == "" {
		return nil, "", nil
	}
	hosts, err := LoadFile(path)
	return hosts, path, err
}

// LoadFile reads one config file. A non-existent path yields no hosts and no error (see
// Load); any other read failure is reported.
func LoadFile(path string) ([]Host, error) {
	return loadFile(path, 0)
}

func loadFile(path string, depth int) ([]Host, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	return parse(f, filepath.Dir(path), depth)
}

// Parse reads a config from r. Relative Include paths resolve against dir.
func Parse(r io.Reader, dir string) ([]Host, error) {
	return parse(r, dir, 0)
}

func parse(r io.Reader, dir string, depth int) ([]Host, error) {
	var (
		hosts   []Host
		current []*Host // every alias on the active Host line shares the block's options
		inMatch bool    // inside a Match block: its body sets defaults, not a target
	)

	// flush moves the active block's entries into the result. Called on each new Host
	// line and once at EOF, so a block is only emitted after all of its keys are seen.
	flush := func() {
		for _, h := range current {
			hosts = append(hosts, *h)
		}
		current = nil
	}

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		key, value, ok := splitLine(sc.Text())
		if !ok {
			continue
		}

		switch strings.ToLower(key) {
		case "host":
			flush()
			inMatch = false
			for _, alias := range splitFields(value) {
				if isPattern(alias) {
					continue // a defaults block (Host *) or a negation — not a target
				}
				current = append(current, &Host{Alias: alias})
			}
			continue

		case "match":
			// A Match block ends the preceding Host block and its own body is
			// conditional defaults, so nothing in it names a connectable target.
			flush()
			inMatch = true
			continue

		case "include":
			if depth >= maxIncludeDepth {
				continue
			}
			// Include contributes its Host blocks; per-block rebinding is not modeled.
			flush()
			hosts = append(hosts, includedHosts(value, dir, depth)...)
			continue
		}

		if inMatch || len(current) == 0 {
			continue // a Match body, or a key before any Host line (global defaults)
		}
		for _, h := range current {
			applyKey(h, key, value)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	flush()

	fillDefaults(hosts)
	return hosts, nil
}

// applyKey records one option on a host. The first occurrence wins, matching ssh, where
// the earliest-obtained value for a keyword is the one used.
func applyKey(h *Host, key, value string) {
	set := func(dst *string) {
		if *dst == "" {
			*dst = value
		}
	}
	switch strings.ToLower(key) {
	case "hostname":
		set(&h.HostName)
	case "user":
		set(&h.User)
	case "port":
		set(&h.Port)
	case "identityfile":
		set(&h.IdentityFile)
	case "proxyjump":
		set(&h.ProxyJump)
	}
}

// fillDefaults applies ssh's fallbacks: HostName = alias, Port = 22, User = local user.
func fillDefaults(hosts []Host) {
	local := localUser()
	for i := range hosts {
		if hosts[i].HostName == "" {
			hosts[i].HostName = hosts[i].Alias
		}
		if hosts[i].Port == "" {
			hosts[i].Port = DefaultPort
		}
		if hosts[i].User == "" {
			hosts[i].User = local
		}
		hosts[i].IdentityFile = expandHome(hosts[i].IdentityFile)
	}
}

func localUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}

// includedHosts expands an Include value (several, possibly globbed, patterns) and parses
// each match. Unreadable includes are skipped, as ssh does.
func includedHosts(value, dir string, depth int) []Host {
	var out []Host
	for _, pattern := range splitFields(value) {
		pattern = expandHome(pattern)
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(dir, pattern)
		}
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, m := range matches {
			hosts, err := loadFile(m, depth+1)
			if err != nil {
				continue
			}
			out = append(out, hosts...)
		}
	}
	return out
}

// splitLine strips comments and splits "Key value" or "Key=value". ok is false for blank
// lines.
func splitLine(line string) (key, value string, ok bool) {
	if i := strings.IndexByte(line, '#'); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", false
	}
	// The separator is the first whitespace or '=' run; everything after it is the value.
	i := strings.IndexAny(line, " \t=")
	if i < 0 {
		return line, "", true // a bare keyword; no value to apply
	}
	key = line[:i]
	value = strings.TrimLeft(line[i:], " \t")
	value = strings.TrimPrefix(value, "=")
	value = strings.TrimSpace(value)
	return key, unquote(value), true
}

// unquote removes one layer of surrounding quotes from a value, as ssh does for values
// containing spaces.
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// splitFields splits a multi-value line (a Host line's aliases, an Include's patterns)
// on whitespace, honoring quotes so a quoted path with spaces survives as one field.
func splitFields(s string) []string {
	var (
		out   []string
		cur   strings.Builder
		quote rune
	)
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			cur.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// isPattern reports whether a Host alias is a wildcard or negation, not a target.
func isPattern(alias string) bool {
	return strings.ContainsAny(alias, "*?") || strings.HasPrefix(alias, "!")
}

// expandHome resolves a leading ~. A path it cannot resolve is kept as written.
func expandHome(path string) string {
	if p, err := strutil.ExpandHome(path); err == nil {
		return p
	}
	return path
}
