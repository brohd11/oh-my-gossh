package app

import (
	"os/exec"

	"github.com/brohd11/go-ssh/internal/sshcfg"

	"github.com/brohd11/bubblestack/core"
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
	return runInWindow(Of(sh).SSHCommandLine(h.Target()), "session to "+h.Alias)
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

// runInWindow opens a detached terminal running cmdline, reporting on the status line
// when no emulator is available rather than failing silently.
func runInWindow(cmdline, what string) core.Action {
	cmd := terminalRunCmd(cmdline)
	if cmd == nil {
		return core.SetStatusAndLog("no terminal emulator found — use the inline option instead")
	}
	return core.Seq(
		core.SetStatus("opening "+what+" in a new window"),
		core.Async(func() tea.Msg {
			if err := cmd.Start(); err != nil {
				return core.SetStatusAndLog("could not open terminal: " + err.Error()).Msg
			}
			go cmd.Wait() //nolint:errcheck // reap the child; a window left open just parks this goroutine
			return nil
		}),
	)
}
