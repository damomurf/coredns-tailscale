package tailscale

import (
	"net/netip"
	"testing"

	"github.com/google/go-cmp/cmp"
	"tailscale.com/tailcfg"
	"tailscale.com/types/netmap"
)

func TestProcessNetMap(t *testing.T) {
	ts := &Tailscale{zone: "example.com"}

	self := (&tailcfg.Node{
		ComputedName: "self",
		Addresses: []netip.Prefix{
			netip.MustParsePrefix("100.0.0.1/24"),
			netip.MustParsePrefix("fd7a:115c:a1e0::1/128"),
		},
		Tags: []string{"tag:cname-app"},
	}).View()

	nm := &netmap.NetworkMap{
		SelfNode: self,
		Peers: []tailcfg.NodeView{
			(&tailcfg.Node{
				ComputedName: "peer",
				Addresses: []netip.Prefix{
					netip.MustParsePrefix("100.0.0.2/24"),
					netip.MustParsePrefix("fd7a:115c:a1e0::2/128"),
				},
				Tags: []string{"tag:cname-app"},
			}).View(),
			(&tailcfg.Node{
				// shared node should be excluded
				ComputedName: "shared",
				Sharer:       1,
				Addresses: []netip.Prefix{
					netip.MustParsePrefix("100.0.0.3/24"),
					netip.MustParsePrefix("fd7a:115c:a1e0::3/128"),
				},
				Tags: []string{"tag:cname-app"},
			}).View(),
			(&tailcfg.Node{
				// mullvad exit node should be excluded
				ComputedName:    "mullvad",
				IsWireGuardOnly: true,
				Addresses: []netip.Prefix{
					netip.MustParsePrefix("100.0.0.4/24"),
					netip.MustParsePrefix("fd7a:115c:a1e0::4/128"),
				},
				Tags: []string{"tag:cname-app"},
			}).View(),
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
	}

	ts.processNetMap(nm)
	if !cmp.Equal(ts.entries, want) {
		t.Errorf("ts.entries = %v, want %v", ts.entries, want)
	}

	// now process another netmap with only self, and make sure peer is removed
	ts.processNetMap(&netmap.NetworkMap{SelfNode: self})
	want = map[string]map[string][]string{
		"self": {
			"A":    {"100.0.0.1"},
			"AAAA": {"fd7a:115c:a1e0::1"},
		},
		"app": {
			"CNAME": {"self.example.com."},
		},
	}
	if !cmp.Equal(ts.entries, want) {
		t.Errorf("ts.entries = %v, want %v", ts.entries, want)
	}
}
