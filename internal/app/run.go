package app

import (
	"github.com/brohd11/bubblestack"
	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"

	tea "charm.land/bubbletea/v2"
)

// Run launches the gossh TUI. paths is the argv selection (may be empty); configPath
// overrides ~/.ssh/config.
func Run(paths []string, configPath, version string) error {
	return bubblestack.Run(bubblestack.Config{
		App:    New(paths, configPath, version),
		Header: Header,
		Output: components.NewLogPane(),
		Status: components.NewStatusLine(),
		Tabs: []bubblestack.TabEntry{
			{Title: "Hosts", New: NewHostsScreen},
		},
		// Both startup jobs are async so the list is interactive immediately.
		Init: func(sh *core.Shared) tea.Cmd {
			return tea.Batch(sweepReachability(sh), SelfUpdateCheckCmd(sh))
		},
		RefreshAction: func(sh *core.Shared) core.Action {
			return refreshAction(sh, configPath)
		},
	})
}
