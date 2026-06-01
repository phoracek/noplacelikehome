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
(see [Forgejo and TLS](#forgejo-and-tls)) and the Glances basic-auth credentials
(see [Glances](#glances)). Install the `ansible.posix` collection too — the
hardware-MCP play uses its `synchronize` (rsync) module.

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
| `forgejo.service` | <https://forge.lab.pacmag.cz> | Git forge (internal, via Caddy) |
| `forgejo-runner-1.service`, `forgejo-runner-2.service` | (workers, no UI) | Forgejo Actions runners — one systemd unit per entry in `forgejo_runners` |
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

## Forgejo Actions runners

Two [Forgejo Actions](https://forgejo.org/docs/latest/user/actions/) runners
pick up CI jobs locally and run each job as a Podman container. Actions are
already enabled on the server (`FORGEJO__actions__ENABLED=true`); the runners
are defined declaratively as one entry each in `forgejo_runners`
(`group_vars/server.yml`), rendered into one Quadlet unit + one `config.yml`
per entry. Add or remove entries to change how many runners run.

### Registering the runners

Forgejo 15 / Runner 12 uses a YAML config file with persistent connection
credentials. All host-side steps go through Ansible — no manual editing on the
server.

1. In Forgejo: **Site Administration → Actions → Runners → "Create new
   runner"**. The page shows a UUID and a registration token (the token is
   shown once — copy both).
2. On your workstation, paste the values into the gitignored secrets file
   (created from `server.yml.example` during provisioning):

   ```sh
   $EDITOR ansible/group_vars/server.yml   # fill uuid/token under forgejo_runners
   ```

   Entries left on the `PASTE_*` placeholders are skipped — no `config.yml` is
   written and the runner is not started, so you can fill them in one at a time.
3. Re-run `deploy_services_containerized_applications.yml`. It writes one
   `/var/lib/homelab/<name>/config.yml`, renders one Quadlet per entry, and
   starts each runner once its credentials are real:

   ```sh
   ansible-playbook -i inventory.file -u ansible deploy_services_containerized_applications.yml
   ```

   The Quadlet's `WantedBy=` handles auto-start at boot.

### Runner ↔ host Podman: how it's wired

Each runner reaches Forgejo at `https://forge.lab.pacmag.cz/` — the same URL
clients use. The web UI publishes no host port, so the runner Quadlet maps
`forge.lab.pacmag.cz` to `host-gateway` (and passes the same `--add-host` to
every job container it spawns, via `container.options` in `config.yml`),
hairpinning back to Caddy's published 443. The cert is a real Let's Encrypt
cert, so no insecure-TLS flags are needed.

To spawn job containers as siblings, the runner needs the host's rootful Podman
socket — granted without disabling any security layer:

1. **DAC** — `install_podman.yml` creates a system group `podman-users`
   (GID 980) and drops in `/etc/systemd/system/podman.socket.d/override.conf`
   setting `SocketMode=0660` + `SocketGroup=podman-users`. The runner Quadlet
   has `GroupAdd=980` so the in-container user is a supplementary member.
2. **MAC** — the runner Quadlet sets
   `SecurityLabelType=container_runtime_t`. The Podman socket is labeled
   `container_var_run_t`, which the default `container_t` domain is forbidden
   from touching; `container_runtime_t` (the domain Podman itself uses) is
   permitted. SELinux stays enforcing for every other container on the host.

Confirm the type is applied:
`sudo podman inspect forgejo-runner-1 --format '{{.ProcessLabel}}'` should
report `…:container_runtime_t:…`.

### Daily ops

```sh
sudo systemctl status 'forgejo-runner-*'
sudo journalctl -u forgejo-runner-1 -f   # or -u forgejo-runner-2
sudo podman ps --filter name=ACT_        # job containers (siblings) while a job runs
```

The admin Actions page
(<https://forge.lab.pacmag.cz/-/admin/actions/runners>) shows each runner's
`Idle`/`Active` status.

### Running CI on a repo

Drop a workflow at `.forgejo/workflows/<name>.yml`. Minimal example running in
the runner-default `node:20-bookworm` image:

```yaml
name: check

on:
  push:
  pull_request:

jobs:
  check:
    runs-on: docker
    steps:
      - uses: actions/checkout@v4
      - run: echo "hello from CI"
```

`runs-on: docker` (or `ubuntu-latest`) matches the labels the runner registers
with (set in `ansible/templates/forgejo-runner/config.yml.j2`).
`actions/checkout@v4` resolves via
`FORGEJO__actions__DEFAULT_ACTIONS_URL=https://code.forgejo.org`.

Each runner's state (`config.yml` + workdir/cache) lives at
`/var/lib/homelab/<runner-name>/`, so the daily backup (which tars all of
`/var/lib/homelab`, see [Backups](#backups)) captures it automatically.

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
and re-run `deploy_services_containerized_applications.yml` to change the tiles; Ansible restarts the
container so Dashy reloads the config.

## Backups

All persistent service state lives in host bind mounts under
`/var/lib/homelab/`. `deploy_services_backups.yml` installs a small backup script
(`/usr/local/sbin/homelab-backup.sh`) driven by a systemd timer
(`homelab-backup.timer`) that runs **daily at 04:00**. Each run tars + gzips the
data into a dated archive under `/var/backups/homelab/`
(`homelab-YYYY-MM-DD.tar.gz`) and prunes archives older than **14 days**.

What's captured: Caddy's issued certs and ACME state (`caddy/data`,
`caddy/config`), the Caddyfile and the 0600 secret env files (`wedos.env`,
`glances.env`), Forgejo's `gitea.db` and metadata (`forgejo/gitea`), its git
repositories (`forgejo/git`) and runtime data (`forgejo/var`), and the Dashy
config. The transient Caddy build dir (`caddy/build`) is excluded — it's rebuilt
from the repo on every `deploy_services_caddy.yml` run, and Glances has no
persistent state. The backup destination is mode `0700` because the archives
contain those 0600 secrets.

The backup is a **live copy**: it tars the data while the containers run, so
there's no downtime, but a Forgejo SQLite snapshot taken mid-write could in
principle be inconsistent. For a low-traffic personal forge backed up at 04:00
this is an acceptable risk; if it ever matters, switch the script to
`forgejo dump` or briefly stop the containers around the copy.

Trigger a backup on demand and inspect it with:

```sh
systemctl start homelab-backup.service
journalctl -u homelab-backup.service          # shows the "Backup written" line
systemctl list-timers homelab-backup.timer    # next scheduled run
```

To **restore**, stop the containers, extract an archive over the root
filesystem (`tar -p` restores the original ownership and modes, including the
uid/gid 1000 Forgejo dirs and the 0600 secrets), then start the containers
again:

```sh
systemctl stop caddy.service forgejo.service glances.service dashy.service
tar -xzpf /var/backups/homelab/homelab-YYYY-MM-DD.tar.gz -C /
systemctl start caddy.service forgejo.service glances.service dashy.service
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
  (the libasound `python-rtmidi` links against) comes from dnf. Remaining prereqs:
  the `ansible.posix` collection on the control node for the rsync module
  (`ansible-galaxy collection install -r requirements.yml`; the full `ansible`
  package already bundles it), and outbound internet on the host at deploy time.
  The Caddy site block, credentials env file, and DNS-01 cert are handled by
  `deploy_services_caddy.yml`.
- DNS: add an A record `ramus-mcp.lab.pacmag.cz` -> `192.168.88.254`.
- Auth: Caddy requires HTTP basic auth (`ramus_mcp_basic_auth_user` /
  `ramus_mcp_basic_auth_password_hash` in `group_vars/server.yml`) on every path
  **except** the token-authenticated data plane (`/flash/*`, `/attach/*`,
  `/midi/*`, and the token-gated `/api/v1/*` calls), which is gated by the
  server's own one-time lock token. Deny-by-default: a new token-minting route
  (e.g. `/sse`, `/api/v1/lock`) is covered automatically rather than relying on an
  allowlist. `/reopen` + `/probe-reset` are blocked outright at the edge.
- Firewall: do **not** open `8766/tcp` to the LAN - clients reach the bench only
  through Caddy on 443. The service binds `0.0.0.0` so the Caddy container can
  reach it over the podman bridge (`host.containers.internal`); if the default
  firewalld zone blocks it, allow the podman bridge subnet to reach host port 8766.
