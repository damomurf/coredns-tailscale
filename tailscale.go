package tailscale

import (
	"context"
	"fmt"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/test"
	"tailscale.com/client/tailscale"
	"tailscale.com/ipn/ipnstate"
)

type Tailscale struct {
	Next     plugin.Handler
	tsClient tailscale.LocalClient
	entries  map[string]map[string][]string
	zone     string
}

func NewTailscale(next plugin.Handler) *Tailscale {
	ts := Tailscale{Next: test.ErrorHandler()}
	ts.pollPeers()
	return &ts
}

// Name implements the Handler interface.
func (t *Tailscale) Name() string { return "tailscale" }

func (t *Tailscale) pollPeers() {
	t.entries = map[string]map[string][]string{}

	res, err := t.tsClient.Status(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Debugf("Self tags: %+v", res.Self.Tags)

	// Add self to list of considered hosts
	hosts := []*ipnstate.PeerStatus{res.Self}

	// Add all peers to considered host list
	for _, status := range res.Peer {
		hosts = append(hosts, status)
	}

	for _, v := range hosts {
		// Process IPs for A and AAAA records
		for _, addr := range v.TailscaleIPs {
			entries, ok := t.entries[strings.ToLower(v.HostName)]
			if !ok {
				entries = map[string][]string{}
			}

			// Currently entries["A"/"AAAA"] will have max one element
			if addr.Is4() {
				entries["A"] = append(entries["A"], addr.String())
			} else if addr.Is6() {
				entries["AAAA"] = append(entries["AAAA"], addr.String())
			}

			t.entries[strings.ToLower(v.HostName)] = entries
		}

		// Process Tags looking for cname- prefixed ones
		if v.Tags != nil {
			for i := 0; i < v.Tags.Len(); i++ {
				raw := v.Tags.At(i)
				if strings.HasPrefix(raw, "tag:cname-") {
					tag := strings.TrimPrefix(raw, "tag:cname-")

					t.entries[tag] = map[string][]string{
						"CNAME": append(t.entries[tag]["CNAME"], fmt.Sprintf("%s.%s.", v.HostName, t.zone)),
					}
				}
			}
		}
	}
}
