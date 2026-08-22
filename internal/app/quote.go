package app

import (
	"path/filepath"
	"strings"

	"github.com/brohd11/bubblestack/sysopen"
)

// Everything that crosses a shell boundary goes through this file: the local line the
// detached-window launcher hands to `bash -c` (see windowArgv in open.go) and the remote
// command line ssh hands to the far shell. The POSIX single-quoting itself lives in
// bubblestack/sysopen (sysopen.ShellQuote/ShellJoin, shared with the terminal launcher); the
// ssh-specific tilde handling stays here.
//
// scp is deliberately not on that list. Since OpenSSH 9.0 it transfers over the SFTP
// protocol, and the path after the colon is sent as a literal string no remote shell ever
// parses — scp(1) puts its "requires careful quoting" caveat on -O, the legacy SCP
// protocol, alone. Quoting an scp target therefore puts the quote characters *in the name*:
// `host:~/'Desktop'` wrote a file called 'Desktop', quotes included. The transfer resolves
// the destination to an absolute path first (mkdirAndPwd below, resolveDest in transfer.go)
// and hands scp that, unquoted.

// quoteRemotePath quotes a path that ssh will hand to the *remote* shell, leaving a
// leading ~ or ~user prefix unquoted so that shell still expands it.
//
// Plain sysopen.ShellQuote is wrong here: it produces '~/Desktop', and a tilde inside single
// quotes is a literal, so the mkdir would create a directory actually named "~" in the
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

// mkdirAndPwd is the remote command that creates the transfer destination and then prints
// where it landed. The `cd` and `pwd` are what turn ~/Desktop into an absolute path, which
// is the only form scp's SFTP target can carry — see the file comment above.
//
// `&&`, `mkdir -p`, `cd` and `pwd` are the common subset of sh, bash, zsh, csh and fish, so
// this does not assume what the remote login shell is.
func mkdirAndPwd(dest string) string {
	q := quoteRemotePath(dest)
	return "mkdir -p " + q + " && cd " + q + " && pwd"
}

// unquoteInput strips one matching pair of surrounding quotes from a value typed into a
// form field. A destination pasted out of a terminal or a file manager's "copy as path"
// commonly arrives as "~/My Folder", and those are the user quoting for a shell that is not
// involved: kept, they become part of the directory's name and the ~ stops expanding.
//
// sshcfg.unquote (config.go) does the same thing for ssh config values and stays separate —
// that one answers to the config file's grammar, this one to what people paste.
func unquoteInput(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
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
