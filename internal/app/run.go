package app

import (
	"github.com/brohd11/bubblestack"
	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"

	tea "charm.land/bubbletea/v2"
)

// Run launches the go-ssh TUI: a single host-list tab (so bubblestack draws no tab strip),
// the per-host operation menus reached from it, the persistent header, a log pane the
// transfer task streams into, and a status line. paths is the argv selection — empty is a
// supported mode, not an error. configPath overrides ~/.ssh/config when non-empty, and
// version is the running binary's, for the self-update check.
func Run(paths []string, configPath, version string) error {
	return bubblestack.Run(bubblestack.Config{
		App:    New(paths, configPath, version),
		Header: Header,
		Output: components.NewLogPane(),
		Status: components.NewStatusLine(),
		Tabs: []bubblestack.TabEntry{
			{Title: "Hosts", New: NewHostsScreen},
		},
		// Both startup jobs run asynchronously, so the list is interactive immediately:
		// the reachability markers fill in when the dials return, and the release check
		// writes a line only when there is actually an update.
		Init: func(sh *core.Shared) tea.Cmd {
			return tea.Batch(sweepReachability(sh), SelfUpdateCheckCmd(sh))
		},
		RefreshAction: func(sh *core.Shared) core.Action {
			return refreshAction(sh, configPath)
		},
	})
}
