package tailscale

import (
	"context"
	"net"
	"strings"

	"github.com/miekg/dns"

	clog "github.com/coredns/coredns/plugin/pkg/log"
)

var log = clog.NewWithPlugin("tailscale")

const (
	TypeAll = iota
	TypeA
	TypeAAAA
)

// ServeDNS implements the plugin.Handler interface. This method gets called when tailscale is used
// in a Server.

func (t *Tailscale) resolveA(domainName string, msg *dns.Msg) {

	name := strings.Split(domainName, ".")[0]
	entry, ok := t.entries[name]["A"]
	if ok {
		log.Debug("Found an v4 entry after lookup")
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: domainName, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP(entry),
		})
	} else {
		// There's no A record, so see if a CNAME exists
		log.Debug("No v4 entry after lookup, so trying CNAME")
		t.resolveCNAME(domainName, msg, TypeA)
	}

}

func (t *Tailscale) resolveAAAA(domainName string, msg *dns.Msg) {

	name := strings.Split(domainName, ".")[0]
	entry, ok := t.entries[name]["AAAA"]
	if ok {
		log.Debug("Found a v6 entry after lookup")
		msg.Answer = append(msg.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: domainName, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
			AAAA: net.ParseIP(entry),
		})
	} else {
		// There's no AAAA record, so see if a CNAME exists
		log.Debug("No v6 entry after lookup, so trying CNAME")
		t.resolveCNAME(domainName, msg, TypeAAAA)
	}

}

func (t *Tailscale) resolveCNAME(domainName string, msg *dns.Msg, lookupType int) {

	name := strings.Split(domainName, ".")[0]
	target, ok := t.entries[name]["CNAME"]
	if ok {
		log.Debug("Found a CNAME entry after lookup")
		msg.Answer = append(msg.Answer, &dns.CNAME{
			Hdr:    dns.RR_Header{Name: domainName, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60},
			Target: target,
		})

		// Resolve local zone A or AAAA records if they exist for the referenced target
		if lookupType == TypeAll || lookupType == TypeA {
			t.resolveA(target, msg)
		}
		if lookupType == TypeAll || lookupType == TypeAAAA {
			t.resolveAAAA(target, msg)
		}
	}

}

func (t *Tailscale) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	log.Debugf("Received request for name: %v", r.Question[0].Name)
	log.Debugf("Tailscale peers list has %d entries", len(t.entries))

	msg := dns.Msg{}
	msg.SetReply(r)
	msg.Authoritative = true

	switch r.Question[0].Qtype {

	case dns.TypeA:
		log.Debug("Handling A record lookup")
		t.resolveA(r.Question[0].Name, &msg)

	case dns.TypeAAAA:
		log.Debug("Handling AAAA record lookup")
		t.resolveAAAA(r.Question[0].Name, &msg)

	case dns.TypeCNAME:
		log.Debug("Handling CNAME record lookup")
		t.resolveCNAME(r.Question[0].Name, &msg, TypeAll)

	}

	log.Debugf("Writing response: %+v", msg)
	w.WriteMsg(&msg)

	// Export metric with the server label set to the current server handling the request.
	//requestCount.WithLabelValues(metrics.WithServer(ctx)).Inc()

	// Call next plugin (if any).
	//return plugin.NextOrFailure(t.Name(), t.Next, ctx, w, r)
	return 0, nil
}
