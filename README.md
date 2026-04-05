# coredns-tailscale

A CoreDNS plugin implementation for Tailscale networks.

## Rationale

Tailscale has some great built-in support for DNS and it keeps getting better. But there are some nice (if not purely cosmetic) reasons to be able to have Tailscale hosts resolve within your own existing domain. In addition, it's common for various services to run at a single Tailscale IP, for example, on a load balancer or web server hosting multiple virtual hosts.

## Features

This plugin for CoreDNS allows the following:

1. Automatically serving an (arbitrary) DNS zone with each Tailscale server in your Tailnet added with A and AAAA records.
1. Allowing CNAME records to be defined via Tailscale node tags that link logical names to Tailscale machines.
1. Exposing [Tailscale Services](https://tailscale.com/docs/features/tailscale-services) entries


## Configuration

The configurations below will serve the connected Tailnet on the `example.com` domain. So, for a Tailnet with a machine named `test-machine`, A and AAAA records for `test-machine.example.com` will resolve.

### Using a Tailscale AuthKey

```
example.com:53 {
  tailscale example.com {
    authkey <authkey-from-admin-console>
    hostname <hostname>
  }
}
```

Obtain an auth key from the Tailscale admin console at (https://login.tailscale.com/admin/settings/keys). The hostname field defines what Tailscale node this plugin will appear as in your Tailscale Machines list.

_Note that this auth key will expire_.

### Using the local Tailscale Socket

```
example.com:53 {
  tailscale example.com
  log 
  errors
}
```

This approach uses local machine Tailscale socket to access Tailnet information. As a result, only machines reachable from the hosting Tailscale machine will be configured in DNS. Those machines are the ones output in `tailscale status` output (and the machine itself).

If using a container to run coredns-tailscale, the Tailscale socket will need to be mounted in to the container for `coredns-tailscale` to be able to connect to it.

## CNAME records via Labels

A CNAME record can be added to point to a machine by simply creating a Tailscale machine tag prefixed by `cname-`. Any text in the tag after that prefix will be used to generate the resulting CNAME entry, so for example, the tag `cname-friendly-name` on the above `test-machine` will result in the following DNS records:

```
friendly-name IN CNAME test-machine.example.com.
test-machine  IN A <Tailscale IPv4 Address>
test-machine  IN AAAA <Tailscale IPv6 Address>
```

## Services

[Tailscale Services](https://tailscale.com/docs/features/tailscale-services) are an alternative way to expose underlying services to your Tailnet. `coredns-tailscale` will retrieve Services entries based on their name (without the Tailnet domain) and expose their corresponding `A` and `AAAA` records in the configured CoreDNS domain. 

## TODO

   * Support Tailscale API key to avoid expiring auth keys

