coredns-tailscale
=================

A CoreDNS plugin implementation for Tailscale networks.

Rationale
---------

Tailscale has some great built-in support for DNS and it keeps getting better. But there are some nice (if not purely cosmetic) reasons to be able to have Tailscale hosts resolve within your own existing domain. In addition, it's common for various services to run at a single Tailscale IP, for example, on a load balancer or web server hosting multiple virtual hosts.

Features
--------
This plugin for CoreDNS allows the following:

1. Automatically serving an (arbitrary) DNS zone with each Tailscale server in your Tailnet added with A and AAAA records.
1. Allowing CNAME records to be defined via Tailscale node tags that link logical names to Tailscale machines.


Configuration
-------------

```
example.com:53 {
  tailscale example.com
  log 
  errors
}
```
The above configuration will serve the connected Tailnet on the `example.com`. So, for a Tailnet with a machine named `test-machine`, A and AAAA records for `test-machine.example.com` will resolve.

CNAME records via Labels
------------------------

A CNAME record can be added to point to a machine by simply creating a Tailscale machine tag prefixed by `cname-`. Any text in the tag after that prefix will be used to generate the resulting CNAME entry, so for example, the tag `cname-friendly-name` on the above `test-machine` will result in the following DNS records:

```
friendly-name IN CNAME test-machine.example.com.
test-machine  IN A <Tailscale IPv4 Address>
test-machine  IN AAAA <Tailscale IPv6 Address>
```

Tailscale
---------
Note that currently this plugin uses the local machine Tailscale socket to access Tailnet information. As a result, only machines reachable from the hosting Tailscale machine will be configured in DNS. Those machines are the ones output in `tailscale status` output (and the machine itself). This was implemented to avoid the need for managing expiring Tailscale API tokens.


TODO
----
   * Update documentation to CoreDNS plugin [documentation standard](https://github.com/coredns/coredns/blob/master/plugin.md#documentation)
   * Add metrics support

