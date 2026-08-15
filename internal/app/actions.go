package app

import (
	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"
)

// actionsMenu is the Actions picker opened with "a" from the host list. It is the shared
// bubblestack sheet — theme, self-update, refresh — rather than a hand-rolled copy, so a
// row added to the standard menu reaches gossh too. gossh ships no in-TUI manual, so the
// docs argument is nil and no Docs row appears; it has no app-specific rows either.
func actionsMenu(sh *core.Shared) *components.PickerScreen {
	return components.NewActionsMenu(
		selfUpdateHooks(Of(sh).Version),
		"reload the ssh config and re-probe reachability",
		// Same action the global "r" key runs: Of(sh).override is the --config path the
		// launcher was given (empty ⇒ ~/.ssh/config), captured on the last Load.
		func(sh *core.Shared) core.Action { return refreshAction(sh, Of(sh).override) },
		nil,
	)
}
