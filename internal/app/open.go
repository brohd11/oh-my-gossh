package app

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/brohd11/oh-my-gossh/internal/sshcfg"

	"github.com/brohd11/bubblestack/core"
	"github.com/brohd11/bubblestack/sysopen"
	tea "github.com/charmbracelet/bubbletea"
)

// openInline attaches an interactive ssh session to this terminal: bubbletea suspends,
// ssh owns the tty until the user exits, then the TUI is restored.
//
// Only the alias is passed. The Python rebuilt user@ip (ssh.py:44) and handed ssh that,
// which silently discarded IdentityFile, Port, and ProxyJump from the very config block
// the address came from; naming the alias lets ssh resolve its own options.
func openInline(sh *core.Shared, h sshcfg.Host) core.Action {
	cmd := exec.Command("ssh", Of(sh).SSHArgs(h.Target())...)
	return core.Async(tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return core.SetStatusAndLog("ssh " + h.Alias + ": " + err.Error()).Msg
		}
		return core.SetStatus("session to " + h.Alias + " closed").Msg
	}))
}

// openWindow launches the session in a detached terminal window, the behavior the Python
// had (ssh_open.py:51). The TUI stays up, so this is how a user keeps several sessions
// while still driving the menu.
func openWindow(sh *core.Shared, h sshcfg.Host) core.Action {
	argv := append([]string{"ssh"}, Of(sh).SSHArgs(h.Target())...)
	return sysopen.Terminal(workDir(), windowArgv(argv)...)
}

// workDir is the directory the new window opens at. sysopen.Terminal stats it, and gossh
// has no directory of its own — the cwd is the closest thing to one, and it is the folder
// being browsed when a file-manager action launched us.
func workDir() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	home, _ := os.UserHomeDir()
	return home // an empty string fails sysopen's stat, which reports it on the status line
}

// windowArgv wraps argv so the window survives its command: a detached terminal that
// exits with ssh shows the result for a few milliseconds and then vanishes. Ported from
// ssh_shutdown_nas.py:67, which paused for the same reason.
//
// The line has to stay on one line. darwin runs it through `osascript ... do script`, and
// an AppleScript string literal cannot hold a raw newline — the multi-line form the local
// launcher used would be a syntax error there. Windows is exempt: sysopen opens it with
// `cmd /k`, which already keeps the window up, and there is no bash to wrap with.
func windowArgv(argv []string) []string {
	if runtime.GOOS == "windows" {
		return argv
	}
	line := sysopen.ShellJoin(argv) + `; echo; echo "---------------"; echo "Command finished, Enter to close window."; read _`
	return []string{"bash", "-c", line}
}

// powerOffInline runs the shutdown over an ssh tty (-t), inline rather than through a
// TaskScreen.
//
// The tty is the whole point: `sudo poweroff` prompts for a password on most hosts, and a
// streaming background task has no terminal to answer with — it would sit there looking
// busy until the user aborted it. Suspending the TUI hands sudo a real one.
func powerOffInline(sh *core.Shared, h sshcfg.Host) core.Action {
	cmd := exec.Command("ssh", Of(sh).SSHArgs("-t", h.Target(), poweroffCommand)...)
	return core.Async(tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			// A host that powers off mid-command drops the connection, so ssh exits
			// non-zero on success as often as on failure. Report it without calling it
			// an error.
			return core.SetStatusAndLog("poweroff " + h.Alias + ": " + err.Error() + " (expected if the host went down)").Msg
		}
		return core.SetStatusAndLog("poweroff sent to " + h.Alias).Msg
	}))
}

// poweroffCommand is the generic shutdown. The Python hardcoded a different path per NAS
// (`sudo /sbin/poweroff` for qnap, `sudo poweroff` for synology — ssh_shutdown_nas.py:60);
// ssh config has nowhere to record that, so per-host overrides wait for the supplemental
// config. `sudo poweroff` resolves through PATH and covers both.
const poweroffCommand = "sudo poweroff"
