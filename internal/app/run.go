package app

import (
	"github.com/brohd11/bubblestack"
	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"
)

// Run launches the go-ssh TUI: a single host-list tab (so bubblestack draws no tab strip),
// the per-host operation menus reached from it, the persistent header, a log pane the
// transfer task streams into, and a status line. paths is the argv selection — empty is a
// supported mode, not an error. configPath overrides ~/.ssh/config when non-empty.
func Run(paths []string, configPath string) error {
	return bubblestack.Run(bubblestack.Config{
		App:    New(paths, configPath),
		Header: Header,
		Output: components.NewLogPane(),
		Status: components.NewStatusLine(),
		Tabs: []bubblestack.TabEntry{
			{Title: "Hosts", New: NewHostsScreen},
		},
		// The startup sweep runs asynchronously, so the list is interactive immediately
		// and the reachability markers fill in when the dials return.
		Init: sweepReachability,
		RefreshAction: func(sh *core.Shared) core.Action {
			return refreshAction(sh, configPath)
		},
	})
}
