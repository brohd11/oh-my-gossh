package app

import (
	"os"
	"path/filepath"
	"runtime"
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
	if line := c.SSHCommandLine("nas"); line != `ssh 'nas'` {
		t.Errorf("SSHCommandLine = %q", line)
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

// The detached-window launcher passes its command through `bash -c`, so every element
// has to survive a shell.
func TestSSHCommandLineIsShellSafe(t *testing.T) {
	c := &Ctx{override: "/tmp/my configs/ssh.conf"}

	got := c.SSHCommandLine("nas")
	if !strings.HasPrefix(got, "ssh ") {
		t.Fatalf("SSHCommandLine = %q", got)
	}
	// A space in the path must not split into two arguments.
	if strings.Contains(got, "/tmp/my configs") && !strings.Contains(got, `'/tmp/my configs/ssh.conf'`) {
		t.Errorf("SSHCommandLine = %q, want the path quoted", got)
	}
	if out := shellEcho(t, strings.TrimPrefix(got, "ssh ")); !strings.Contains(out, "/tmp/my configs/ssh.conf") {
		t.Errorf("shell parsed the arguments as %q", out)
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

// The window launcher has to produce a runnable command on this platform, or the "open
// in a new window" option is dead on arrival.
func TestTerminalRunCmdBuildsSomethingRunnable(t *testing.T) {
	cmd := terminalRunCmd("ssh 'nas'")

	if cmd == nil {
		if runtime.GOOS == "linux" {
			t.Skip("no terminal emulator on PATH")
		}
		t.Fatal("terminalRunCmd returned nil")
	}
	if cmd.Path == "" {
		t.Error("command has no path")
	}

	if runtime.GOOS == "darwin" {
		// macOS Terminal has no run-this-command flag, so the command is staged in an
		// executable temp script.
		script := cmd.Args[len(cmd.Args)-1]
		defer os.Remove(script)

		body, err := os.ReadFile(script)
		if err != nil {
			t.Fatalf("temp script: %v", err)
		}
		if !strings.Contains(string(body), "ssh 'nas'") {
			t.Errorf("script does not run the command:\n%s", body)
		}
		info, err := os.Stat(script)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o100 == 0 {
			t.Errorf("script mode %v is not executable", info.Mode().Perm())
		}
	}
}
