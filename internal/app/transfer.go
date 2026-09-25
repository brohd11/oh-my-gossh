package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/brohd11/oh-my-gossh/internal/sshcfg"

	"charm.land/bubbles/v2/key"
	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"
	"github.com/brohd11/goutil/stream"
	"github.com/brohd11/goutil/strutil"
)

// defaultDest is where a blank destination lands.
const defaultDest = "~/Desktop"

// transferForm asks for the destination directory (free text).
func transferForm(sh *core.Shared, h sshcfg.Host) core.Screen {
	// The label carries its own colon and trailing space: fieldRow lays the label
	// straight against the input with no separator of its own.
	dest := components.NewTextField("dest", "Destination: ", defaultDest)

	return components.NewForm(components.FormOpts{
		Title: "Transfer to " + h.Alias,
		Crumb: "Transfer",
		Fields: []components.FormField{
			components.NewNote(strutil.Count(len(Of(sh).Paths), "item") + " → " + h.Display()),
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
		strutil.Count(len(paths), "item"), dest, h.Alias, h.Display(), quoteJoin(paths))

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
		"transferring "+strutil.Count(len(paths), "item")+" to "+h.Alias,
		"transfer finished — esc to go back",
		run,
		// The task reports everything through the log, so the terminating event needs no
		// handling; dismissing returns to the host menu rather than the transfer form.
		func(*core.Shared, core.TaskEvent) core.Action { return core.Action{} },
		func(*core.Shared) core.Action { return core.PopTo() },
	)
}

// runTransfer checks the host, creates the destination and scps each path. There are no
// per-file existence checks; scp's overwrite behavior applies.
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

	// Unquoted: scp sends the remainder over SFTP literally (see quote.go).
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
		// scp refuses a directory without -r.
		scpArgs := args()
		if info.IsDir() {
			scpArgs = append(scpArgs, "-r")
		}
		scpArgs = append(scpArgs, p, remote)

		report("copying %s", p)
		if err := stream.Cmd(ctx, "", nil, report, append([]string{"scp"}, scpArgs...)...); err != nil {
			report("failed: %s (%v)", p, err)
			failed++
		}
	}

	switch {
	case ctx.Err() != nil:
		report("aborted")
	case failed > 0:
		report("finished with %s", strutil.Count(failed, "failure"))
	default:
		report("transferred %s to %s", strutil.Count(len(paths), "item"), remote)
	}
	return core.TaskEvent{Done: true}
}

// resolveDest creates the remote destination and returns its absolute path, the form scp's
// SFTP target needs.
func resolveDest(ctx context.Context, report func(string, ...any), args []string) (string, error) {
	cmd := exec.CommandContext(ctx, "ssh", args...)

	// Keep the streams apart: only stdout carries the path, and a login banner on it would
	// redirect the transfer. The banner still goes to the log.
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
