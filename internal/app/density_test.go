package app

import (
	"strings"
	"testing"

	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"
	"github.com/brohd11/oh-my-gossh/internal/sshcfg"

	tea "charm.land/bubbletea/v2"
)

func TestHostRootDensityRefreshAndShortcuts(t *testing.T) {
	c := &Ctx{Hosts: []sshcfg.Host{{Alias: "demo", HostName: "example.invalid"}}}
	sh := core.NewShared(c)
	r := core.NewRouter(sh, []core.TabEntry{{Title: "Hosts", New: NewHostsScreen}})
	step := func(msg tea.Msg) {
		m, _ := r.Update(msg)
		r = m.(core.Router)
	}
	letter := func(k rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: k, Text: string(k)} }
	step(tea.WindowSizeMsg{Width: 80, Height: 24})
	root := r.Top().(*components.RootListScreen)
	step(letter('D'))
	step(tea.KeyPressMsg{Code: tea.KeyEnter})
	menu, ok := r.Top().(*components.PickerScreen)
	if !ok || !menu.Compact() {
		t.Fatal("host menu must inherit density")
	}
	step(letter('D'))
	step(tea.KeyPressMsg{Code: tea.KeyEscape})
	if r.Top() != root || root.Compact() || c.ListCompact {
		t.Fatal("host-menu density must propagate back to the root")
	}
	root.List().SetFilterText("demo")
	c.setReach("demo", Up)
	step(core.PropagateAll(ReachMsg{}))
	if root.List().FilterValue() != "demo" || !strings.Contains(components.SelectedTitle(root.List()), Up.Marker()) {
		t.Fatal("reachability refresh must retain populated filters and update markers")
	}
	c.Hosts[0].HostName = "updated.invalid"
	step(core.PropagateAll(HostsMsg{}))
	if !strings.Contains(root.List().SelectedItem().(components.Item).Desc, "updated.invalid") {
		t.Fatal("config refresh must update host rows")
	}
	step(letter('p')) // opens confirmation only; no SSH command is executed
	if _, ok := r.Top().(*components.DialogScreen); !ok {
		t.Fatal("unhandled root keys must reach the host's power-confirm shortcut")
	}
}
