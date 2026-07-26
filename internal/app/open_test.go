package app

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// The wrapper must keep the terminal alive after the command exits; without the trailing
// read the window closes before anything can be read.
func TestWindowArgvPausesAfterTheCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows opens with cmd /k, which pauses on its own")
	}

	got := windowArgv([]string{"ssh", "nas"})

	if len(got) != 3 || got[0] != "bash" || got[1] != "-c" {
		t.Fatalf("windowArgv = %q, want a bash -c invocation", got)
	}
	if !strings.HasPrefix(got[2], `'ssh' 'nas'`) {
		t.Errorf("line = %q, want the command first", got[2])
	}
	if !strings.Contains(got[2], "read _") {
		t.Errorf("line = %q, want a trailing read so the window stays open", got[2])
	}
}

// The line has to stay on one line: darwin runs it through `osascript ... do script`, and
// a raw newline inside an AppleScript string literal is a syntax error, so a multi-line
// wrapper would fail there and only there.
func TestWindowArgvStaysOnOneLine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows is not wrapped")
	}

	for _, argv := range [][]string{
		{"ssh", "nas"},
		{"ssh", "-F", "/tmp/my configs/ssh.conf", "nas"},
	} {
		line := windowArgv(argv)[2]
		if strings.ContainsAny(line, "\n\r") {
			t.Errorf("windowArgv(%q) line contains a newline:\n%s", argv, line)
		}
	}
}

// Windows takes the argv untouched — there is no bash to wrap with, and sysopen opens it
// with `cmd /k`, which already holds the window open.
func TestWindowArgvLeavesWindowsAlone(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("not windows")
	}

	argv := []string{"ssh", "nas"}
	if got := windowArgv(argv); strings.Join(got, "|") != strings.Join(argv, "|") {
		t.Errorf("windowArgv = %q, want it unchanged", got)
	}
}

// sysopen.Terminal stats the directory it is given and refuses to open at one that is
// missing, so workDir must always name a real place.
func TestWorkDirExists(t *testing.T) {
	got := workDir()

	if got == "" {
		t.Fatal("workDir is empty")
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatalf("workDir = %q: %v", got, err)
	}
	if !info.IsDir() {
		t.Errorf("workDir = %q, want a directory", got)
	}
}
