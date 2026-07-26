package app

import (
	"path/filepath"
	"strings"
)

// Everything that crosses a shell boundary goes through this file: the local line the
// detached-window launcher hands to `bash -c` (see windowArgv in open.go) and the remote
// command line ssh/scp hand to the far shell.

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

// shellJoin renders argv as one shell command line, every element quoted so the shell
// that parses it splits the words back exactly where they started.
func shellJoin(argv []string) string {
	out := make([]string, len(argv))
	for i, a := range argv {
		out[i] = shellQuote(a)
	}
	return strings.Join(out, " ")
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
