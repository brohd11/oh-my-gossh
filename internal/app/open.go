package app

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/brohd11/oh-my-gossh/internal/sshcfg"

	tea "charm.land/bubbletea/v2"
	"github.com/brohd11/bubblestack/core"
	"github.com/brohd11/bubblestack/sysopen"
)

// openInline suspends the TUI and runs an interactive ssh session in this terminal. Only the
// alias is passed, so ssh applies the block's IdentityFile, Port, ProxyJump etc.
func openInline(sh *core.Shared, h sshcfg.Host) core.Action {
	return runInlineSSH(sh, []string{h.Target()}, func(err error) core.Action {
		if err != nil {
			return core.SetStatusAndLog("ssh " + h.Alias + ": " + err.Error())
		}
		return core.SetStatus("session to " + h.Alias + " closed")
	})
}

// runInlineSSH suspends the TUI for ssh (args follow SSHArgs' prefix). report turns the
// outcome into the status line.
func runInlineSSH(sh *core.Shared, args []string, report func(error) core.Action) core.Action {
	cmd := exec.Command("ssh", Of(sh).SSHArgs(args...)...)
	return core.Async(tea.ExecProcess(cmd, func(err error) tea.Msg {
		return report(err).Msg
	}))
}

// openWindow launches the session in a detached terminal window, keeping the TUI up.
func openWindow(sh *core.Shared, h sshcfg.Host) core.Action {
	argv := append([]string{"ssh"}, Of(sh).SSHArgs(h.Target())...)
	return sysopen.Terminal(workDir(), windowArgv(argv)...)
}

// workDir is the cwd, where the new window opens; gossh has no directory of its own.
func workDir() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	home, _ := os.UserHomeDir()
	return home // an empty string fails sysopen's stat, which reports it on the status line
}

// windowArgv keeps the window open after the command exits. It must be a single line:
// darwin passes it through an AppleScript string, which cannot hold a newline. Windows
// already keeps the window via `cmd /k`.
func windowArgv(argv []string) []string {
	if runtime.GOOS == "windows" {
		return argv
	}
	line := sysopen.ShellJoin(argv) + `; echo; echo "---------------"; echo "Command finished, Enter to close window."; read _`
	return []string{"bash", "-c", line}
}

// powerOffInline runs the shutdown over ssh -t inline, so sudo has a terminal to prompt on.
func powerOffInline(sh *core.Shared, h sshcfg.Host) core.Action {
	return runInlineSSH(sh, []string{"-t", h.Target(), poweroffCommand}, func(err error) core.Action {
		if err != nil {
			// The host drops the connection when it powers off, so a non-zero exit is expected.
			return core.SetStatusAndLog("poweroff " + h.Alias + ": " + err.Error() + " (expected if the host went down)")
		}
		return core.SetStatusAndLog("poweroff sent to " + h.Alias)
	})
}

// poweroffCommand is resolved through PATH; per-host overrides need a supplemental config.
const poweroffCommand = "sudo poweroff"
