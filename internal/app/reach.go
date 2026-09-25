package app

import (
	"net"
	"sync"
	"time"

	"github.com/brohd11/goutil/strutil"
	"github.com/brohd11/oh-my-gossh/internal/sshcfg"

	tea "charm.land/bubbletea/v2"
	"github.com/brohd11/bubblestack/core"
)

// Reachability is a host's last probe result.
type Reachability int

const (
	Unknown Reachability = iota // never probed, or a sweep is in flight
	Up                          // the ssh port accepted a connection
	Down                        // the dial failed or timed out
)

// Marker is the glyph appended to a host's list row. Unknown renders as nothing rather
// than a placeholder, so rows don't visibly churn while the first sweep lands.
func (r Reachability) Marker() string {
	switch r {
	case Up:
		return " ●"
	case Down:
		return " ○"
	}
	return ""
}

// dialTimeout bounds a single probe.
const dialTimeout = 2 * time.Second

// probe reports whether a host's ssh port accepts a TCP connection (sshd up, not just ICMP).
func probe(h sshcfg.Host) Reachability {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(h.HostName, h.Port), dialTimeout)
	if err != nil {
		return Down
	}
	conn.Close()
	return Up
}

// sweepReachability probes every host concurrently and broadcasts once, so the list rebuilds
// a single time.
func sweepReachability(sh *core.Shared) tea.Cmd {
	c := Of(sh)
	hosts := c.Hosts
	if len(hosts) == 0 {
		return nil
	}
	return func() tea.Msg {
		var wg sync.WaitGroup
		for _, h := range hosts {
			wg.Add(1)
			go func(h sshcfg.Host) {
				defer wg.Done()
				c.setReach(h.Alias, probe(h))
			}(h)
		}
		wg.Wait()
		return core.PropagateAll(ReachMsg{}).Msg
	}
}

// ReachMsg is the broadcast raised when a sweep completes: the host list re-reads the
// results and rebuilds its rows with fresh markers.
type ReachMsg struct{}

// HostsMsg is the broadcast raised when the config is reloaded, so the list rebuilds
// from the new host set.
type HostsMsg struct{}

// refreshAction is the global Refresh key: reload the ssh config, rebuild the rows from
// it immediately, and kick off a fresh sweep whose result broadcasts again when it lands.
func refreshAction(sh *core.Shared, configPath string) core.Action {
	c := Of(sh)
	c.Load(configPath)
	if c.LoadErr != nil {
		return core.SetStatusAndLog("config: " + c.LoadErr.Error())
	}
	return core.Seq(
		core.SetStatus("reloaded "+strutil.Count(len(c.Hosts), "host")+" from "+c.ConfigPath),
		core.PropagateAll(HostsMsg{}),
		core.Async(sweepReachability(sh)),
	)
}
