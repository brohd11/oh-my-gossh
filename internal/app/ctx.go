package app

import (
	"path/filepath"
	"sync"

	"github.com/brohd11/oh-my-gossh/internal/sshcfg"

	"github.com/brohd11/bubblestack/core"
)

// Ctx is go-ssh's app context, stored on core.Shared.App and recovered with Of. It holds
// the hosts read from the ssh config, the paths handed in on argv (the nemo selection),
// and the last reachability sweep's results.
//
// Paths is the switch the whole UI turns on: an empty selection is a first-class mode
// where the transfer operations simply aren't offered.
type Ctx struct {
	Hosts      []sshcfg.Host
	ConfigPath string
	LoadErr    error

	// Version is the running binary's version, for the self-update check and the
	// Actions ▸ Update row. "dev" for an unstamped build, which never compares as
	// older than a release and so is never offered an update.
	Version string

	// override is the --config path, empty when reading the default ~/.ssh/config. It
	// is not just bookkeeping: see SSHArgs.
	override string

	// Paths are the argv-supplied files/directories to transfer. Empty ⇒ the launcher
	// was opened with no selection, so only the selection-free ops are shown.
	Paths []string

	// reach maps a host alias to its last probe result. Written by the sweep goroutine
	// and read by the list rows on the update loop, so it is mutex-guarded.
	mu    sync.RWMutex
	reach map[string]Reachability
}

// New builds the context and loads the ssh config, so the first screen has rows to show.
// A config that is missing or unreadable is not fatal — LoadErr is surfaced in the header
// and the list falls back to a placeholder row.
func New(paths []string, configPath, version string) *Ctx {
	c := &Ctx{Paths: paths, Version: version, reach: map[string]Reachability{}}
	c.Load(configPath)
	return c
}

// Of recovers the go-ssh context from a Shared. Screens call c := app.Of(sh).
func Of(sh *core.Shared) *Ctx { return core.App[Ctx](sh) }

// Load re-reads the ssh config. An explicit path (the --config flag) is used verbatim;
// an empty one falls back to ~/.ssh/config. A read error leaves the previous host list
// intact rather than blanking the screen.
func (c *Ctx) Load(configPath string) {
	var (
		hosts []sshcfg.Host
		path  string
		err   error
	)
	if configPath != "" {
		path = configPath
		hosts, err = sshcfg.LoadFile(configPath)
	} else {
		hosts, path, err = sshcfg.Load()
	}

	c.ConfigPath = path
	c.override = configPath
	c.LoadErr = err
	if err != nil {
		return // keep whatever we had rather than emptying the list
	}
	c.Hosts = hosts
}

// SSHArgs prefixes args with the options every ssh/scp invocation needs.
//
// The -F matters whenever --config was given: this tool passes the Host *alias* and
// lets ssh resolve the block itself, which only works if ssh reads the same file we
// parsed. Without it, aliases from a custom config fail with "Could not resolve
// hostname" because ssh consulted ~/.ssh/config instead. scp takes -F with the same
// meaning, so one helper serves both.
func (c *Ctx) SSHArgs(args ...string) []string {
	if c.override == "" {
		return args // the default path: ssh finds ~/.ssh/config on its own
	}
	return append([]string{"-F", c.override}, args...)
}

// Host looks up a parsed host by alias, reporting whether it is still present — a config
// reload between opening a menu and acting on it can drop one.
func (c *Ctx) Host(alias string) (sshcfg.Host, bool) {
	for _, h := range c.Hosts {
		if h.Alias == alias {
			return h, true
		}
	}
	return sshcfg.Host{}, false
}

// Reach reports the last probe result for a host (Unknown when never probed).
func (c *Ctx) Reach(alias string) Reachability {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reach[alias]
}

// setReach records a probe result. Called from the sweep goroutine.
func (c *Ctx) setReach(alias string, r Reachability) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.reach == nil {
		c.reach = map[string]Reachability{}
	}
	c.reach[alias] = r
}

// PathLabel is a one-line summary of the argv selection that names a lone file rather
// than counting it ("notes.txt", not "1 item").
//
// Nothing calls it today. The header and the transfer screens all want the plain count
// (plural(n, "item")) — the header would otherwise render "notes.txt: 'notes.txt'" — so
// this is the naming variant, kept with its test for a caller that wants it.
func (c *Ctx) PathLabel() string {
	switch len(c.Paths) {
	case 0:
		return "no selection"
	case 1:
		return filepath.Base(c.Paths[0])
	default:
		return plural(len(c.Paths), "item")
	}
}

// Receive handles app-level broadcasts. A theme change rebuilds the cached root so it
// re-bakes its list/delegate styles from the new palette (core.OnThemeChange); everything
// else is the screens' business.
func (c *Ctx) Receive(sh *core.Shared, payload any) core.Action {
	return core.OnThemeChange(payload)
}
