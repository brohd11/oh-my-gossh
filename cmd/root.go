package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/brohd11/oh-my-gossh/internal/app"

	"github.com/spf13/cobra"
)

// version is the binary version; defaults to "dev" for a plain `go build`. The makefile stamps
// it via -X ldflags (git describe --tags --always --dirty), so release and `make` binaries report
// their real version and the self-update check can compare it against the latest tag.
var version = "dev"

var configPath string

// The nemo placeholder %F in the Long text is a literal percent-sign to the user; cobra prints
// Long verbatim (no fmt formatting), so it needs no escaping — it only ever reads as a format
// directive to vet when passed through a fmt function.
var rootCmd = &cobra.Command{
	Use:   "gossh [flags] [paths...]",
	Short: "Pick a host from your ssh config and act on it",
	Long: `gossh — pick a host from your ssh config and act on it

Positional arguments are the files and directories to offer for transfer, as passed by a
file-manager action (nemo's %F). With none, only the operations that need no selection are
shown.

  gossh                       # open, power off
  gossh ~/notes.txt ~/photos  # the above, plus transferring those two
  gossh update                # update to the latest release`,
	Version:       version,
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE:          runRoot,
}

func init() {
	rootCmd.SetVersionTemplate("gossh {{.Version}}\n")
	rootCmd.Flags().StringVar(&configPath, "config", "", "ssh config file to read (default ~/.ssh/config)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	paths, skipped := resolvePaths(args)
	for _, s := range skipped {
		fmt.Fprintln(os.Stderr, "skipping:", s)
	}
	return app.Run(paths, configPath, version)
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
