package app

import (
	"github.com/brohd11/oh-my-gossh/internal/sshcfg"

	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"
	"github.com/charmbracelet/bubbles/list"
)

// hostMenu is the per-host operation hub, reached by picking a host. It is a PopStop
// boundary, so a sub-flow (the transfer form → confirm → task chain) returns here rather
// than to the host list.
//
// The Python was op-first: script_launcher picked the operation, then each script picked
// a target from the same dict. Host-first is what lets the menu vary by context — with no
// argv selection the transfer entry is simply absent, which is the requested behavior.
func hostMenu(sh *core.Shared, h sshcfg.Host) core.Screen {
	return components.NewPicker(hostMenuItems(sh, h), components.PickerOpts{
		Title:   "Host: " + h.Alias,
		Crumb:   h.Alias,
		PopStop: true,
		Refresh: func(sh *core.Shared, payload any) ([]list.Item, bool) {
			if _, ok := payload.(ReachMsg); ok {
				return hostMenuItems(sh, h), true
			}
			return nil, false
		},
	})
}

func hostMenuItems(sh *core.Shared, h sshcfg.Host) []list.Item {
	c := Of(sh)

	items := []list.Item{
		components.Item{
			Name: "Open shell",
			Desc: "attach ssh to this terminal — the menu returns when the session ends",
			Pick: func(sh *core.Shared) core.Action { return openInline(sh, h) },
		},
		components.Item{
			Name: "Open shell in a new window",
			Desc: "launch a detached terminal and keep this menu up",
			Pick: func(sh *core.Shared) core.Action { return openWindow(sh, h) },
		},
	}

	// The transfer entry exists only when the launcher was given something to send. With
	// no selection the menu is just the connect/power operations.
	if len(c.Paths) > 0 {
		items = append(items, components.Item{
			Name: "Transfer " + plural(len(c.Paths), "item") + "…",
			Desc: quoteJoin(c.Paths),
			Pick: func(sh *core.Shared) core.Action { return transferAction(sh, h) },
		})
	}

	items = append(items,
		components.Item{
			Name: "Power off",
			Desc: "run `" + poweroffCommand + "` over an ssh tty",
			Pick: func(sh *core.Shared) core.Action { return powerOffAction(sh, h) },
		},
		components.Item{
			Name: "Reachability",
			Desc: reachDesc(c.Reach(h.Alias), h),
			Pick: func(sh *core.Shared) core.Action {
				return core.Seq(core.SetStatus("probing "+h.Alias+"…"), core.Async(sweepReachability(sh)))
			},
		},
	)
	return items
}

// The per-host operations, as actions. Both ways into them — the host row's keyboard
// shortcuts (items.go) and this menu's rows — go through these, so a shortcut and its
// menu row cannot come to mean different things. openInline/openWindow are already
// single calls and are used directly.

// powerOffAction opens the shutdown confirm.
func powerOffAction(sh *core.Shared, h sshcfg.Host) core.Action {
	return core.Push(powerOffConfirm(sh, h))
}

// transferAction opens the transfer form, or says why it can't: with no argv selection
// there is nothing to send. The menu omits its Transfer row entirely in that case and so
// never trips the guard — it is here for the row shortcut, which is always bound.
func transferAction(sh *core.Shared, h sshcfg.Host) core.Action {
	if len(Of(sh).Paths) == 0 {
		return core.SetStatus("nothing selected — launch with paths to transfer")
	}
	return core.Push(transferForm(sh, h))
}

// powerOffConfirm gates the shutdown. It is the one irreversible operation here, so it
// never fires straight off a keystroke.
func powerOffConfirm(sh *core.Shared, h sshcfg.Host) core.Screen {
	return components.CreateConfirmScreen(components.ConfirmSimple{
		Title: "Power off " + h.Alias,
		Crumb: "Power off",
		Text: "Run `" + poweroffCommand + "` on " + h.Alias + " (" + h.Display() + ")?\n\n" +
			"The TUI suspends so a sudo password prompt is answerable.",
		OnYesLambda: func(sh *core.Shared) core.Action {
			return core.Seq(core.Pop(), powerOffInline(sh, h))
		},
	})
}

func reachDesc(r Reachability, h sshcfg.Host) string {
	switch r {
	case Up:
		return "port " + h.Port + " is open — enter to re-probe"
	case Down:
		return "no answer on port " + h.Port + " — enter to re-probe"
	}
	return "not probed yet — enter to probe"
}
