package tailscale

import (
	"context"
	"net"
	"strings"

	"github.com/miekg/dns"

	"github.com/coredns/coredns/plugin"
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

	name := strings.TrimSuffix(strings.ToLower(domainName), ".")
	entries, ok := t.entries[name]["A"]
	if ok {
		log.Debugf("Found an v4 entry after lookup for: %s", name)
		for _, entry := range entries {
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domainName, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP(entry),
			})
		}
	} else {
		// There's no A record, so see if a CNAME exists
		log.Debug("No v4 entry after lookup, so trying CNAME")
		t.resolveCNAME(domainName, msg, TypeA)
	}

}

func (t *Tailscale) resolveAAAA(domainName string, msg *dns.Msg) {

	name := strings.TrimSuffix(strings.ToLower(domainName), ".")
	entries, ok := t.entries[name]["AAAA"]
	if ok {
		log.Debugf("Found a v6 entry after lookup for: %s", name)
		for _, entry := range entries {
			msg.Answer = append(msg.Answer, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: domainName, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
				AAAA: net.ParseIP(entry),
			})
		}
	} else {
		// There's no AAAA record, so see if a CNAME exists
		log.Debug("No v6 entry after lookup, so trying CNAME")
		t.resolveCNAME(domainName, msg, TypeAAAA)
	}

}

func (t *Tailscale) resolveCNAME(domainName string, msg *dns.Msg, lookupType int) {

	name := strings.TrimSuffix(strings.ToLower(domainName), ".")
	targets, ok := t.entries[name]["CNAME"]
	if ok {
		log.Debugf("Found a CNAME entry after lookup for: %s", name)
		for _, target := range targets {
			msg.Answer = append(msg.Answer, &dns.CNAME{
				Hdr:    dns.RR_Header{Name: domainName, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60},
				Target: target,
			})

			// Resolve local zone A or AAAA records if they exist for the referenced target
			if lookupType == TypeAll || lookupType == TypeA {
				log.Debug("CNAME record found, lookup up local recursive A")
				t.resolveA(target, msg)
			}
			if lookupType == TypeAll || lookupType == TypeAAAA {
				log.Debug("CNAME record found, lookup up local recursive AAAA")
				t.resolveAAAA(target, msg)
			}
		}
	}

}

func (t *Tailscale) handleNoRecords(ctx context.Context, w dns.ResponseWriter, r *dns.Msg, msg *dns.Msg) (int, error) {
	if t.fall.Through(r.Question[0].Name) {
		log.Debug("falling through")
		return plugin.NextOrFailure(t.Name(), t.next, ctx, w, r)
	} else {
		log.Debugf("Writing response: %+v", msg)
		w.WriteMsg(msg)
		return dns.RcodeNameError, nil
	}
}

func (t *Tailscale) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	log.Debugf("Received request for name: %v", r.Question[0].Name)
	log.Debugf("Tailscale peers list has %d entries", len(t.entries))

	msg := dns.Msg{}
	msg.SetReply(r)
	msg.Authoritative = true

	name := r.Question[0].Name

	t.mu.RLock()
	switch r.Question[0].Qtype {
	case dns.TypeA:
		log.Debug("Handling A record lookup")
		t.resolveA(name, &msg)

	case dns.TypeAAAA:
		log.Debug("Handling AAAA record lookup")
		t.resolveAAAA(name, &msg)

	case dns.TypeCNAME:
		log.Debug("Handling CNAME record lookup")
		t.resolveCNAME(name, &msg, TypeAll)
	}
	defer t.mu.RUnlock()

	if len(msg.Answer) == 0 {
		return t.handleNoRecords(ctx, w, r, &msg)
	}

	// Export metric with the server label set to the current server handling the request.
	//requestCount.WithLabelValues(metrics.WithServer(ctx)).Inc()

	log.Debugf("Writing response: %+v", msg)
	w.WriteMsg(&msg)
	return dns.RcodeSuccess, nil
}
