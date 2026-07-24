package app

import (
	"strings"

	"github.com/brohd11/bubblestack/core"
)

// Header is the persistent context box: where the host list came from, and what the
// launcher was handed on argv. The selection line is the one a user checks before a
// transfer, so it names the items when there are few enough to fit.
func Header(sh *core.Shared) string {
	c := Of(sh)

	var b strings.Builder
	b.WriteString(core.Label("config") + " " + c.ConfigPath)
	b.WriteString("  " + core.Label("hosts") + " " + plural(len(c.Hosts), "host"))
	b.WriteString("\n" + core.Label("selection") + " " + selectionSummary(sh))
	return b.String()
}

// selectionSummary describes the argv paths, naming them while they fit the header width
// and falling back to a count when they don't.
func selectionSummary(sh *core.Shared) string {
	c := Of(sh)
	if len(c.Paths) == 0 {
		return "none — transfer operations are hidden"
	}

	names := quoteJoin(c.Paths)
	summary := plural(len(c.Paths), "item") + ": " + names
	if width := core.HeaderInnerWidth(sh.Width()); width > 0 && len(summary) > width {
		return plural(len(c.Paths), "item")
	}
	return summary
}
