package tailscale

import (
	"net/netip"
	"testing"

	"github.com/google/go-cmp/cmp"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"
	"tailscale.com/types/views"
)

func TestProcessStatus(t *testing.T) {
	ts := &Tailscale{zone: "example.com"}

	selfTags := views.SliceOf([]string{"tag:cname-app"})
	peerTags := views.SliceOf([]string{"tag:cname-app"})
	sharedTags := views.SliceOf([]string{"tag:cname-app"})
	status := &ipnstate.Status{
		Self: &ipnstate.PeerStatus{
			DNSName:      "self.example.ts.net.",
			TailscaleIPs: []netip.Addr{netip.MustParseAddr("100.0.0.1"), netip.MustParseAddr("fd7a:115c:a1e0::1")},
			Tags:         &selfTags,
		},
		Peer: map[key.NodePublic]*ipnstate.PeerStatus{
			key.NewNode().Public(): {
				DNSName:      "peer.example.ts.net.",
				TailscaleIPs: []netip.Addr{netip.MustParseAddr("100.0.0.2"), netip.MustParseAddr("fd7a:115c:a1e0::2")},
				Tags:         &peerTags,
			},
			key.NewNode().Public(): {
				DNSName:      "shared.example.ts.net.",
				TailscaleIPs: []netip.Addr{netip.MustParseAddr("100.0.0.3"), netip.MustParseAddr("fd7a:115c:a1e0::3")},
				ShareeNode:   true,
				Tags:         &sharedTags,
			},
			key.NewNode().Public(): {
				TailscaleIPs: []netip.Addr{netip.MustParseAddr("100.0.0.4"), netip.MustParseAddr("fd7a:115c:a1e0::4")},
			},
		},
	}
	services := map[tailcfg.ServiceName]tailcfg.ServiceDetails{
		"svc:vip-service": {
			Addrs: []netip.Addr{netip.MustParseAddr("100.0.0.5"), netip.MustParseAddr("fd7a:115c:a1e0::5")},
		},
	}

	want := map[string]map[string][]string{
		"self": {
			"A":    {"100.0.0.1"},
			"AAAA": {"fd7a:115c:a1e0::1"},
		},
		"peer": {
			"A":    {"100.0.0.2"},
			"AAAA": {"fd7a:115c:a1e0::2"},
		},
		"app": {
			"CNAME": {"self.example.com.", "peer.example.com."},
		},
		"vip-service": {
			"A":    {"100.0.0.5"},
			"AAAA": {"fd7a:115c:a1e0::5"},
		},
	}

	ts.processStatus(status, services)
	if !cmp.Equal(ts.entries, want) {
		t.Errorf("ts.entries = %v, want %v", ts.entries, want)
	}

	// now process another netmap with only self, and make sure peer is removed
	status.Peer = nil
	want = map[string]map[string][]string{
		"self": {
			"A":    {"100.0.0.1"},
			"AAAA": {"fd7a:115c:a1e0::1"},
		},
		"app": {
			"CNAME": {"self.example.com."},
		},
		"vip-service": {
			"A":    {"100.0.0.5"},
			"AAAA": {"fd7a:115c:a1e0::5"},
		},
	}
	ts.processStatus(status, services)
	if !cmp.Equal(ts.entries, want) {
		t.Errorf("ts.entries = %v, want %v", ts.entries, want)
	}
}
