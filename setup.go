package tailscale

import (
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
)

// init registers this plugin.
func init() { plugin.Register("tailscale", setup) }

// setup is the function that gets called when the config parser see the token "example". Setup is responsible
// for parsing any extra options the example plugin may have. The first token this function sees is "example".
func setup(c *caddy.Controller) error {
	var zone string
	c.Next() // Ignore "tailscale" and give us the next token.
	if c.NextArg() {
		zone = c.Val()
	}

	f := fall.F{}

	for c.NextBlock() {
		switch c.Val() {
		case "fallthrough":
			f.SetZonesFromArgs(c.RemainingArgs())
		default:
			return c.Errf("unknown property '%s'", c.Val())
		}
	}
	if c.NextArg() {
		return plugin.Error("tailscale", c.ArgErr())
	}

	ts := &Tailscale{
		zone: zone,
		fall: f,
	}

	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		ts.Next = next
		ts.pollPeers()
		go func() {
			for range time.Tick(1 * time.Minute) {
				ts.pollPeers()
			}
		}()
		return ts
	})

	// All OK, return a nil error.
	return nil
}
