package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brohd11/bubblestack/sysopen"
)

// The pure single-quoting (sysopen.ShellQuote/ShellJoin) is exercised by bubblestack's own
// tests; the tests here cover go-ssh's shell-boundary contract — the remote-path tilde
// handling and the real-shell round trips the transfer commands depend on.

// The remote path is the case the Python got wrong. A tilde inside single quotes is a
// literal, so quoting the whole path would make the remote mkdir create a directory
// actually named "~" instead of writing to the home directory.
func TestQuoteRemotePathPreservesTilde(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"~/Desktop", `~/'Desktop'`},
		{"~/My Files", `~/'My Files'`},
		{"~", "~"},
		{"~user/dir", `~user/'dir'`},
		{"/absolute/path", `'/absolute/path'`},
		{"/path with space", `'/path with space'`},
		{"relative/dir", `'relative/dir'`},
		{"", `''`},
	} {
		if got := quoteRemotePath(tc.in); got != tc.want {
			t.Errorf("quoteRemotePath(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

// The real question is not what the quoted string looks like but what a shell makes of
// it, so this round-trips through /bin/sh — the same interpreter on the far side of an
// ssh command. Each input must come back as exactly one argument, unchanged.
func TestShellQuoteRoundTripsThroughRealShell(t *testing.T) {
	for _, in := range []string{
		"plain",
		"with space",
		"it's",
		"$HOME",
		"`whoami`",
		"a;b|c&d",
		`back\slash`,
		"quote\"double",
		"*glob?",
		"new\tline",
		`dir'; rm -rf /tmp/nope; echo '`, // the injection the quoting exists to stop
	} {
		out := shellEcho(t, sysopen.ShellQuote(in))
		if out != in {
			t.Errorf("ShellQuote(%q) round-tripped to %q", in, out)
		}
	}
}

// quoteRemotePath's contract is the same round trip, except a leading tilde is meant to
// expand — that is the whole reason it isn't plain sysopen.ShellQuote.
func TestQuoteRemotePathRoundTripsAndExpandsTilde(t *testing.T) {
	home := shellEcho(t, "~")
	if home == "" || home == "~" {
		t.Skip("shell does not expand ~")
	}

	for _, tc := range []struct{ in, want string }{
		{"~/Desktop", home + "/Desktop"},
		{"~/My Files", home + "/My Files"},
		{"~/dir'; rm -rf /tmp/nope; echo '", home + "/dir'; rm -rf /tmp/nope; echo '"},
		{"/absolute/path", "/absolute/path"},
		{"/path with space", "/path with space"},
		{"relative/dir", "relative/dir"},
	} {
		if out := shellEcho(t, quoteRemotePath(tc.in)); out != tc.want {
			t.Errorf("quoteRemotePath(%q) round-tripped to %q, want %q", tc.in, out, tc.want)
		}
	}
}

// shellEcho asks /bin/sh what a quoted fragment expands to, asserting it stays a single
// argument. printf '%s' collapses several arguments into one string, so the separate
// argument count check is what actually catches a word split.
func shellEcho(t *testing.T, quoted string) string {
	t.Helper()

	out, err := exec.Command("/bin/sh", "-c", "printf '%s' "+quoted+"; printf ' [%d]' $#").Output()
	if err != nil {
		t.Fatalf("sh with %s: %v", quoted, err)
	}
	got := string(out)
	// $# is 0 because the fragment is inside the -c script, not passed as arguments; the
	// marker just proves the script parsed and ran to completion.
	if !strings.HasSuffix(got, " [0]") {
		t.Fatalf("sh with %s produced %q, want a single completed expansion", quoted, got)
	}
	return strings.TrimSuffix(got, " [0]")
}

// ShellJoin's contract is that the shell splits the words back where they started, so
// this asks a real one. The --config path with a space in it is the case that matters:
// the detached window loses its -F argument if that splits in two.
func TestShellJoinRoundTripsThroughRealShell(t *testing.T) {
	argv := []string{"ssh", "-F", "/tmp/my configs/ssh.conf", "nas"}

	got := shellWords(t, sysopen.ShellJoin(argv))
	if strings.Join(got, "|") != strings.Join(argv, "|") {
		t.Errorf("ShellJoin(%q) round-tripped to %q", argv, got)
	}
	// Empty argv must not produce a stray argument.
	if got := shellWords(t, sysopen.ShellJoin(nil)); len(got) != 0 {
		t.Errorf("ShellJoin(nil) round-tripped to %q, want nothing", got)
	}
}

// shellWords asks /bin/sh to split a command line and reports the arguments it found.
// Unlike shellEcho this keeps them apart, which is the point when checking a whole argv.
func shellWords(t *testing.T, line string) []string {
	t.Helper()

	out, err := exec.Command("/bin/sh", "-c", "for a in "+line+"; do printf '%s\\n' \"$a\"; done").Output()
	if err != nil {
		t.Fatalf("sh with %s: %v", line, err)
	}
	if len(out) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
}

func TestQuoteJoinShowsBaseNames(t *testing.T) {
	got := quoteJoin([]string{"/home/b/notes.txt", "/home/b/photos"})

	if want := "notes.txt, photos"; got != want {
		t.Errorf("quoteJoin = %q, want %q", got, want)
	}
	if got := quoteJoin(nil); got != "" {
		t.Errorf("quoteJoin(nil) = %q, want empty", got)
	}
}

func TestPlural(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{{0, "0 items"}, {1, "1 item"}, {2, "2 items"}} {
		if got := plural(tc.n, "item"); got != tc.want {
			t.Errorf("plural(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// mkdirAndPwd is what turns the typed destination into the absolute path scp's SFTP target
// has to be, so the test that matters is what a real shell makes of it: the directory has
// to exist afterwards and the printed path has to be the one that was asked for — including
// for names a shell would otherwise split, expand, or execute.
func TestMkdirAndPwdCreatesAndReportsPath(t *testing.T) {
	for _, dest := range []string{
		"sub",
		"sub with space",
		"nested/deeper",
		`dir'; echo pwned; echo '`,
		`dir"double`,
		"$HOME",
		"*glob?",
	} {
		t.Run(dest, func(t *testing.T) {
			tmp := t.TempDir()
			line := mkdirAndPwd(dest)

			cmd := exec.Command("/bin/sh", "-c", line)
			cmd.Dir = tmp
			// A relative cd resolves against $PWD, which exec.Command leaves pointing at
			// the test process's own directory however Dir is set.
			cmd.Env = append(os.Environ(), "PWD="+tmp, "CDPATH=")
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("sh with %s: %v", line, err)
			}

			want := filepath.Join(tmp, dest)
			if got := strings.TrimSpace(string(out)); got != want {
				t.Errorf("mkdirAndPwd(%q) printed %q, want %q", dest, got, want)
			}
			if info, err := os.Stat(want); err != nil || !info.IsDir() {
				t.Errorf("mkdirAndPwd(%q) did not create %s (%v)", dest, want, err)
			}
		})
	}
}

// A destination pasted from a terminal arrives quoted for a shell that never sees it, so
// those quotes would end up in the remote directory's name.
func TestUnquoteInput(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`"~/Desktop"`, "~/Desktop"},
		{`'~/Desktop'`, "~/Desktop"},
		{"~/Desktop", "~/Desktop"},
		{`"unbalanced`, `"unbalanced`},
		{`unbalanced"`, `unbalanced"`},
		{`"`, `"`}, // one character is not a pair
		{`""`, ""},
		{`''`, ""},
		{"", ""},
		// Only the outer pair is stripped: anything further in is the name's own business.
		{`"a" and "b"`, `a" and "b`},
	} {
		if got := unquoteInput(tc.in); got != tc.want {
			t.Errorf("unquoteInput(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// lastLine is what keeps a chatty remote login from being mistaken for the destination:
// resolveDest reads the path off stdout, and an rc file can print there first.
func TestLastLine(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"/home/b/Desktop\n", "/home/b/Desktop"},
		{"welcome to the nas\n/home/b/Desktop\n", "/home/b/Desktop"},
		{"/home/b/Desktop\n\n\n", "/home/b/Desktop"},
		{"  /home/b/Desktop  \n", "/home/b/Desktop"},
		{"\n \n", ""},
		{"", ""},
	} {
		if got := lastLine(tc.in); got != tc.want {
			t.Errorf("lastLine(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
