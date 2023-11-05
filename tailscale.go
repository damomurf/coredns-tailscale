package tailscale

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"tailscale.com/client/tailscale"
	"tailscale.com/ipn"
	"tailscale.com/tailcfg"
	"tailscale.com/tsnet"
	"tailscale.com/types/netmap"
)

type Tailscale struct {
	next plugin.Handler
	zone string
	fall fall.F

	authkey string
	srv     *tsnet.Server
	lc      *tailscale.LocalClient

	mu      sync.RWMutex
	entries map[string]map[string][]string
}

// Name implements the Handler interface.
func (t *Tailscale) Name() string { return "tailscale" }

// start connects the Tailscale plugin to a tailscale daemon and populates DNS entries for nodes in the tailnet.
// DNS entries are automatically kept up to date with any node changes.
//
// If t.authkey is non-empty, this function uses that key to connect to the Tailnet using a tsnet server
// instead of connecting to the local tailscaled instance.
func (t *Tailscale) start() error {
	if t.authkey != "" {
		// authkey was provided, so startup a local tsnet server
		t.srv = &tsnet.Server{
			Hostname: "coredns",
			AuthKey:  t.authkey,
			Logf:     log.Debugf,
		}
		err := t.srv.Start()
		if err != nil {
			return err
		}
		t.lc, err = t.srv.LocalClient()
		if err != nil {
			return err
		}
	} else {
		// zero value LocalClient will connect to local tailscaled
		t.lc = &tailscale.LocalClient{}
	}

	go t.watchIPNBus()
	return nil
}

// watchIPNBus watches the Tailscale IPN Bus and updates DNS entries for any netmap update.
// This function does not return. If it is unable to read from the IPN Bus, it will continue to retry.
func (t *Tailscale) watchIPNBus() {
	for {
		watcher, err := t.lc.WatchIPNBus(context.Background(), ipn.NotifyInitialNetMap)
		if err != nil {
			log.Info("unable to read from Tailscale event bus, retrying in 1 minute")
			time.Sleep(1 * time.Minute)
			continue
		}
		defer watcher.Close()

		for {
			n, err := watcher.Next()
			if err != nil {
				// If we're unable to read, then close watcher and reconnect
				watcher.Close()
				break
			}
			t.processNetMap(n.NetMap)
		}
	}
}

func (t *Tailscale) processNetMap(nm *netmap.NetworkMap) {
	if nm == nil {
		return
	}

	log.Debugf("Self tags: %+v", nm.SelfNode.Tags().AsSlice())
	nodes := []tailcfg.NodeView{nm.SelfNode}
	nodes = append(nodes, nm.Peers...)

	entries := map[string]map[string][]string{}
	for _, node := range nodes {
		if node.IsWireGuardOnly() {
			// IsWireGuardOnly identifies a node as a Mullvad exit node.
			continue
		}
		if !node.Sharer().IsZero() {
			// Skip shared nodes, since they don't necessarily have unique hostnames within this tailnet.
			// TODO: possibly make it configurable to include shared nodes and figure out what hostname to use.
			continue
		}

		hostname := node.ComputedName()
		entry, ok := entries[hostname]
		if !ok {
			entry = map[string][]string{}
		}

		// Currently entry["A"/"AAAA"] will have max one element
		for _, pfx := range node.Addresses().AsSlice() {

			addr := pfx.Addr()
			if addr.Is4() {
				entry["A"] = append(entry["A"], addr.String())
			} else if addr.Is6() {
				entry["AAAA"] = append(entry["AAAA"], addr.String())
			}
		}

		// Process Tags looking for cname- prefixed ones
		if node.Tags().Len() > 0 {
			for _, raw := range node.Tags().AsSlice() {
				if tag, ok := strings.CutPrefix(raw, "tag:cname-"); ok {
					if _, ok := entries[tag]; !ok {
						entries[tag] = map[string][]string{}
					}
					entries[tag]["CNAME"] = append(entries[tag]["CNAME"], fmt.Sprintf("%s.%s.", hostname, t.zone))
				}
			}
		}

		entries[hostname] = entry
	}

	t.mu.Lock()
	t.entries = entries
	t.mu.Unlock()
	log.Debugf("updated %d Tailscale entries", len(entries))
}
