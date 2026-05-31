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
in the Wedos WAPI credentials Caddy uses for TLS (see [Forgejo and TLS](#forgejo-and-tls)).

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

## Remote access

MikroTik [Back To Home](https://help.mikrotik.com/docs/spaces/ROS/pages/197984280/Back+To+Home)
can be used to create a WireGuard tunnel to the router and connect to the lab
from outside the home network.
