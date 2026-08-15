package app

import (
	"path/filepath"
	"strings"

	"github.com/brohd11/bubblestack/sysopen"
)

// Everything that crosses a shell boundary goes through this file: the local line the
// detached-window launcher hands to `bash -c` (see windowArgv in open.go) and the remote
// command line ssh/scp hand to the far shell. The POSIX single-quoting itself lives in
// bubblestack/sysopen (sysopen.ShellQuote/ShellJoin, shared with the terminal launcher); the
// ssh-specific tilde handling stays here.

// quoteRemotePath quotes a path that ssh/scp will hand to the *remote* shell, leaving a
// leading ~ or ~user prefix unquoted so that shell still expands it.
//
// Plain sysopen.ShellQuote is wrong here: it produces '~/Desktop', and a tilde inside single
// quotes is a literal, so the transfer would create a directory actually named "~" in the
// remote home. Only the segments after the tilde need protecting.
//
// The Python built remote commands by interpolating straight into f-strings, which is how
// ssh_transfer_files.py:65 ended up emitting `'[ -e /path' ]` — a misplaced quote that made
// every existence check report the same answer. Anything crossing a shell boundary goes
// through proper quoting instead.
func quoteRemotePath(p string) string {
	if p == "" || p[0] != '~' {
		return sysopen.ShellQuote(p)
	}
	slash := strings.IndexByte(p, '/')
	if slash < 0 {
		return p // bare ~ or ~user: nothing left to quote
	}
	return p[:slash+1] + sysopen.ShellQuote(p[slash+1:])
}

// quoteJoin renders a path list for display (confirm dialogs, headers) as the base names
// joined with ", ". Nothing here is quoted — display only; real shell quoting happens in
// quoteRemotePath via sysopen.ShellQuote.
func quoteJoin(paths []string) string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	return strings.Join(out, ", ")
}
