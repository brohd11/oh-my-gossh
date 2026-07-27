// Command gossh is a terminal picker for the hosts in your ssh config: open a shell,
// power a host off, or copy the paths you launched it with.
//
// It replaces a set of curses Python scripts driven from a Linux Mint nemo action, which
// passed the file-manager selection as arguments. That interface is preserved — any
// positional arguments are the paths to transfer — but the selection is now optional:
// with none, the transfer operations are simply not offered.
package main

import "github.com/brohd11/oh-my-gossh/cmd"

func main() {
	cmd.Execute()
}
