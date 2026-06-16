package tailscale

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/miekg/dns"
	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/tailcfg"
	"tailscale.com/tsnet"
	"tailscale.com/types/netmap"
)

type Tailscale struct {
	next plugin.Handler
	zone string
	fall fall.F

	authkey     string
	hostname    string
	srv         *tsnet.Server
	lc          *local.Client
	serveDNS    bool

	mu      sync.RWMutex
	entries map[string]map[string][]string

	dnsServer *dns.Server
	dnsConn   net.PacketConn
	dnsTCP    net.Listener
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

	if t.serveDNS {
		go func() {
			for i := 0; i < 30; i++ {
				ip4, ip6 := t.srv.TailscaleIPs()
				if ip4.IsValid() || ip6.IsValid() {
					if err := t.startDNSServer(); err != nil {
						log.Errorf("Failed to start DNS server: %v", err)
					}
					return
				}
				log.Debugf("Waiting for Tailscale IP assignment... (%d/30)", i+1)
				time.Sleep(1 * time.Second)
			}
			log.Error("Timeout waiting for Tailscale IP, DNS server not started")
		}()
	}

	return nil
}

// watchIPNBus watches the Tailscale IPN Bus and updates DNS entries for any netmap update.
// This function does not return. If it is unable to read from the IPN Bus, it will continue to retry.
func (t *Tailscale) watchIPNBus() {
	for {
		watcher, err := t.lc.WatchIPNBus(context.Background(), ipn.NotifyInitialNetMap|ipn.NotifyNoPrivateKeys)
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
			if n.NetMap != nil {
				t.processNetMap(n.NetMap)
			}
		}
	}
}

func (t *Tailscale) processNetMap(nm *netmap.NetworkMap) {

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

	// Grab service (VIP) definitions from the only place the API seems to return them accessible
	// via the netmap.
	for _, rec := range nm.DNS.ExtraRecords {
		name := strings.Split(rec.Name, ".")[0]
		ip, err := netip.ParseAddr(rec.Value)
		if err != nil {
			log.Errorf("Error parsing DNS extra record value \"%s\" as netip: %v", rec.Value, err)
		}
		if _, ok := entries[name]; !ok {
			entries[name] = map[string][]string{}
		}
		if ip.Is6() {
			entries[name]["AAAA"] = append(entries[name]["AAAA"], ip.String())
		} else {
			entries[name]["A"] = append(entries[name]["A"], ip.String())
		}
	}

	t.mu.Lock()
	t.entries = entries
	t.mu.Unlock()
	log.Debugf("updated %d Tailscale entries", len(entries))
}

func (t *Tailscale) startDNSServer() error {
	if !t.serveDNS {
		log.Debug("DNS server disabled by configuration")
		return nil
	}
	if t.srv == nil {
		log.Debug("DNS server not started: tsnet server not available")
		return nil
	}

	ip4, ip6 := t.srv.TailscaleIPs()
	if !ip4.IsValid() && !ip6.IsValid() {
		log.Debug("DNS server not started: no Tailscale IPs available yet")
		return nil
	}

	var bindAddr string
	if ip4.IsValid() {
		bindAddr = ip4.String()
	} else {
		bindAddr = ip6.String()
	}

	udpConn, err := t.srv.ListenPacket("udp", bindAddr+":53")
	if err != nil {
		return fmt.Errorf("failed to create UDP listener on %s:53: %w", bindAddr, err)
	}
	t.dnsConn = udpConn

	tcpLn, err := t.srv.Listen("tcp", bindAddr+":53")
	if err != nil {
		udpConn.Close()
		return fmt.Errorf("failed to create TCP listener on %s:53: %w", bindAddr, err)
	}
	t.dnsTCP = tcpLn

	t.dnsServer = &dns.Server{
		PacketConn: udpConn,
		Listener:   tcpLn,
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			t.ServeDNS(context.Background(), w, r)
		}),
	}

	go func() {
		if err := t.dnsServer.ActivateAndServe(); err != nil {
			log.Errorf("DNS server error: %v", err)
		}
	}()

	log.Infof("DNS server started on UDP/TCP port 53 (%s)", bindAddr)
	return nil
}
