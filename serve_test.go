package tailscale

import (
	"context"
	"net"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
)

var ts = Tailscale{
	zone: "example.com",
	entries: map[string]map[string]string{
		"test1": {
			"A":    "127.0.0.1",
			"AAAA": "::1",
		},
		"test2": {
			"CNAME": "test1.example.com",
		},
	},
}

func TestServeDNS(t *testing.T) {
	test3 := net.ParseIP("100.100.100.100")

	// No match, no next plugin.
	var msg dns.Msg
	msg.SetQuestion("test3.example.com", dns.TypeA)
	resp, err := ts.ServeDNS(context.Background(), dnstest.NewRecorder(&test.ResponseWriter{}), &msg)
	if err == nil {
		t.Fatal("expected error, got none")
	}
	if want, got := dns.RcodeServerFailure, resp; got != want {
		t.Fatalf("want response code %d, got %d", want, got)
	}

	ts.Next = plugin.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: "test3.example.com", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   test3,
		})
		if err := w.WriteMsg(&msg); err != nil {
			return dns.RcodeServerFailure, err
		}
		return dns.RcodeSuccess, nil
	})

	// Match, next plugin configured.
	msg.SetQuestion("test1.example.com", dns.TypeA)
	w := dnstest.NewRecorder(&test.ResponseWriter{})
	resp, err = ts.ServeDNS(context.Background(), w, &msg)
	if want, got := dns.RcodeSuccess, resp; got != want {
		t.Fatalf("want response code %d, got %d", want, got)
	}
	if want, got := net.ParseIP("127.0.0.1"), w.Msg.Answer[0].(*dns.A).A; !got.Equal(want) {
		t.Errorf("want %s, got: %s", want, got)
	}

	// No match, next plugin configured.
	msg.SetQuestion("test3.example.com", dns.TypeA)
	w = dnstest.NewRecorder(&test.ResponseWriter{})
	ts.ServeDNS(context.Background(), w, &msg)

	if w.Msg == nil {
		t.Fatal("no answer")
	}
	if want, got := 1, len(w.Msg.Answer); want != got {
		t.Fatalf("want %d answer, got: %d", want, got)
	}
	if got := w.Msg.Answer[0].(*dns.A).A; !got.Equal(test3) {
		t.Errorf("want %s, got: %s", test3, got)
	}
}

func TestResolveA(t *testing.T) {

	msg := dns.Msg{}

	domain := "test1.example.com"

	ts.resolveA(domain, &msg)

	testEquals(t, "answer count", 1, len(msg.Answer))
	testEquals(t, "query name", domain, msg.Answer[0].Header().Name)

	if a, ok := msg.Answer[0].(*dns.A); ok {
		testEquals(t, "A record", "127.0.0.1", a.A.String())
	} else {
		t.Errorf("Expected AAAA return RR value type")
	}

}

func TestResolveAAAA(t *testing.T) {

	msg := dns.Msg{}

	domain := "test1.example.com"

	ts.resolveAAAA(domain, &msg)

	testEquals(t, "answer count", 1, len(msg.Answer))
	testEquals(t, "query name", domain, msg.Answer[0].Header().Name)

	if a, ok := msg.Answer[0].(*dns.AAAA); ok {
		testEquals(t, "A record", "::1", a.AAAA.String())
	} else {
		t.Errorf("Expected AAAA return RR value")
	}

}

func TestResolveCNAME(t *testing.T) {

	msg := dns.Msg{}
	domain := "test2.example.com"

	ts.resolveCNAME(domain, &msg, TypeAll)

	testEquals(t, "answer count", 3, len(msg.Answer))

	for _, rr := range msg.Answer {
		switch rec := rr.(type) {
		case *dns.CNAME:
			testEquals(t, "CNAME record", "test1.example.com", rec.Target)

		case *dns.A:
			testEquals(t, "A record", "127.0.0.1", rec.A.String())

		case *dns.AAAA:
			testEquals(t, "AAAA record", "::1", rec.AAAA.String())
		}

	}

}

func TestResolveAIsCNAME(t *testing.T) {

	msg := dns.Msg{}
	domain := "test2.example.com"

	ts.resolveA(domain, &msg)

	testEquals(t, "answer count", 2, len(msg.Answer))

	for _, rr := range msg.Answer {

		switch rec := rr.(type) {

		case *dns.CNAME:
			testEquals(t, "CNAME record", "test1.example.com", rec.Target)

		case *dns.A:
			testEquals(t, "A record", "127.0.0.1", rec.A.String())

		}

	}

}

func TestResolveAAAAIsCNAME(t *testing.T) {

	msg := dns.Msg{}
	domain := "test2.example.com"

	ts.resolveA(domain, &msg)

	testEquals(t, "answer count", 2, len(msg.Answer))

	for _, rr := range msg.Answer {

		switch rec := rr.(type) {

		case *dns.CNAME:
			testEquals(t, "CNAME record", "test1.example.com", rec.Target)

		case *dns.AAAA:
			testEquals(t, "AAAA record", "::1", rec.AAAA.String())

		}

	}

}

func testEquals(t *testing.T, msg string, expected interface{}, received interface{}) {

	if expected != received {
		t.Errorf("Expected %s %s: received %s", msg, expected, received)
	}

}
