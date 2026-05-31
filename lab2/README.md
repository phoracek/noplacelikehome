# Lab 2

A home lab built around a Lenovo ThinkPad T470s running Fedora, managed by
Ansible and Podman Quadlets, with a dedicated router providing network
isolation from the main home LAN.

## Topology

```
Internet
    │
    ▼
[ Router ]
  eth1 ── WAN: 192.168.0.0/24  (DHCP from home network, router is .2)
  eth2 ── LAN: 192.168.88.0/24  (GW 192.168.88.1)
               │
               ▼
        [ T470s server ]  192.168.88.254  (static DHCP lease)
```

The router is a standalone device connected to the home network as a regular
DHCP client on its WAN side, and hands out addresses on `192.168.88.0/24` to
its own LAN. The T470s is the only host on that LAN.

## Configuration

The router is configured manually via its web UI.

The T470s runs AlmaLinux 10 and is provisioned with Ansible: playbooks bootstrap
the OS, install Podman, and deploy applications. Applications run as rootful
Podman containers defined as systemd Quadlet units (`.container` files in this
repo). Ansible copies the Quadlet units to the host and enables the
corresponding services.

## Provisioning

Install AlmaLinux 10 from the minimal ISO. Create an `petr` user and allow it
to become root. Ensure the static DHCP lease is configured on the router so the
host gets 192.168.88.254 before proceeding.

```sh
ssh-copy-id petr@192.168.88.254
```

