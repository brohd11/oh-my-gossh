package main

import (
	"context"
	"fmt"
	"time"

	"github.com/brohd11/oh-my-gossh/internal/selfupdate"
)

// runUpdate implements the -update flag: check for a newer release and, when one
// exists, install it by running install.sh against the running binary's directory.
func runUpdate() error {
	// One budget for the whole operation: the redirect check is quick, the
	// download of the release zip is the slow part.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	info, err := selfupdate.Check(ctx, version)
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}
	if !info.Available {
		if version == "dev" {
			fmt.Println("dev build, skipping update")
		} else {
			fmt.Printf("gossh is up to date (%s)\n", version)
		}
		return nil
	}

	fmt.Printf("updating gossh %s → %s\n", version, info.LatestTag)
	report := func(format string, args ...any) { fmt.Printf(format+"\n", args...) }
	if err := selfupdate.Apply(ctx, info, report); err != nil {
		return err
	}
	fmt.Println("gossh updated to", info.LatestTag)
	return nil
}
