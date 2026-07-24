package sshcfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// aliases is the shorthand most assertions want: which hosts came back, in order.
func aliases(hosts []Host) []string {
	out := make([]string, len(hosts))
	for i, h := range hosts {
		out[i] = h.Alias
	}
	return out
}

func mustParse(t *testing.T, cfg string) []Host {
	t.Helper()
	hosts, err := Parse(strings.NewReader(cfg), t.TempDir())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return hosts
}

func find(t *testing.T, hosts []Host, alias string) Host {
	t.Helper()
	for _, h := range hosts {
		if h.Alias == alias {
			return h
		}
	}
	t.Fatalf("host %q not found in %v", alias, aliases(hosts))
	return Host{}
}

// The real-world shape: indented keys, one block per host. Everything else in this file
// tests a deviation from it.
func TestParseBasicBlocks(t *testing.T) {
	hosts := mustParse(t, `
Host qnap-nas
    HostName 10.0.0.248
    User brohd11
    IdentityFile /Users/brohd/.ssh/qnap-nas

Host syn-nas
    HostName 10.0.0.205
    User brohd11
    Port 2222
`)

	if got, want := len(hosts), 2; got != want {
		t.Fatalf("got %d hosts %v, want %d", got, aliases(hosts), want)
	}

	qnap := find(t, hosts, "qnap-nas")
	if qnap.HostName != "10.0.0.248" || qnap.User != "brohd11" {
		t.Errorf("qnap-nas = %+v", qnap)
	}
	if qnap.IdentityFile != "/Users/brohd/.ssh/qnap-nas" {
		t.Errorf("IdentityFile = %q", qnap.IdentityFile)
	}
	// No Port declared, so the ssh default is filled in.
	if qnap.Port != DefaultPort {
		t.Errorf("Port = %q, want %q", qnap.Port, DefaultPort)
	}
	if syn := find(t, hosts, "syn-nas"); syn.Port != "2222" {
		t.Errorf("syn-nas Port = %q, want 2222", syn.Port)
	}
}

// Target must stay the alias: it is what preserves IdentityFile/Port/ProxyJump when ssh
// re-reads the block. Display is the decorated form and never reaches a command line.
func TestTargetIsAliasAndDisplayShowsAddress(t *testing.T) {
	hosts := mustParse(t, `
Host nas
    HostName 10.0.0.248
    User brohd11
    IdentityFile ~/.ssh/nas

Host oddport
    HostName example.com
    User bob
    Port 2222
`)

	nas := find(t, hosts, "nas")
	if nas.Target() != "nas" {
		t.Errorf("Target() = %q, want the alias", nas.Target())
	}
	// The default port stays out of the display; a custom one is worth showing.
	if got, want := nas.Display(), "brohd11@10.0.0.248"; got != want {
		t.Errorf("Display() = %q, want %q", got, want)
	}
	if got, want := find(t, hosts, "oddport").Display(), "bob@example.com:2222"; got != want {
		t.Errorf("Display() = %q, want %q", got, want)
	}
}

// Wildcards and negations attach options to a set of hosts; they name no target, so they
// must not appear as pickable rows.
func TestPatternHostsAreSkipped(t *testing.T) {
	hosts := mustParse(t, `
Host *
    ServerAliveInterval 60
    User defaultuser

Host *.internal
    User intern

Host !prod
    User notprod

Host real
    HostName 10.0.0.1
`)

	if got, want := aliases(hosts), []string{"real"}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("aliases = %v, want %v", got, want)
	}
	// Keys under "Host *" are global defaults this parser does not propagate; the
	// concrete host falls back to the local username instead of inheriting "defaultuser".
	if h := hosts[0]; h.User == "defaultuser" {
		t.Errorf("User = %q, want the wildcard block not to leak in", h.User)
	}
}

// One Host line may declare several aliases sharing a block. Each becomes its own entry
// with the same options.
func TestMultipleAliasesOnOneHostLine(t *testing.T) {
	hosts := mustParse(t, `
Host web1 web2
    HostName 10.0.0.9
    User deploy
`)

	if got, want := len(hosts), 2; got != want {
		t.Fatalf("got %d hosts %v, want %d", got, aliases(hosts), want)
	}
	for _, h := range hosts {
		if h.HostName != "10.0.0.9" || h.User != "deploy" {
			t.Errorf("%s = %+v, want the shared block's options", h.Alias, h)
		}
	}
	// A wildcard alongside real aliases is dropped without taking them with it.
	mixed := mustParse(t, "Host prod *.dev\n    User deploy\n")
	if got := aliases(mixed); len(got) != 1 || got[0] != "prod" {
		t.Errorf("aliases = %v, want [prod]", got)
	}
}

// A Match block ends the preceding Host block, and its body is conditional defaults that
// must not be attributed to the host above it.
func TestMatchBlockEndsHostAndIsSkipped(t *testing.T) {
	hosts := mustParse(t, `
Host alpha
    HostName 10.0.0.1

Match host beta
    User matched
    Port 9999

Host gamma
    HostName 10.0.0.3
`)

	if got, want := aliases(hosts), []string{"alpha", "gamma"}; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("aliases = %v, want %v", got, want)
	}
	if alpha := find(t, hosts, "alpha"); alpha.User == "matched" || alpha.Port == "9999" {
		t.Errorf("alpha = %+v, want the Match body excluded", alpha)
	}
}

