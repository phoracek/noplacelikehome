# Lab 3

The OptiPlex (`192.168.0.252`). One Caddy reverse proxy terminates TLS for every
service and routes by hostname under `*.lab.pacmag.cz`, with certificates from
Let's Encrypt via ACME DNS-01.

```
[ home LAN 192.168.0.0/24 ]
            │
            ▼
   [ OptiPlex ]  192.168.0.252
      :80  :443  ──▶ Caddy ──┬──▶ auth.lab.pacmag.cz     ──▶ voidauth:3000
                             ├──▶ lab.pacmag.cz          ──▶ dashy:8080     ┐ gated by
                             ├──▶ glances.lab.pacmag.cz  ──▶ glances:61208  ┘ forward_auth
                             ├──▶ grist.lab.pacmag.cz    ──▶ grist:8484      (own OIDC login)
                             └──▶ forge.lab.pacmag.cz    ──▶ forgejo:3000    (own OIDC login)
      :2222 ─────────────────────▶ forgejo (git over SSH — raw TCP, can't be proxied)
```

Services reach each other on the `homelab` Podman network by container name and
publish no host ports of their own — only Caddy binds 80 and 443. Everything
runs as rootful Podman containers defined as systemd Quadlet units
(`quadlet/*.container`); Ansible copies them to the host and starts them.

The `lab/` stack — Home Assistant, Zigbee2MQTT, Mosquitto, Clouds over
Czechoslovakia — also runs on this host, on published ports outside Caddy.

## Services

| Service | URL | Notes |
|---------|-----|-------|
| `caddy.service`    | — | TLS ingress, binds 80/443 |
| `voidauth.service` | <https://auth.lab.pacmag.cz> | Single sign-on provider and login portal |
| `glances.service`  | <https://glances.lab.pacmag.cz> | Host system monitor (no login of its own — gated by VoidAuth) |
| `dashy.service`    | <https://lab.pacmag.cz> | Service dashboard (no login of its own — gated by VoidAuth) |
| `grist.service`    | <https://grist.lab.pacmag.cz> | Spreadsheet / database (logs users in itself, via OIDC against VoidAuth) |
| `forgejo.service`  | <https://forge.lab.pacmag.cz> | Git forge, container registry and CI (logs users in itself, via OIDC against VoidAuth). Also binds host port 2222 for git-over-SSH |

Every hostname above is an A record pointing at `192.168.0.252`. Forgejo's two
Actions runners run on the T470s (`../lab2`) and reach the forge over HTTPS.

## Deploy

Copy `ansible/group_vars/server.yml.example` to `ansible/group_vars/server.yml`
(gitignored) and fill in the values it documents.

```sh
cd ansible
ansible-galaxy collection install -r requirements.yml
ansible-playbook -i inventory.file -u admin   -K create_ansible_user.yml
ansible-playbook -i inventory.file -u ansible    update_dnf_packages.yml
ansible-playbook -i inventory.file -u ansible    install_dnf_automatic.yml
ansible-playbook -i inventory.file -u ansible    install_podman.yml
ansible-playbook -i inventory.file -u ansible    deploy_services.yml
```

The first four are host setup and only need re-running to change the host itself.
`deploy_services.yml` is the entry point for the stack: it deploys the
containerised backends and the shared network first, then Caddy last, so no vhost
forwards to a backend that isn't up yet. Each imported playbook also runs on its
own.

## Operating

```sh
# Status
sudo systemctl status caddy voidauth glances dashy grist forgejo

# Logs (certificate issuance lives in Caddy's journal)
sudo journalctl -u caddy -f

# Pull newer images (or wait for podman-auto-update.timer)
sudo systemctl start podman-auto-update.service
```

The Caddy image is built locally and has no registry copy, so auto-update skips
it — rebuild it by re-running `deploy_services_caddy.yml` after changing the
Containerfile or the vendored provider.

All persistent state lives under `/var/lib/homelab/`, one directory per service.
There is no backup playbook here yet.

## Adding a service

1. Write `quadlet/<name>.container`. Join `homelab.network`, publish no host
   port, bind-mount its data under `/var/lib/homelab/<name>/`.
2. Add `<name>` to `container_units` in
   `ansible/deploy_services_containerized_applications.yml`, plus a task creating
   its data directory (Podman will not create bind-mount sources) and, if it has
   secrets, a template writing a 0600 env file from `group_vars/server.yml`.
3. Add its vhost to `ansible/files/caddy/Caddyfile` and a matching A record →
   `192.168.0.252` in the WEDOS zone. Services with no login of their own get
   gated at the edge with `import voidauth`; ones that speak OIDC register a
   client in VoidAuth instead and are proxied plain (Grist and Forgejo).

## Scripts

Operator tools, run from a workstation. Both read the WEDOS WAPI credentials from
the same gitignored `ansible/group_vars/server.yml` Ansible uses, and both are
dry-run until passed `--apply`.

* `scripts/wedos-dns.py` — list, add, repoint, and delete A records in the zone.
  Commits the zone afterwards, which the WEDOS web UI leaves as a separate step.
* `scripts/clear-acme-challenge.py` — delete an orphaned `_acme-challenge` TXT
  record (see below).

## Troubleshooting

### A certificate won't issue

Every `*.lab.pacmag.cz` name resolves to a private address, so Let's Encrypt
cannot reach this host for an HTTP-01 or TLS-ALPN challenge. Caddy proves control
of the domain by writing a TXT record through the WEDOS WAPI instead, using a
vendored provider (`ansible/files/caddy/caddy-wedos/`) compiled into a custom
Caddy image built on the host.

Two things go wrong there, both in the zone rather than in the config:

* **An orphaned `_acme-challenge` record.** Caddy deletes its TXT record when a
  challenge ends, so killing it mid-challenge leaves one behind — and a leftover
  record satisfies the propagation check instantly, so the next attempt validates
  before its own token exists and fails. Clear it with
  `scripts/clear-acme-challenge.py <name> --apply`.
* **WEDOS nameservers disagreeing.** They do not sync promptly on a write, and
  `ns.wedos.net` has been seen a full challenge cycle behind the others. The
  Caddyfile's `propagation_delay 600s` sits out that window; don't lower it. Caddy
  falls back to the ACME staging endpoint after a production failure, so check the
  issuer of any certificate that appears after a failed attempt.

Query WEDOS's authoritative nameservers, not a public resolver, when checking on a
challenge: the zone's negative-cache TTL is 3600s, so one `dig @1.1.1.1` of an
`_acme-challenge` name while it is still empty poisons public resolvers for an
hour and guarantees the attempt fails.

### Every vhost returns 502 at once

Suspect the Podman network before the services. A firewalld reload flushes the
rules netavark installs for each Podman network; the containers keep running and
reporting healthy while nothing between them can talk, and `journalctl -u caddy`
shows `dial tcp: lookup <service>: i/o timeout`. `install_podman.yml` enables
`netavark-firewalld-reload.service` to re-apply the rules automatically; if that
unit is ever off, the manual repair is:

```sh
sudo podman network reload --all
```

`sudo firewall-cmd --zone=trusted --list-sources` should list the bridge subnets
(`10.88.0.0/16` and `10.89.0.0/24`) — if they're missing, this is what happened.
