package app

import (
	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"

	"github.com/charmbracelet/bubbles/list"
)

// actionsMenu is the small Actions picker opened with "a" from the host list: switch the
// color theme, or refresh (reload the ssh config and re-sweep reachability). PopStop makes
// it the hub its sub-flows (the theme picker) return to.
func actionsMenu(sh *core.Shared) *components.PickerScreen {
	items := []list.Item{
		components.Item{
			Name: "◑ Theme",
			Desc: "switch the color theme",
			Pick: func(sh *core.Shared) core.Action { return core.Push(components.ThemePicker()) },
		},
		components.Item{
			Name: "⟳ Refresh",
			Desc: "reload the ssh config and re-probe reachability",
			// Same action the global "r" key runs: Of(sh).override is the --config path the
			// launcher was given (empty ⇒ ~/.ssh/config), captured on the last Load.
			Pick: func(sh *core.Shared) core.Action { return refreshAction(sh, Of(sh).override) },
		},
	}
	return components.NewPicker(items, components.PickerOpts{
		Title:   "Actions",
		Crumb:   "Actions",
		PopStop: true,
	})
}
