package tailscale

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/tsnet"
)

type Tailscale struct {
	next plugin.Handler
	zone string
	fall fall.F

	authkey  string
	hostname string
	srv      *tsnet.Server
	lc       *local.Client

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
		hostname := t.hostname
		if t.hostname == "" {
			hostname = "coredns"
		}
		// authkey was provided, so startup a local tsnet server
		t.srv = &tsnet.Server{
			Hostname:     hostname,
			AuthKey:      t.authkey,
			Logf:         log.Debugf,
			RunWebClient: true,
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
		t.lc = &local.Client{}
	}

	go t.watchIPNBus()
	return nil
}

// watchIPNBus watches the Tailscale IPN Bus and refreshes DNS entries when node state changes.
// This function does not return. If it is unable to read from the IPN Bus, it will continue to retry.
func (t *Tailscale) watchIPNBus() {
	for {
		watcher, err := t.lc.WatchIPNBus(context.Background(), ipn.NotifyInitialStatus|ipn.NotifyPeerChanges|ipn.NotifyNoPrivateKeys)
		if err != nil {
			log.Infof("unable to read from Tailscale event bus, retrying in 1 minute: %v", err)
			time.Sleep(1 * time.Minute)
			continue
		}

		for {
			_, err := watcher.Next()
			if err != nil {
				// If we're unable to read, then close watcher and reconnect
				watcher.Close()
				break
			}
			status, err := t.lc.Status(context.Background())
			if err != nil {
				log.Infof("unable to get Tailscale status: %v", err)
				continue
			}
			services, err := t.lc.GetServices(context.Background())
			if err != nil {
				// Services are optional (e.g. no netmap yet on a fresh tsnet node); still publish nodes.
				log.Debugf("unable to get Tailscale services: %v", err)
				services = nil
			}
			t.processStatus(status, services)
		}
	}
}

func (t *Tailscale) processStatus(status *ipnstate.Status, services map[tailcfg.ServiceName]tailcfg.ServiceDetails) {
	if status == nil {
		return
	}

	entries := map[string]map[string][]string{}
	nodes := []*ipnstate.PeerStatus{status.Self}
	for _, peer := range status.Peer {
		nodes = append(nodes, peer)
	}

	for _, node := range nodes {
		if node == nil || node.DNSName == "" || node.ShareeNode {
			// Shared nodes may not have unique hostnames; nodes without DNS names are unresolvable.
			continue
		}

		hostname := strings.SplitN(strings.TrimSuffix(node.DNSName, "."), ".", 2)[0]
		entry, ok := entries[hostname]
		if !ok {
			entry = map[string][]string{}
		}

		// Currently entry["A"/"AAAA"] will have max one element
		for _, addr := range node.TailscaleIPs {
			if addr.Is4() {
				entry["A"] = append(entry["A"], addr.String())
			} else if addr.Is6() {
				entry["AAAA"] = append(entry["AAAA"], addr.String())
			}
		}

		// Process Tags looking for cname- prefixed ones
		if node.Tags != nil {
			for _, raw := range node.Tags.AsSlice() {
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

	// Grab service (VIP) definitions
	for serviceName, svc := range services {
		name := serviceName.WithoutPrefix()
		if _, ok := entries[name]; !ok {
			entries[name] = map[string][]string{}
		}
		for _, addr := range svc.Addrs {
			if addr.Is6() {
				entries[name]["AAAA"] = append(entries[name]["AAAA"], addr.String())
			} else {
				entries[name]["A"] = append(entries[name]["A"], addr.String())
			}
		}
	}

	t.mu.Lock()
	t.entries = entries
	t.mu.Unlock()
	log.Debugf("updated %d Tailscale entries", len(entries))
}
