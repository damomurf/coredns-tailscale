package tailscale

import (
	"context"
	"net/netip"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/test"
	"tailscale.com/client/tailscale"
)

type Tailscale struct {
	Next     plugin.Handler
	tsClient tailscale.LocalClient
	entries  map[string][]netip.Addr
}

func NewTailscale(next plugin.Handler) *Tailscale {
	ts := Tailscale{Next: test.ErrorHandler()}
	ts.pollPeers()
	return &ts
}

// Name implements the Handler interface.
func (t *Tailscale) Name() string { return "tailscale" }

func (t *Tailscale) pollPeers() {
	t.entries = map[string][]netip.Addr{}

	res, err := t.tsClient.Status(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range res.Peer {
		for _, addr := range v.TailscaleIPs {
			t.entries[strings.ToLower(v.HostName)] = append(t.entries[strings.ToLower(v.HostName)], addr)
		}
	}
}
