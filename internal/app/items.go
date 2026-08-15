package app

import (
	"strconv"

	"github.com/brohd11/oh-my-gossh/internal/sshcfg"

	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"
	"github.com/brohd11/goutil/strutil"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
)

// Row shortcuts, so the common operations are one keystroke from the host list rather
// than a trip through the submenu. Actions is screen-level (not row-level) — it opens the
// Actions menu regardless of the highlighted row.
var keys = struct {
	Open, Window, Power, Transfer, Actions key.Binding
}{
	Open:     key.NewBinding(key.WithKeys("o")),
	Window:   key.NewBinding(key.WithKeys("w")),
	Power:    key.NewBinding(key.WithKeys("p")),
	Transfer: key.NewBinding(key.WithKeys("t")),
	Actions:  key.NewBinding(key.WithKeys("a")),
}

// NewHostsScreen builds the root: one row per host from the ssh config. It re-reads its
// rows on the reachability and config broadcasts, so a completed sweep or a Refresh
// repaints the markers without the user navigating.
func NewHostsScreen(sh *core.Shared) core.Screen {
	return components.NewPicker(hostItems(sh), components.PickerOpts{
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
		// OnKey owns the screen-level "a" (Actions). Because the picker consults OnKey
		// *instead of* the highlighted row's Item.Keys (they're mutually exclusive in
		// components/picker.go), OnKey must also delegate anything it doesn't claim back to
		// the row — otherwise the per-host o/w/p/t shortcuts (hostRow) would go dead.
		OnKey: func(sh *core.Shared, k string, it list.Item) (core.Action, bool) {
			if core.MatchKey(k, keys.Actions) {
				return core.Push(actionsMenu(sh)), true
			}
			if row, ok := it.(components.Item); ok && row.Keys != nil {
				return row.Keys(sh, k)
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
	})
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

// hostRow builds one list row: the alias plus its reachability marker as the name, the
// resolved address as the description, enter → the per-host operation menu, and the row's
// own shortcuts. Transfer is bound only when there is a selection to transfer.
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

// hostDesc is the row's subtitle: the address ssh will resolve to, plus the identity file
// when the block names one — that is the piece the Python's user@ip reconstruction lost.
func hostDesc(h sshcfg.Host) string {
	desc := h.Display()
	if h.ProxyJump != "" {
		desc += " via " + h.ProxyJump
	}
	return desc
}

// plural renders "1 host" / "3 hosts" — used in headers, confirms, and task labels.
// It formats the whole count, which is why it stays gossh's own: the shared
// strutil.Plural only picks the noun form, and does the "s" branch inside here.
func plural(n int, noun string) string {
	return strconv.Itoa(n) + " " + strutil.Plural(n, noun, noun+"s")
}
