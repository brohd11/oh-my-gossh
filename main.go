// Command go-ssh is a terminal picker for the hosts in your ssh config: open a shell,
// power a host off, or copy the paths you launched it with.
//
// It replaces a set of curses Python scripts driven from a Linux Mint nemo action, which
// passed the file-manager selection as arguments. That interface is preserved — any
// positional arguments are the paths to transfer — but the selection is now optional:
// with none, the transfer operations are simply not offered.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/brohd11/go-ssh/internal/app"
)

func main() {
	configPath := flag.String("config", "", "ssh config file to read (default ~/.ssh/config)")
	flag.Usage = usage
	flag.Parse()

	paths, skipped := resolvePaths(flag.Args())
	for _, s := range skipped {
		fmt.Fprintln(os.Stderr, "skipping:", s)
	}

	if err := app.Run(paths, *configPath); err != nil {
		fmt.Fprintln(os.Stderr, "go-ssh:", err)
		os.Exit(1)
	}
}

// usage writes the help text. io.WriteString rather than fmt.Fprint because the nemo
// placeholder %F reads as a format directive to vet.
func usage() {
	io.WriteString(flag.CommandLine.Output(), `go-ssh — pick a host from your ssh config and act on it

Usage:
  go-ssh [flags] [paths...]

Positional arguments are the files and directories to offer for transfer, as passed by a
file-manager action (nemo's %F). With none, only the operations that need no selection are
shown.

  go-ssh                       # open, power off
  go-ssh ~/notes.txt ~/photos  # the above, plus transferring those two

Flags:
`)
	flag.PrintDefaults()
}

// resolvePaths turns the raw arguments into absolute paths, dropping any that cannot be
// read and reporting them. A file manager can hand over a path that was deleted or
// unmounted between the selection and the launch; that is worth a note on stderr, not a
// refusal to start.
func resolvePaths(args []string) (paths, skipped []string) {
	for _, arg := range args {
		abs, err := filepath.Abs(arg)
		if err != nil {
			skipped = append(skipped, arg+": "+err.Error())
			continue
		}
		if _, err := os.Stat(abs); err != nil {
			skipped = append(skipped, arg+": "+err.Error())
			continue
		}
		paths = append(paths, abs)
	}
	return paths, skipped
}
