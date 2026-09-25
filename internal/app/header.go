package app

import (
	"strings"

	"github.com/brohd11/bubblestack/core"
	"github.com/brohd11/goutil/strutil"
)

// Header shows the config source and the argv selection (named when few enough to fit).
func Header(sh *core.Shared) string {
	c := Of(sh)

	var b strings.Builder
	b.WriteString(core.Label("config") + " " + c.ConfigPath)
	b.WriteString("  " + core.Label("hosts") + " " + strutil.Count(len(c.Hosts), "host"))
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
	summary := strutil.Count(len(c.Paths), "item") + ": " + names
	if width := core.HeaderInnerWidth(sh.Width()); width > 0 && len(summary) > width {
		return strutil.Count(len(c.Paths), "item")
	}
	return summary
}
