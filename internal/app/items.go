package app

import (
	"github.com/brohd11/oh-my-gossh/internal/sshcfg"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"
)

// Row shortcuts for the common operations. Actions is screen-level.
var keys = struct {
	Open, Window, Power, Transfer, Actions key.Binding
}{
	Open:     key.NewBinding(key.WithKeys("o")),
	Window:   key.NewBinding(key.WithKeys("w")),
	Power:    key.NewBinding(key.WithKeys("p")),
	Transfer: key.NewBinding(key.WithKeys("t")),
	Actions:  core.Keys.Actions,
}

// NewHostsScreen builds the root, one row per host, rebuilt on the reachability and config
// broadcasts.
func NewHostsScreen(sh *core.Shared) core.Screen {
	return components.NewRootList(hostItems(sh), components.RootListOpts{PickerOpts: components.PickerOpts{
		Title:   "SSH hosts",
		Crumb:   "Hosts",
		PopStop: true,
		Help: []key.Binding{
			core.Hint("open", keys.Open),
			core.Hint("window", keys.Window),
			core.Hint("power off", keys.Power),
			core.Hint("transfer", keys.Transfer),
			core.Hint("actions", keys.Actions),
		},
		// Unhandled app commands fall through to the highlighted host's shortcuts.
		OnKey: func(sh *core.Shared, k string, _ list.Item) (core.Action, bool) {
			if core.MatchKey(k, keys.Actions) {
				return core.Push(actionsMenu(sh)), true
			}
			return core.Action{}, false
		},
		Refresh: func(sh *core.Shared, payload any) ([]list.Item, bool) {
			switch payload.(type) {
			case ReachMsg, HostsMsg:
				return hostItems(sh), true
			}
			return nil, false
		},
	}})
}

// hostItems builds the list contents, or a single inert placeholder explaining why the
// list is empty — a missing config and an empty one are different problems.
func hostItems(sh *core.Shared) []list.Item {
	c := Of(sh)

	if c.LoadErr != nil {
		return []list.Item{components.Item{
			Name: "Could not read the ssh config",
			Desc: c.ConfigPath + ": " + c.LoadErr.Error(),
		}}
	}

	items := make([]list.Item, 0, len(c.Hosts))
	for _, h := range c.Hosts {
		items = append(items, hostRow(sh, h))
	}
	return components.EnsurePlaceholder(items, "No hosts in the ssh config",
		"add a Host block to "+c.ConfigPath+" — wildcard blocks like `Host *` are not targets")
}

// hostRow builds one row: alias plus reachability marker, the address as description, enter
// opens the host menu. Transfer is bound only with a selection.
func hostRow(sh *core.Shared, h sshcfg.Host) components.Item {
	return components.Item{
		Name:   h.Alias + Of(sh).Reach(h.Alias).Marker(),
		Desc:   hostDesc(h),
		Filter: h.Alias + " " + h.HostName,
		Pick:   func(sh *core.Shared) core.Action { return core.Push(hostMenu(sh, h)) },
		Keys: func(sh *core.Shared, k string) (core.Action, bool) {
			switch {
			case core.MatchKey(k, keys.Open):
				return openInline(sh, h), true
			case core.MatchKey(k, keys.Window):
				return openWindow(sh, h), true
			case core.MatchKey(k, keys.Power):
				return powerOffAction(sh, h), true
			case core.MatchKey(k, keys.Transfer):
				return transferAction(sh, h), true
			}
			return core.Action{}, false
		},
	}
}

// hostDesc is the row's subtitle: the resolved address, plus the identity file when set.
func hostDesc(h sshcfg.Host) string {
	desc := h.Display()
	if h.ProxyJump != "" {
		desc += " via " + h.ProxyJump
	}
	return desc
}
