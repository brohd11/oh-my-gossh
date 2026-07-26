package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathLabel(t *testing.T) {
	for _, tc := range []struct {
		paths []string
		want  string
	}{
		{nil, "no selection"},
		{[]string{"/home/b/notes.txt"}, "notes.txt"},
		{[]string{"/a", "/b"}, "2 items"},
	} {
		c := &Ctx{Paths: tc.paths}
		if got := c.PathLabel(); got != tc.want {
			t.Errorf("PathLabel(%v) = %q, want %q", tc.paths, got, tc.want)
		}
	}
}

// Without --config, ssh must be left to find ~/.ssh/config itself — adding a redundant
// -F would only be a way to get it wrong.
func TestSSHArgsDefaultAddsNothing(t *testing.T) {
	c := &Ctx{}

	got := c.SSHArgs("nas")
	if len(got) != 1 || got[0] != "nas" {
		t.Errorf("SSHArgs = %q, want just the alias", got)
	}
}

// With --config, -F is what makes the alias resolvable. Without it ssh reads
// ~/.ssh/config, finds no such Host, and fails with "Could not resolve hostname".
func TestSSHArgsOverridePassesConfigThrough(t *testing.T) {
	c := &Ctx{override: "/tmp/custom.conf"}

	got := c.SSHArgs("-t", "nas", "sudo poweroff")
	want := []string{"-F", "/tmp/custom.conf", "-t", "nas", "sudo poweroff"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("SSHArgs = %q, want %q", got, want)
	}

	// SSHArgs() with no arguments is how the transfer builds its scp prefix.
	if base := c.SSHArgs(); len(base) != 2 || base[0] != "-F" {
		t.Errorf("SSHArgs() = %q, want the -F prefix alone", base)
	}
}

// Load must not report a missing file as an error — that is the normal state before a
// user has written a config, and the UI shows a placeholder for it.
func TestLoadMissingConfigLeavesNoError(t *testing.T) {
	c := &Ctx{}
	c.Load(filepath.Join(t.TempDir(), "absent.conf"))

	if c.LoadErr != nil {
		t.Errorf("LoadErr = %v, want nil", c.LoadErr)
	}
	if len(c.Hosts) != 0 {
		t.Errorf("got %d hosts, want 0", len(c.Hosts))
	}
}

func TestLoadReadsHostsAndRecordsOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh.conf")
	if err := os.WriteFile(path, []byte("Host nas\n    HostName 10.0.0.1\n    User bob\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := &Ctx{}
	c.Load(path)

	if len(c.Hosts) != 1 || c.Hosts[0].Alias != "nas" {
		t.Fatalf("Hosts = %+v", c.Hosts)
	}
	if c.ConfigPath != path {
		t.Errorf("ConfigPath = %q, want %q", c.ConfigPath, path)
	}
	// The override must be recorded, or the ssh invocations lose their -F.
	if got := c.SSHArgs("nas"); got[0] != "-F" {
		t.Errorf("SSHArgs = %q, want the -F prefix", got)
	}
	if _, ok := c.Host("nas"); !ok {
		t.Error("Host(nas) not found")
	}
	if _, ok := c.Host("absent"); ok {
		t.Error("Host(absent) found")
	}
}
