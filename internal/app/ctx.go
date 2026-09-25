package app

import (
	"sync"

	"github.com/brohd11/oh-my-gossh/internal/sshcfg"

	"github.com/brohd11/bubblestack/core"
)

// Ctx is gossh's app context: the ssh config hosts, the argv paths and the last reachability
// sweep. With no Paths the transfer operations are hidden.
type Ctx struct {
	// ListCompact is the session density shared by standard roots and pickers.
	ListCompact bool

	Hosts      []sshcfg.Host
	ConfigPath string
	LoadErr    error

	// Version is the running binary's version; "dev" is never offered an update.
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

// New loads the ssh config. A load failure is shown in the header, not fatal.
func New(paths []string, configPath, version string) *Ctx {
	c := &Ctx{Paths: paths, Version: version, reach: map[string]Reachability{}}
	c.Load(configPath)
	return c
}

// Of recovers the go-ssh context from a Shared. Screens call c := app.Of(sh).
func Of(sh *core.Shared) *Ctx { return core.App[Ctx](sh) }

// Load re-reads the ssh config (configPath, else ~/.ssh/config). A read error keeps the
// previous host list.
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

// SSHArgs prefixes args with the options every ssh/scp call needs. -F is passed for
// --config so ssh resolves the alias from the same file we parsed.
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

// Receive rebuilds the cached root on a theme change.
func (c *Ctx) Receive(sh *core.Shared, payload any) core.Action {
	return core.OnThemeChange(payload)
}

// ListDensity opts standard lists into the app-wide session preference.
func (c *Ctx) ListDensity() *bool { return &c.ListCompact }
