package app

import (
	"github.com/brohd11/bubblestack/components"
	"github.com/brohd11/bubblestack/core"
	bsupdate "github.com/brohd11/bubblestack/selfupdate"

	tea "charm.land/bubbletea/v2"
)

// selfUpdateHooks points the shared self-update flow at gossh's release repo (also named
// in cmd/update.go).
func selfUpdateHooks(version string) components.SelfUpdateHooks {
	return bsupdate.Hooks("gossh", "brohd11/oh-my-gossh", version)
}

// SelfUpdateCheckCmd is the startup check; it reports only when an update is available.
func SelfUpdateCheckCmd(sh *core.Shared) tea.Cmd {
	return components.SelfUpdateCheckCmd(selfUpdateHooks(Of(sh).Version))
}
