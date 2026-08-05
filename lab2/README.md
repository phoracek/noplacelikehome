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

Provision the host. Before deploying services, copy
`group_vars/server.yml.example` to `group_vars/server.yml` (gitignored) and fill
in the credentials it documents: the Wedos WAPI credentials Caddy uses for TLS
(see [TLS](#tls)) and the hardware-bench MCP credentials
(see [Hardware bench MCP](#hardware-bench-mcp)).
Install the `ansible.posix` collection too — the hardware-MCP play uses its
`synchronize` (rsync) module.

The host bootstrap + OS-maintenance playbooks run first; then a single
`deploy_services.yml` deploys all the services (it imports
`deploy_services_ramus_mcp` → `deploy_services_containerized_applications` →
`deploy_services_caddy` → `deploy_services_backups` in dependency order: backends
first, the shared Caddy ingress last). Each imported playbook is still runnable
on its own when you only need to touch one service.

```sh
sudo dnf install ansible
cd ansible
cp group_vars/server.yml.example group_vars/server.yml   # then edit in the WAPI credentials
ansible-galaxy collection install -r requirements.yml
ansible-playbook -i inventory.file -u petr    -K create_ansible_user.yml
ansible-playbook -i inventory.file -u ansible    update_dnf_packages.yml
ansible-playbook -i inventory.file -u ansible    install_dnf_automatic.yml
ansible-playbook -i inventory.file -u ansible    install_podman.yml
ansible-playbook -i inventory.file -u ansible    deploy_services.yml
```

To redeploy a single service instead of everything, run its playbook directly,
e.g. `ansible-playbook -i inventory.file -u ansible deploy_services_caddy.yml` (just the
proxy) or `... deploy_services_ramus_mcp.yml` (just the hardware MCP host service).

After `deploy_services.yml` the following services are running:

| Service | URL | Purpose |
|---------|-----|---------|
| `caddy.service` | <http://t470s.lab.pacmag.cz> | Reverse proxy (TLS termination) |
| `ramus-mcp` | <https://ramus-mcp.lab.pacmag.cz> | Hardware bench MCP — a host systemd service, not a container |

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

Nothing on this host binds 2222; the rule can drop it the next time it is edited.

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
| `ramus-mcp.lab.pacmag.cz` | `192.168.88.254` |

The rest of `*.lab.pacmag.cz` is served by the OptiPlex (`../lab3`) and points at
`192.168.0.252`.

## TLS

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

1. Create an A record for the name pointing at `192.168.88.254` (table above).
2. In the Wedos customer admin, enable **WAPI**, set a dedicated WAPI password,
   and allowlist the lab's outbound public IP (WAPI rejects other source IPs).
3. Copy `ansible/group_vars/server.yml.example` to `server.yml` (gitignored) and
   fill in `wedos_username` / `wedos_password` (the plain WAPI password).
4. Ensure the router forwards `443` to the T470s (see the forward rule above).
   It isn't needed for cert issuance, which is outbound-only; it's for reaching
   the service from across the router.

The only certificate this host holds is `ramus-mcp.lab.pacmag.cz`. A name may be
served from exactly one host: two machines renewing one name collide on the
single shared `_acme-challenge` TXT record, so a vhost or a stored certificate
for a name lab3 serves must never be left here.

## Backups

All persistent service state lives in host bind mounts under
`/var/lib/homelab/`. `deploy_services_backups.yml` installs a small backup script
(`/usr/local/sbin/homelab-backup.sh`) driven by a systemd timer
(`homelab-backup.timer`) that runs **daily at 04:00**. Each run tars + gzips the
data into a dated archive under `/var/backups/homelab/`
(`homelab-YYYY-MM-DD.tar.gz`) and prunes archives older than **14 days**.

What's worth capturing here is small: Caddy's issued certs and ACME state
(`caddy/data`, `caddy/config`), the Caddyfile and the 0600 secret env files
(`wedos.env`, `ramus-mcp.env`).

The tar takes all of `/var/lib/homelab`, so any leftover service directory under
it is swept into every archive; delete what is no longer in use rather than
paying for it daily against the 14-day retention.

The transient Caddy build dir (`caddy/build`) is excluded — it's rebuilt from the
repo on every `deploy_services_caddy.yml` run. The backup destination is mode
`0700` because the archives contain those 0600 secrets.

The backup is a **live copy**: it tars the data while the services run, so
there's no downtime. Nothing on this host writes a database during the 04:00 run;
if that ever changes, stop the service around the copy rather than hoping — a
torn SQLite file looks like a perfectly good archive until you need it.

Trigger a backup on demand and inspect it with:

```sh
systemctl start homelab-backup.service
journalctl -u homelab-backup.service          # shows the "Backup written" line
systemctl list-timers homelab-backup.timer    # next scheduled run
```

To **restore**, stop the containers, extract an archive over the root
filesystem (`tar -p` restores the original ownership and modes, including the
0600 secrets), then start the containers again:

```sh
systemctl stop caddy.service
tar -xzpf /var/backups/homelab/homelab-YYYY-MM-DD.tar.gz -C /
systemctl start caddy.service
```

## Remote access

MikroTik [Back To Home](https://help.mikrotik.com/docs/spaces/ROS/pages/197984280/Back+To+Home)
can be used to create a WireGuard tunnel to the router and connect to the lab
from outside the home network.

## Hardware bench MCP

`ramus-mcp.lab.pacmag.cz` fronts the hardware bench MCP server (Daisy Seed
flash / RTT / MIDI) from the `ramus` repo. Unlike the other services it runs as a
**native systemd service on the host**, not a container, because it needs local
USB, `probe-rs`, and ALSA MIDI access to the physically attached board.

- Deploy: `ansible-playbook -i inventory.file -u ansible deploy_services_ramus_mcp.yml`.
  It runs under a dedicated unprivileged service user (`ramus_mcp_service_user`,
  default `ramus-mcp`) that the play creates and adds to the `dialout`/`audio`
  groups, plus udev rules handing the Daisy's three USB nodes (app mode, DFU
  bootloader, ST-Link probe) to that user's own group (so a headless daemon gets
  USB access — `uaccess` only covers logged-in seats). The play
  **rsyncs the code itself** from the control node: it pushes the `mcp-server/`
  tree plus the `tools/probe-utils.sh` helper the server shells out to on every
  flash (set `ramus_mcp_src_dir` to the ramus repo root on your workstation),
  excluding the on-host `.venv` (uv rebuilds it into the service user's home).
  It also installs the bench's runtime deps into `/usr/local/bin`: `uv` +
  `probe-rs` (official install scripts) and `openocd` + `dfu-util` **built from
  source** (both are used by `probe-utils.sh` for flash/reset and neither has an
  EPEL 10 build; pin versions via `openocd_version` / `dfu_util_version`). The
  build adds a few minutes to the first run and is skipped on reruns. `alsa-lib`
  (the libasound `python-rtmidi` links against), `ffmpeg-free` (the `record_audio`
  tool's FLAC capture — the EPEL build keeps the patent-free FLAC encoder), and
  `alsa-utils` (`arecord -L`, for finding the bench mic's ALSA id) come from dnf.
  Point `ramus_mcp_audio_device` in `group_vars/server.yml` at that id (e.g.
  `plughw:CARD=Micro,DEV=0`; the default `default` works until pinned) — the unit
  passes it to the server as `HARDWARE_MCP_AUDIO_DEVICE`. Remaining prereqs:
  the `ansible.posix` collection on the control node for the rsync module
  (`ansible-galaxy collection install -r requirements.yml`; the full `ansible`
  package already bundles it), and outbound internet on the host at deploy time.
  The Caddy site block, credentials env file, and DNS-01 cert are handled by
  `deploy_services_caddy.yml`.
- DNS: add an A record `ramus-mcp.lab.pacmag.cz` -> `192.168.88.254`.
- Auth: Caddy requires HTTP basic auth (`ramus_mcp_basic_auth_user` /
  `ramus_mcp_basic_auth_password_hash` in `group_vars/server.yml`) on every path
  **except** the token-authenticated data plane (`/flash/*`, `/attach/*`,
  `/midi/*`, the one-time `/audio/*` recording downloads, and the token-gated
  `/api/v1/*` calls), which is gated by the server's own one-time lock token. Deny-by-default: a new token-minting route
  (e.g. `/sse`, `/api/v1/lock`) is covered automatically rather than relying on an
  allowlist. `/reopen` + `/probe-reset` are blocked outright at the edge.
- Firewall: do **not** open `8766/tcp` to the LAN - clients reach the bench only
  through Caddy on 443. The service binds `0.0.0.0` so the Caddy container can
  reach it over the podman bridge (`host.containers.internal`); if the default
  firewalld zone blocks it, allow the podman bridge subnet to reach host port 8766.
