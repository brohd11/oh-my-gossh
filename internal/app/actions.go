package app

import (
	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"
)

// actionsMenu is the shared Actions picker opened with "a".
func actionsMenu(sh *core.Shared) *components.PickerScreen {
	return components.NewActionsMenu(
		selfUpdateHooks(Of(sh).Version),
		"reload the ssh config and re-probe reachability",
		// Same as the global "r" key; override is the --config path (empty ⇒ ~/.ssh/config).
		func(sh *core.Shared) core.Action { return refreshAction(sh, Of(sh).override) },
		nil,
	)
}
