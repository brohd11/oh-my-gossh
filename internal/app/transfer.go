package app

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/brohd11/oh-my-gossh/internal/sshcfg"

	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"
	"github.com/charmbracelet/bubbles/key"
)

// defaultDest is where a blank destination lands, matching ssh_transfer_files.py:32.
const defaultDest = "~/Desktop"

// transferForm asks for the destination directory. The Python offered a per-target list
// of preset dirs from its config dict (ssh.py:10); ssh config has nowhere to record
// those, so this pass is a single free-text field and the presets return with the
// supplemental config.
func transferForm(sh *core.Shared, h sshcfg.Host) core.Screen {
	// The label carries its own colon and trailing space: fieldRow lays the label
	// straight against the input with no separator of its own.
	dest := components.NewTextField("dest", "Destination: ", defaultDest)

	return components.NewForm(components.FormOpts{
		Title: "Transfer to " + h.Alias,
		Crumb: "Transfer",
		Fields: []components.FormField{
			components.NewNote(plural(len(Of(sh).Paths), "item") + " → " + h.Display()),
			components.NewSpacer(),
			dest,
			components.NewSpacer(),
			components.NewNote("blank ⇒ " + defaultDest),
		},
		Help: []key.Binding{
			core.Hint("continue", core.Keys.Select),
			core.Hint("cancel", core.Keys.Back),
		},
		OnSubmit: func(sh *core.Shared, f *components.FormScreen) core.Action {
			// unquoteInput because a path pasted from a terminal arrives quoted for a
			// shell that is not involved here; see quote.go.
			target := unquoteInput(strings.TrimSpace(f.Value("dest")))
			if target == "" {
				target = defaultDest
			}
			return core.Push(transferConfirm(sh, h, target))
		},
	})
}

// transferConfirm is the last look before anything is copied: what is going where, and
// under which account.
func transferConfirm(sh *core.Shared, h sshcfg.Host, dest string) core.Screen {
	paths := Of(sh).Paths

	body := fmt.Sprintf("Transfer %s to:\n\n  %s\n  on %s (%s)\n\n%s",
		plural(len(paths), "item"), dest, h.Alias, h.Display(), quoteJoin(paths))

	return components.CreateConfirmScreen(components.ConfirmSimple{
		Title: "Confirm transfer",
		Crumb: "Confirm",
		Text:  body,
		OnYesLambda: func(sh *core.Shared) core.Action {
			return core.Replace(transferTask(sh, h, dest))
		},
	})
}

// transferTask streams the copy into the log pane. It stays on the log when finished so
// the user can read what happened before dismissing it.
func transferTask(sh *core.Shared, h sshcfg.Host, dest string) core.Screen {
	c := Of(sh)
	paths := append([]string(nil), c.Paths...) // snapshot: the task outlives this call
	// Same reason ssh gets -F: scp must resolve the alias against the file we parsed.
	base := c.SSHArgs()

	run := func(ctx context.Context, sh *core.Shared, report func(string, ...any), done chan<- core.TaskEvent) {
		done <- runTransfer(ctx, h, dest, paths, base, report)
	}

	return components.NewStayTask(
		"transferring "+plural(len(paths), "item")+" to "+h.Alias,
		"transfer finished — esc to go back",
		run,
		// The task reports everything through the log, so the terminating event needs no
		// handling; dismissing returns to the host menu rather than the transfer form.
		func(*core.Shared, core.TaskEvent) core.Action { return core.Action{} },
		func(*core.Shared) core.Action { return core.PopTo() },
	)
}

// runTransfer does the work: check the host is up, create the destination, then scp each
// path. It returns the terminating TaskEvent.
//
// This is the simple flow (ssh_quick_transfer.py) — no per-file existence checks, so scp's
// own overwrite behavior applies. The Skip/Overwrite/Skip-All prompts from
// ssh_transfer_files.py are deliberately not ported in this pass.
func runTransfer(ctx context.Context, h sshcfg.Host, dest string, paths, base []string, report func(string, ...any)) core.TaskEvent {
	// Fail before copying rather than letting each scp time out in turn.
	if probe(h) != Up {
		report("%s is not reachable on port %s", h.HostName, h.Port)
		return core.TaskEvent{Done: true}
	}

	args := func(extra ...string) []string { return append(append([]string(nil), base...), extra...) }

	report("creating %s on %s", dest, h.Alias)
	abs, err := resolveDest(ctx, report, args(h.Target(), mkdirAndPwd(dest)))
	if err != nil {
		report("could not create destination: %v", err)
		return core.TaskEvent{Done: true}
	}
	report("destination is %s", abs)

	// scp splits this at the colon and sends the remainder over SFTP as a literal path, so
	// the target goes in unquoted — quoting it is what used to write files named 'Desktop'.
	// resolveDest already expanded the tilde, which that protocol would not have either.
	remote := h.Target() + ":" + abs
	var failed int
	for _, p := range paths {
		if ctx.Err() != nil {
			report("aborted")
			return core.TaskEvent{Done: true}
		}

		info, err := os.Stat(p)
		if err != nil {
			report("skipping %s: %v", p, err)
			failed++
			continue
		}
		// -r for a directory is the one branch the Python kept too (ssh_quick_transfer.py:31);
		// scp refuses a directory without it.
		scpArgs := args()
		if info.IsDir() {
			scpArgs = append(scpArgs, "-r")
		}
		scpArgs = append(scpArgs, p, remote)

		report("copying %s", p)
		if err := runStreaming(ctx, report, "scp", scpArgs...); err != nil {
			report("failed: %s (%v)", p, err)
			failed++
		}
	}

	switch {
	case ctx.Err() != nil:
		report("aborted")
	case failed > 0:
		report("finished with %s", plural(failed, "failure"))
	default:
		report("transferred %s to %s", plural(len(paths), "item"), remote)
	}
	return core.TaskEvent{Done: true}
}

// resolveDest creates the destination directory on the remote and reports back its
// absolute path, which is the form scp's SFTP target has to take (see quote.go). Asking the
// shell that already runs the mkdir to print where it landed costs nothing extra and leaves
// scp with a path that needs neither quoting nor tilde expansion.
func resolveDest(ctx context.Context, report func(string, ...any), args []string) (string, error) {
	cmd := exec.CommandContext(ctx, "ssh", args...)

	// Unlike runStreaming the two streams stay apart: only stdout carries the path, and
	// mistaking a login banner for it would send the whole transfer somewhere else. The
	// banner is still worth showing, so it goes to the log either way.
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	for _, line := range strings.Split(errBuf.String(), "\n") {
		if line = strings.TrimRight(line, "\r"); strings.TrimSpace(line) != "" {
			report("%s", line)
		}
	}
	if err != nil {
		return "", err
	}

	dest := lastLine(string(out))
	if dest == "" {
		return "", fmt.Errorf("remote reported no path")
	}
	return dest, nil
}

// lastLine is the final non-blank line of s, trimmed. It is the last line rather than the
// first because a remote shell's rc files can print on stdout before pwd does.
func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

// runStreaming runs a command, piping both output streams into the task log line by line,
// and honors ctx so esc genuinely kills an in-flight copy rather than just detaching the
// UI from it.
func runStreaming(ctx context.Context, report func(string, ...any), name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // scp reports progress and errors on stderr; one stream is enough

	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(pipe)
	for sc.Scan() {
		if line := strings.TrimRight(sc.Text(), "\r"); strings.TrimSpace(line) != "" {
			report("%s", line)
		}
	}
	return cmd.Wait()
}
