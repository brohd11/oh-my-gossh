package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// This is a terminal probe for *running a command* in a new window. gdaddon's sysopen
// and repoview's terminal.go both solve the neighbouring problem — open a shell *at a
// directory* — and their tables are keyed to `--working-directory`, which is the wrong
// option here. bubblestack ships no terminal helper at all, so this is a local table.

// wrapCommand builds the shell line the new window runs: the command, then a pause so
// the window survives long enough to read the output. Ported from ssh_shutdown_nas.py:67,
// which did the same thing for the same reason — a detached terminal that exits with its
// command shows the result for a few milliseconds and then vanishes.
func wrapCommand(cmdline string) string {
	return cmdline + `
echo
echo "---------------"
echo "Command finished, Enter to close window."
read _
`
}

// emulators is the probe order for linux, each entry paired with the option *it* uses to
// run a command. The order follows what each desktop actually ships first.
//
// x-terminal-emulator is last on purpose: it is the Debian alternatives symlink, and its
// contract only guarantees xterm's -e, so it is a fallback rather than a preference.
var emulators = []struct {
	bin string
	// args is the prefix before the command; {cmd} is substituted with the shell line
	// when present, otherwise the command is appended as trailing arguments.
	args []string
}{
	{"gnome-terminal", []string{"--", "bash", "-c", "{cmd}"}},
	{"ptyxis", []string{"--", "bash", "-c", "{cmd}"}},
	{"kgx", []string{"--", "bash", "-c", "{cmd}"}},
	{"mate-terminal", []string{"--", "bash", "-c", "{cmd}"}},
	{"konsole", []string{"-e", "bash", "-c", "{cmd}"}},
	{"xfce4-terminal", []string{"-e", "bash -c {cmdq}"}},
	{"tilix", []string{"-e", "bash -c {cmdq}"}},
	{"terminator", []string{"-e", "bash -c {cmdq}"}},
	{"lxterminal", []string{"-e", "bash -c {cmdq}"}},
	{"kitty", []string{"bash", "-c", "{cmd}"}},
	{"alacritty", []string{"-e", "bash", "-c", "{cmd}"}},
	{"wezterm", []string{"start", "--", "bash", "-c", "{cmd}"}},
	{"foot", []string{"bash", "-c", "{cmd}"}},
	{"urxvt", []string{"-e", "bash", "-c", "{cmd}"}},
	{"st", []string{"-e", "bash", "-c", "{cmd}"}},
	{"xterm", []string{"-e", "bash", "-c", "{cmd}"}},
	{"x-terminal-emulator", []string{"-e", "bash", "-c", "{cmd}"}},
}

// terminalRunCmd builds the command that opens a new terminal window running cmdline,
// or nil when no emulator could be found (linux with none on PATH).
func terminalRunCmd(cmdline string) *exec.Cmd {
	script := wrapCommand(cmdline)
	switch runtime.GOOS {
	case "darwin":
		return darwinTerminalCmd(script)
	case "windows":
		return exec.Command("cmd", "/c", "start", "cmd", "/k", cmdline)
	default:
		return probeTerminal(script)
	}
}

// probeTerminal returns the first emulator on PATH wired to run script, or nil when none
// is installed.
func probeTerminal(script string) *exec.Cmd {
	for _, t := range emulators {
		if _, err := exec.LookPath(t.bin); err != nil {
			continue
		}
		args := make([]string, 0, len(t.args))
		for _, a := range t.args {
			// The emulators taking a single -e string need the script quoted into it;
			// the ones taking argv pass it through untouched.
			a = strings.ReplaceAll(a, "{cmdq}", shellQuote(script))
			a = strings.ReplaceAll(a, "{cmd}", script)
			args = append(args, a)
		}
		return exec.Command(t.bin, args...)
	}
	return nil
}

// darwinTerminalCmd opens Terminal.app on a temp script. macOS Terminal has no
// run-this-command argument — `open -a Terminal <file>` runs an executable file — so the
// script is written to disk and deletes itself as its last act.
func darwinTerminalCmd(script string) *exec.Cmd {
	f, err := os.CreateTemp("", "go-ssh-*.command")
	if err != nil {
		return nil
	}
	// The self-delete keeps a long-lived session from littering /tmp; it runs after the
	// pause, so the window is still readable until the user dismisses it.
	body := "#!/bin/bash\n" + script + "\nrm -f " + shellQuote(f.Name()) + "\n"
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0o700); err != nil {
		os.Remove(f.Name())
		return nil
	}
	return exec.Command("open", "-a", "Terminal", f.Name())
}

// shellQuote wraps s for a POSIX shell as a single-quoted literal, so nothing inside is
// expanded or word-split. Embedded single quotes are closed, escaped, and reopened.
//
// The Python built remote commands by interpolating straight into f-strings, which is
// how ssh_transfer_files.py:65 ended up emitting `'[ -e /path' ]` — a misplaced quote
// that made every existence check report the same answer. Anything crossing a shell
// boundary goes through here instead.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// quoteRemotePath quotes a path that ssh/scp will hand to the *remote* shell, leaving a
// leading ~ or ~user prefix unquoted so that shell still expands it.
//
// Plain shellQuote is wrong here: it produces '~/Desktop', and a tilde inside single
// quotes is a literal, so the transfer would create a directory actually named "~" in the
// remote home. Only the segments after the tilde need protecting.
func quoteRemotePath(p string) string {
	if p == "" || p[0] != '~' {
		return shellQuote(p)
	}
	slash := strings.IndexByte(p, '/')
	if slash < 0 {
		return p // bare ~ or ~user: nothing left to quote
	}
	return p[:slash+1] + shellQuote(p[slash+1:])
}

// quoteJoin renders a path list for a confirm dialog, quoting only what needs it so the
// common case stays readable.
func quoteJoin(paths []string) string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	return strings.Join(out, ", ")
}
