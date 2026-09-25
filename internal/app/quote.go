package app

import (
	"path/filepath"
	"strings"

	"github.com/brohd11/bubblestack/sysopen"
)

// Shell quoting for local `bash -c` lines and remote ssh commands. scp targets are never
// quoted: since OpenSSH 9.0 scp uses SFTP and sends the path literally, so quotes would end
// up in the filename. Transfers resolve the destination to an absolute path first.

// quoteRemotePath quotes a path for the remote shell but leaves a leading ~ or ~user
// unquoted so it still expands.
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

// mkdirAndPwd creates the destination and prints its absolute path (scp's SFTP target
// cannot expand ~). The syntax is common to sh, bash, zsh, csh and fish.
func mkdirAndPwd(dest string) string {
	q := quoteRemotePath(dest)
	return "mkdir -p " + q + " && cd " + q + " && pwd"
}

// unquoteInput strips one matching pair of surrounding quotes from pasted input.
func unquoteInput(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// quoteJoin joins base names with ", " for display only.
func quoteJoin(paths []string) string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	return strings.Join(out, ", ")
}
