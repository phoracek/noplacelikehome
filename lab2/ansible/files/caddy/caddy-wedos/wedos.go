package wedos

// CaddyProvider is the thin Caddy DNS-module wrapper. It is adapted from
// github.com/caddy-dns/wedos@74e426d so that it lives in the same package as the
// vendored libdns provider (github.com/libdns/wedos@3946767). See NOTICE for the
// exact list of edits. Everything else in this package is verbatim upstream.

import (
	"fmt"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

// CaddyProvider lets Caddy read and manipulate DNS records hosted by this DNS provider.
type CaddyProvider struct{ *Provider }

func init() {
	caddy.RegisterModule(CaddyProvider{})
}

// CaddyModule returns the Caddy module information.
func (CaddyProvider) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "dns.providers.wedos",
		New: func() caddy.Module { return &CaddyProvider{new(Provider)} },
	}
}

// Provision sets up the module. Implements caddy.Provisioner.
func (p *CaddyProvider) Provision(ctx caddy.Context) error {
	repl := caddy.NewReplacer()
	p.Provider.Username = repl.ReplaceAll(p.Provider.Username, "")
	p.Provider.Password = repl.ReplaceAll(p.Provider.Password, "")
	if p.Provider.Username == "" || p.Provider.Password == "" {
		return fmt.Errorf("missing username and/or password")
	}

	return nil
}

// UnmarshalCaddyfile sets up the DNS provider from Caddyfile tokens. Syntax:
//
//	wedos {
//	    username {env.WEDOS_USERNAME}
//	    password {env.WEDOS_PASSWORD}
//	}
func (p *CaddyProvider) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if d.NextArg() {
			username := d.Val()
			if d.NextArg() {
				password := d.Val()
				p.Provider.Username = username
				p.Provider.Password = password
			} else {
				return d.ArgErr()
			}
		}

		if d.NextArg() {
			return d.ArgErr()
		}

		for nesting := d.Nesting(); d.NextBlock(nesting); {
			switch d.Val() {
			case "username":
				if p.Provider.Username != "" {
					return d.Err("Username already set")
				}

				if !d.NextArg() {
					return d.ArgErr()
				}

				p.Provider.Username = d.Val()
				if d.NextArg() {
					return d.ArgErr()
				}

			case "password":
				if p.Provider.Password != "" {
					return d.Err("Password already set")
				}

				if !d.NextArg() {
					return d.ArgErr()
				}

				p.Provider.Password = d.Val()
				if d.NextArg() {
					return d.ArgErr()
				}

			default:
				return d.Errf("unrecognized subdirective '%s'", d.Val())
			}
		}
	}
	if p.Provider.Username == "" || p.Provider.Password == "" {
		return d.Err("missing username and/or password")
	}

	return nil
}

// Interface guards
var (
	_ caddyfile.Unmarshaler = (*CaddyProvider)(nil)
	_ caddy.Provisioner     = (*CaddyProvider)(nil)
)
