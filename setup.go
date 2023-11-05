package tailscale

import (
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

// init registers this plugin.
func init() { plugin.Register("tailscale", setup) }

// setup is the function that gets called when the config parser see the token "example". Setup is responsible
// for parsing any extra options the example plugin may have. The first token this function sees is "example".
func setup(c *caddy.Controller) error {
	ts := &Tailscale{}
	for c.Next() {
		args := c.RemainingArgs()
		if len(args) != 1 {
			return plugin.Error("tailscale", c.ArgErr())
		}
		ts.zone = args[0]

		for c.NextBlock() {
			switch c.Val() {
			case "authkey":
				args := c.RemainingArgs()
				if len(args) != 1 {
					return plugin.Error("tailscale", c.ArgErr())
				}
				ts.authkey = args[0]
			case "fallthrough":
				ts.fall.SetZonesFromArgs(c.RemainingArgs())
			default:
				return plugin.Error("tailscale", c.ArgErr())
			}
		}
	}

	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		ts.next = next
		if err := ts.start(); err != nil {
			log.Error(err)
			return nil
		}
		return ts
	})

	// All OK, return a nil error.
	return nil
}
