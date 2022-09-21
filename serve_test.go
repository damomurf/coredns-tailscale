package tailscale

import (
	"testing"

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