Provision the host. Before `deploy_containers.yml`, copy
`group_vars/server.yml.example` to `group_vars/server.yml` (gitignored) and fill
in the credentials it documents: the Wedos WAPI credentials Caddy uses for TLS
(see [Forgejo and TLS](#forgejo-and-tls)) and the Glances basic-auth credentials
(see [Glances](#glances)).

```sh
sudo dnf install ansible
cd ansible
cp group_vars/server.yml.example group_vars/server.yml   # then edit in the WAPI credentials
ansible-playbook -i inventory.file -u petr    -K create_ansible_user.yml
ansible-playbook -i inventory.file -u ansible    update_dnf_packages.yml
ansible-playbook -i inventory.file -u ansible    install_dnf_automatic.yml
ansible-playbook -i inventory.file -u ansible    install_podman.yml
ansible-playbook -i inventory.file -u ansible    deploy_containers.yml
```

After `deploy_containers.yml` the following services are running:

| Service | URL | Purpose |
|---------|-----|---------|
| `caddy.service` | <http://t470s.lab.pacmag.cz> | Reverse proxy (TLS termination) |
| `forgejo.service` | <https://forge.lab.pacmag.cz> | Git forge (internal, via Caddy) |
| `glances.service` | <https://glances.t470s.lab.pacmag.cz> | System monitor (internal, via Caddy) |
| `dashy.service` | <https://lab.pacmag.cz> | Service dashboard (internal, via Caddy) |

### DHCP static leases

The T470s is assigned a static lease so it always gets 192.168.88.254:

```routeros
/ip dhcp-server lease add \
  mac-address=54:E1:AD:53:46:83 \
  address=192.168.88.254 \
  server=defconf
```

## Accessing from the home network

Devices on the home network (192.168.0.0/24) can reach the lab LAN directly
by adding a static route via the router's WAN address (192.168.0.2).

### Static route (Linux / NetworkManager)

```bash
nmcli connection modify <connection-name> +ipv4.routes "192.168.88.0/24 192.168.0.2"
nmcli connection up <connection-name>
```

### Router firewall

Two forward/input rules are needed — one to reach the T470s, one to administer
the router itself.

Allow traffic to the T470s (forward chain):

```routeros
/ip firewall filter add \
  chain=forward \
  in-interface=ether1 \
  dst-address=192.168.88.254 \
  protocol=tcp \
  dst-port=22,80,443,2222 \
  action=accept \
  comment="Allow SSH, HTTP, HTTPS and Forgejo git-SSH to T470s from WAN" \
  place-before=0
```

Allow SSH, Winbox, and WebFig to the router (input chain):

```routeros
/ip firewall filter add \
  chain=input \
  src-address=192.168.0.0/24 \
  in-interface=ether1 \
  protocol=tcp \
  dst-port=22,80,443,8291 \
  action=accept \
  comment="Allow management from WAN subnet" \
  place-before=0
```

## DNS

Services are published under `*.lab.pacmag.cz` managed on Wedos. Each entry
is an A record pointing to the server's private IP — names resolve only from
machines that have the static route to 192.168.88.0/24 (or are connected via
Back To Home).

DNS entries are A records on Wedos pointing to private IPs — they only resolve
usefully from machines that have the static route to 192.168.88.0/24 via
192.168.0.2.

| Hostname | A record |
|----------|----------|
| `t470s.lab.pacmag.cz` | `192.168.88.254` |
| `forge.lab.pacmag.cz` | `192.168.88.254` |
| `glances.t470s.lab.pacmag.cz` | `192.168.88.254` |
| `lab.pacmag.cz` | `192.168.88.254` |

## Forgejo and TLS

[Forgejo](https://forge.lab.pacmag.cz)'s web UI runs internal-only: it shares a
private Podman network (`forgejo`) with Caddy and publishes no host port for
HTTP, so the UI and HTTPS git are reachable only through Caddy, which terminates
TLS and reverse-proxies to it.

Git over SSH cannot go through Caddy (it's raw TCP, not HTTP), so Forgejo's
built-in SSH server is published directly on **host port 2222** (host port 22 is
the T470s's own `sshd`). Clone over SSH with:

```sh
git clone ssh://git@forge.lab.pacmag.cz:2222/<owner>/<repo>.git
```

Add your SSH public key under *Settings → SSH / GPG Keys* first. SSH access from
the home network requires the router to forward port 2222 (see the forward rule
above) — HTTPS git needs no key and works the same way over `https://`.

Because every `*.lab.pacmag.cz` name resolves to a private IP, the public ACME
HTTP-01 / TLS-ALPN challenges cannot reach the host. Caddy therefore obtains the
Let's Encrypt certificate via the **DNS-01** challenge against Wedos DNS. The
Wedos DNS provider is **vendored in-tree** under
`ansible/files/caddy/caddy-wedos/` (pinned to its upstream commit SHAs in that
directory's `NOTICE`) rather than pulling the third-party `caddy-dns/wedos`
plugin at build time; Ansible builds a custom `localhost/caddy-wedos` image on
the host from `ansible/files/caddy/Containerfile`.

DNS-01 issuance needs only **outbound** access to Let's Encrypt and the Wedos
WAPI — inbound 443 is only for client access. Prerequisites:

1. Create the A record `forge.lab.pacmag.cz → 192.168.88.254` (table above).
2. In the Wedos customer admin, enable **WAPI**, set a dedicated WAPI password,
   and allowlist the lab's outbound public IP (WAPI rejects other source IPs).
3. Copy `ansible/group_vars/server.yml.example` to `server.yml` (gitignored) and
   fill in `wedos_username` / `wedos_password` (the plain WAPI password).
4. Ensure the router forwards the client-facing ports to the T470s — `443` for
   the web UI / HTTPS git, and `2222` for git over SSH (see the forward rule
   above). Neither is needed for cert issuance (which is outbound-only); they're
   for reaching Forgejo from across the router.

## Glances

[Glances](https://glances.t470s.lab.pacmag.cz) runs in web-server mode as an
internal-only container: like Forgejo it shares the private Podman network with
Caddy and publishes no host port, so its UI is reachable only through Caddy,
which terminates TLS and reverse-proxies to it on port 61208. It reuses the same
shared `wedos_tls` snippet in the Caddyfile, so issuance works the same way as
for Forgejo — the only prerequisite is the A record
`glances.t470s.lab.pacmag.cz → 192.168.88.254` (table above). No new router
forward is needed: client access rides the existing `443` rule.

The container runs with `--pid=host` so Glances reports the host's processes and
load rather than just its own container. Glances' web UI exposes only read-only
stats — its REST API has no process-kill or command-execution endpoint (killing
processes is a feature of the terminal UI only). The real exposure is
information disclosure: with `--pid=host` the process list shows every host
process's full command line, which routinely leaks secrets passed as CLI args.

Glances has **no authentication of its own**, so Caddy gates the site with HTTP
basic auth. Set `glances_user` / `glances_password_hash` in
`group_vars/server.yml` (the hash is a bcrypt string generated with Python's
`bcrypt`, not the plaintext — see `server.yml.example`); Ansible templates them
into a 0600 env file that Caddy reads.

## Dashy

[Dashy](https://lab.pacmag.cz) is a static service dashboard linking to the
other lab services. Like Forgejo and Glances it runs internal-only: it joins the
shared private Podman network with Caddy and publishes no host port, so its UI is
reachable only through Caddy, which terminates TLS and reverse-proxies to it on
port 8080. It reuses the same shared `wedos_tls` snippet in the Caddyfile, so
issuance works the same way as for the other sites — the only prerequisite is the
A record `lab.pacmag.cz → 192.168.88.254` (table above). No new router forward is
needed: client access rides the existing `443` rule.

The dashboard is declarative: its tile layout lives in
`ansible/files/dashy/conf.yml` and is copied to the host (no secrets, so it is
checked into the repo rather than templated from `group_vars`). Edit that file
and re-run `deploy_containers.yml` to change the tiles; Ansible restarts the
container so Dashy reloads the config.

## Remote access

MikroTik [Back To Home](https://help.mikrotik.com/docs/spaces/ROS/pages/197984280/Back+To+Home)
can be used to create a WireGuard tunnel to the router and connect to the lab
from outside the home network.
