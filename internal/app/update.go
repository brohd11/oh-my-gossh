package app

import (
	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"
	bsupdate "github.com/brohd11/bubblestack/selfupdate"

	tea "github.com/charmbracelet/bubbletea"
)

// selfUpdateRepo is gossh's own GitHub repo slug, passed to the shared self-update bridge.
// It is the repo name, not the binary name — the two differ here, and `gossh update`
// (cmd/update.go) resolves releases from this same slug.
const selfUpdateRepo = "brohd11/oh-my-gossh"

// selfUpdateHooks builds the shared self-update flow's (bubblestack/components) hook set
// for gossh. The goutil↔components wiring — the Check/Apply closures and the conversion
// between goutil's selfupdate.Info and the flow's app-agnostic SelfUpdateInfo
// (field-identical by design) — lives in the bubblestack/selfupdate bridge, which every
// app in the monorepo shares.
func selfUpdateHooks(version string) components.SelfUpdateHooks {
	return bsupdate.Hooks("gossh", selfUpdateRepo, version)
}

// SelfUpdateCheckCmd is the app-level startup command (wired onto bubblestack Config.Init):
// it checks gossh's own repo for a newer release off the UI thread and, only when an update
// is available, writes an "update available" line to the shared status line and log.
// Anything else (up to date, dev build, fetch error) is silent. The flow and timeout are the
// shared ones in bubblestack/components; only the hooks are gossh's.
func SelfUpdateCheckCmd(sh *core.Shared) tea.Cmd {
	return components.SelfUpdateCheckCmd(selfUpdateHooks(Of(sh).Version))
}
