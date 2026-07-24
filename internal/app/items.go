package app

import (
	"strconv"

	"github.com/brohd11/go-ssh/internal/sshcfg"

	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
)

// Row shortcuts, so the common operations are one keystroke from the host list rather
// than a trip through the submenu.
var keys = struct {
	Open, Window, Power, Transfer key.Binding
}{
	Open:     key.NewBinding(key.WithKeys("o")),
	Window:   key.NewBinding(key.WithKeys("w")),
	Power:    key.NewBinding(key.WithKeys("p")),
	Transfer: key.NewBinding(key.WithKeys("t")),
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
				return core.Push(powerOffConfirm(sh, h)), true
			case core.MatchKey(k, keys.Transfer):
				if len(Of(sh).Paths) == 0 {
					return core.SetStatus("nothing selected — launch with paths to transfer"), true
				}
				return core.Push(transferForm(sh, h)), true
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
func plural(n int, noun string) string {
	s := strconv.Itoa(n) + " " + noun
	if n != 1 {
		s += "s"
	}
	return s
}
