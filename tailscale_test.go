package tailscale

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestExample(t *testing.T) {
	// Create a new Example Plugin. Use the test.ErrorHandler as the next plugin.
	ts := Tailscale{Next: test.ErrorHandler()}

	ts.pollPeers()

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("devshell.murf.dev.", dns.TypeA)
	// Create a new Recorder that captures the result, this isn't actually used in this test
	// as it just serves as something that implements the dns.ResponseWriter interface.
	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	// Call our plugin directly, and check the result.
	ts.ServeDNS(ctx, rec, r)

	if rec.Rcode != 0 {
		t.Errorf("unexpected rcode, expected %d, got %d", 0, rec.Rcode)
	}
	if len(rec.Msg.Answer) == 0 {
		t.Errorf("unexpected answer count, expected 1 got %d", len(rec.Msg.Answer))
	}
}
