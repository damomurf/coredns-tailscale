package tailscale

import (
	"context"
	"net"

	"github.com/miekg/dns"

	clog "github.com/coredns/coredns/plugin/pkg/log"
)

var log = clog.NewWithPlugin("tailscale")

// ServeDNS implements the plugin.Handler interface. This method gets called when example is used
// in a Server.
func (t *Tailscale) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	log.Debugf("Received request for name: %v", r.Question[0].Name)

	msg := dns.Msg{}
	msg.SetReply(r)

	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg.Authoritative = true
		entries, ok := t.entries[msg.Question[0].Name]
		if ok {
			for _, addr := range entries {
				if addr.Is4() {
					msg.Answer = append(msg.Answer, &dns.A{
						Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
						A:   net.ParseIP(addr.String()),
					})

				}
			}

		}
	case dns.TypeAAAA:
		msg.Authoritative = true
		entries, ok := t.entries[msg.Question[0].Name]
		if ok {
			for _, addr := range entries {
				if addr.Is6() {
					msg.Answer = append(msg.Answer, &dns.AAAA{
						Hdr:  dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
						AAAA: net.ParseIP(addr.String()),
					})

				}
			}

		}

	}
	w.WriteMsg(&msg)

	// Export metric with the server label set to the current server handling the request.
	//requestCount.WithLabelValues(metrics.WithServer(ctx)).Inc()

	// Call next plugin (if any).
	//return plugin.NextOrFailure(t.Name(), t.Next, ctx, w, r)
	return 0, nil
}