func TestKeyEqualsValueAndCaseInsensitiveKeys(t *testing.T) {
	hosts := mustParse(t, `
Host eq
    hostname=10.0.0.7
    USER=bob
    PoRt = 2200
`)

	h := find(t, hosts, "eq")
	if h.HostName != "10.0.0.7" || h.User != "bob" || h.Port != "2200" {
		t.Errorf("eq = %+v", h)
	}
}

func TestCommentsAndQuotedValues(t *testing.T) {
	hosts := mustParse(t, `
# a leading comment
Host quoted   # trailing comment on the host line
    HostName 10.0.0.8   # and on a key line
    IdentityFile "/path/with spaces/key"
`)

	h := find(t, hosts, "quoted")
	if h.HostName != "10.0.0.8" {
		t.Errorf("HostName = %q, want the trailing comment stripped", h.HostName)
	}
	if h.IdentityFile != "/path/with spaces/key" {
		t.Errorf("IdentityFile = %q, want the quotes removed and the space kept", h.IdentityFile)
	}
}

// ssh uses the first value obtained for a keyword, not the last.
func TestFirstValueWins(t *testing.T) {
	hosts := mustParse(t, `
Host dup
    HostName first.example
    HostName second.example
`)

	if h := find(t, hosts, "dup"); h.HostName != "first.example" {
		t.Errorf("HostName = %q, want first.example", h.HostName)
	}
}

// An absent HostName means the alias is the hostname — the case where someone writes a
// bare "Host myserver" for a name DNS already resolves.
func TestAliasIsHostNameWhenAbsent(t *testing.T) {
	hosts := mustParse(t, "Host bare.example.com\n    User bob\n")

	if h := find(t, hosts, "bare.example.com"); h.HostName != "bare.example.com" {
		t.Errorf("HostName = %q, want the alias", h.HostName)
	}
}

func TestIncludeIsFollowed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "extra.conf"), "Host included\n    HostName 10.0.0.50\n    User inc\n")

	hosts, err := Parse(strings.NewReader("Host main\n    HostName 10.0.0.1\n\nInclude extra.conf\n"), dir)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got, want := len(hosts), 2; got != want {
		t.Fatalf("got %d hosts %v, want %d", got, aliases(hosts), want)
	}
	if h := find(t, hosts, "included"); h.HostName != "10.0.0.50" || h.User != "inc" {
		t.Errorf("included = %+v", h)
	}
}

func TestIncludeGlobAndMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.conf"), "Host a\n    HostName 10.0.0.11\n")
	writeFile(t, filepath.Join(dir, "b.conf"), "Host b\n    HostName 10.0.0.12\n")

	// The missing include must not cost us the hosts declared around it.
	hosts, err := Parse(strings.NewReader("Include *.conf\nInclude nope/*.conf\n\nHost local\n    HostName 10.0.0.1\n"), dir)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	for _, want := range []string{"a", "b", "local"} {
		find(t, hosts, want)
	}
	if got, want := len(hosts), 3; got != want {
		t.Fatalf("got %d hosts %v, want %d", got, aliases(hosts), want)
	}
}

// A file that includes its own directory glob would recurse forever without the depth
// cap; the parser must terminate and still report the hosts it saw.
func TestIncludeCycleTerminates(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "loop.conf"), "Host looped\n    HostName 10.0.0.60\nInclude loop.conf\n")

	done := make(chan []Host, 1)
	go func() {
		hosts, _ := Parse(strings.NewReader("Include loop.conf\n"), dir)
		done <- hosts
	}()

	select {
	case hosts := <-done:
		if len(hosts) == 0 {
			t.Fatal("got no hosts, want the cycle to still yield its declarations")
		}
		for _, h := range hosts {
			if h.Alias != "looped" {
				t.Errorf("unexpected alias %q", h.Alias)
			}
		}
	case <-timeout(t):
		t.Fatal("Parse did not terminate on a self-including file")
	}
}

// A missing config is the normal state on a fresh machine, not a failure: the UI shows a
// placeholder instead of an error.
func TestLoadFileMissingIsNotAnError(t *testing.T) {
	hosts, err := LoadFile(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("LoadFile: %v, want nil for a missing file", err)
	}
	if len(hosts) != 0 {
		t.Errorf("got %d hosts, want 0", len(hosts))
	}
}

func TestEmptyAndKeysBeforeAnyHost(t *testing.T) {
	// Keys before the first Host line are global defaults with no block to attach to.
	hosts := mustParse(t, "ServerAliveInterval 60\nUser globaluser\n")
	if len(hosts) != 0 {
		t.Errorf("got %v, want no hosts", aliases(hosts))
	}
	if hosts := mustParse(t, ""); len(hosts) != 0 {
		t.Errorf("got %v, want no hosts", aliases(hosts))
	}
}

func TestIdentityFileHomeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}

	hosts := mustParse(t, "Host tilde\n    IdentityFile ~/.ssh/id_test\n")

	want := filepath.Join(home, ".ssh", "id_test")
	if h := find(t, hosts, "tilde"); h.IdentityFile != want {
		t.Errorf("IdentityFile = %q, want %q", h.IdentityFile, want)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// timeout bounds a test that would otherwise hang rather than fail.
func timeout(t *testing.T) <-chan time.Time {
	t.Helper()
	return time.After(5 * time.Second)
}
