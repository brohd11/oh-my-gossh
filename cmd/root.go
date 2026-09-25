package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/brohd11/oh-my-gossh/internal/app"

	"github.com/spf13/cobra"
)

// version is stamped by the makefile via -X ldflags; "dev" for a plain go build.
var version = "dev"

var configPath string

// cobra prints Long verbatim, so the nemo placeholder %F needs no escaping.
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

// resolvePaths makes the arguments absolute, dropping (and returning) unreadable ones so a
// stale file-manager selection doesn't block startup.
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
