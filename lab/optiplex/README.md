# OptiPlex

The OptiPlex (`192.168.0.252`). One Caddy reverse proxy terminates TLS for every
service and routes by hostname under `*.lab.pacmag.cz`, with certificates from
Let's Encrypt via ACME DNS-01. The network it sits on — and the hardware bench
it drives — are in the [lab README](../README.md).

```
[ home LAN 192.168.0.0/24 ]
            │
            ▼
   [ OptiPlex ]  192.168.0.252
      :80  :443  ──▶ Caddy ──┬──▶ auth.lab.pacmag.cz     ──▶ voidauth:3000
                             ├──▶ lab.pacmag.cz          ──▶ dashy:8080     ┐ gated by
                             ├──▶ glances.lab.pacmag.cz  ──▶ glances:61208  ┘ forward_auth
                             ├──▶ grist.lab.pacmag.cz    ──▶ grist:8484      (own OIDC login)
                             ├──▶ forge.lab.pacmag.cz    ──▶ forgejo:3000    (own OIDC login)
                             ├──▶ clouds.lab.pacmag.cz   ──▶ clouds-…-server:80  (open, http + https)
                             ├──▶ home.lab.pacmag.cz     ──▶ host:8123       (own login)
                             └──▶ zigbee.lab.pacmag.cz   ──▶ host:8124       (open)
      :2222 ─────────────────────▶ forgejo (git over SSH — raw TCP, can't be proxied)
```

Most services reach each other on the `homelab` Podman network by container name
and publish no host ports of their own; of the ports they listen on, only Caddy's
80 and 443 and Forgejo's 2222 are open in the firewall. Everything runs as
rootful Podman containers defined as systemd Quadlet units
(`quadlet/*.container`); Ansible copies them to the host and starts them.

Three units are the exception and run with `Network=host`: Home Assistant needs
the host's interfaces for SSDP/mDNS discovery and Bluetooth, Zigbee2MQTT needs
the USB dongle, and Mosquitto is the broker they both dial on
`127.0.0.1:1883`. Having no container name to resolve, they are reached from
Caddy through the bridge gateway (`host.containers.internal`) instead. Their
ports are not open in the firewall, so the only way in from the LAN is Caddy.

MQTT itself is not proxied — it is raw TCP, not HTTP, and this Caddy has no
layer 4 module. The broker listens on loopback only and is reachable just by the
two services on this host.

## Services

| Service | URL | Notes |
|---------|-----|-------|
| `caddy.service`    | — | TLS ingress, binds 80/443 |
| `voidauth.service` | <https://auth.lab.pacmag.cz> | Single sign-on provider and login portal |
| `glances.service`  | <https://glances.lab.pacmag.cz> | Host system monitor (no login of its own — gated by VoidAuth) |
| `dashy.service`    | <https://lab.pacmag.cz> | Service dashboard (no login of its own — gated by VoidAuth) |
| `grist.service`    | <https://grist.lab.pacmag.cz> | Spreadsheet / database (logs users in itself, via OIDC against VoidAuth) |
| `forgejo.service`  | <https://forge.lab.pacmag.cz> | Git forge, container registry and CI (logs users in itself, via OIDC against VoidAuth). Also binds host port 2222 for git-over-SSH |
| `clouds-over-czechoslovakia-server.service` | <https://clouds.lab.pacmag.cz> | Satellite cloud cover, served by nginx. Ungated, and also served over plain HTTP — an ESP32 e-ink display polls it and can't do TLS or log in |
| `clouds-over-czechoslovakia.service` | — | Regenerates those artifacts; runs once per firing of `clouds-over-czechoslovakia.timer` (every 10 min), not at boot |
| `homeassistant.service` | <https://home.lab.pacmag.cz> | Home automation (logs users in itself). Host network |
| `zigbee2mqtt.service` | <https://zigbee.lab.pacmag.cz> | Zigbee bridge. Host network, owns the USB dongle. **No login of its own** — see the note below |
| `mosquitto.service` | `127.0.0.1:1883` | MQTT broker between the two. Host network, loopback only, no web UI to proxy |
| `forgejo-runner-1.service`, `forgejo-runner-2.service` | (workers, no UI) | Forgejo Actions runners — one systemd unit per entry in `forgejo_runners`. They serve the forge on this same host |

Every hostname above is an A record pointing at `192.168.0.252`.

Zigbee2MQTT's frontend is served over TLS but is **not** gated by VoidAuth and
has no password, so anyone who can resolve the name can pair devices and rewrite
the Zigbee network. Its own `frontend.auth_token` setting in
`/var/lib/homelab/zigbee2mqtt/configuration.yaml` is the fix if that matters
later; `import voidauth` in the Caddyfile is the other.

## Deploy

Copy `ansible/group_vars/server.yml.example` to `ansible/group_vars/server.yml`
(gitignored) and fill in the values it documents.

```sh
cd ansible
ansible-galaxy collection install -r requirements.yml
ansible-playbook -i inventory.file -u admin   -K create_ansible_user.yml
ansible-playbook -i inventory.file -u ansible    update_dnf_packages.yml
ansible-playbook -i inventory.file -u ansible    install_dnf_automatic.yml
ansible-playbook -i inventory.file -u ansible    configure_network.yml
ansible-playbook -i inventory.file -u ansible    install_podman.yml
ansible-playbook -i inventory.file -u ansible    deploy_services.yml
```

The first five are host setup and only need re-running to change the host itself.
`configure_network.yml` sets the hostname and owns the NetworkManager profile's
static routes — the `static_routes` list at the top of that playbook is the only
place to add one, since it is written wholesale and overwrites anything set on
the profile by hand. Today it carries a single route, `192.168.89.0/24` via
`192.168.0.2`, which is this host's way onto the [bench
network](../README.md#reaching-the-bench).
`deploy_services.yml` is the entry point for the stack: it deploys the
containerised backends and the shared network first, then Caddy last, so no vhost
forwards to a backend that isn't up yet. Each imported playbook also runs on its
own.

## Operating

```sh
# Status
sudo systemctl status caddy voidauth glances dashy grist forgejo \
                     homeassistant zigbee2mqtt mosquitto

# Logs (certificate issuance lives in Caddy's journal)
sudo journalctl -u caddy -f

# Pull newer images (or wait for podman-auto-update.timer)
sudo systemctl start podman-auto-update.service
```

The Caddy image is built locally and has no registry copy, so auto-update skips
it — rebuild it by re-running `deploy_services_caddy.yml` after changing the
Containerfile or the vendored provider.

`podman-prune.timer` runs `podman image prune -f` daily, removing only dangling
images — the ones orphaned each time a `:latest` tag moves to a new build. CI
churn on this host had otherwise filled the disk to the point where image builds
failed with ENOSPC. Tagged images are never removed, so nothing the runners or
the services depend on can be collected; the tradeoff is that a tagged image
that falls out of use has to be removed by hand.

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

## Forgejo Actions runners

Two [Forgejo Actions](https://forgejo.org/docs/latest/user/actions/) runners pick
up CI jobs and run each job as a Podman container on this host. Actions are
enabled on the forge itself (`FORGEJO__actions__ENABLED=true` in its Quadlet);
the runners are declared as one entry each in `forgejo_runners`
(`group_vars/server.yml`), rendered into one Quadlet unit + one `config.yml` per
entry. Add or remove entries to change how many runners run.

Jobs that need real hardware have nothing attached to this host to drive:
ramus's hardware lanes (`protocol-hw-tests`, `editor-hw-tests`) flash firmware
and talk to the labgrid coordinator on the [bench host](../README.md) at
`192.168.89.2`. This host reaches it over the `192.168.89.0/24` static route
`configure_network.yml` installs, and the MikroTik in front of the bench lets
in only SSH and the coordinator port.

### Registering a runner

Forgejo 15 / Runner 12 uses a YAML config file with persistent connection
credentials. All host-side steps go through Ansible — no manual editing on the
server.

1. In Forgejo: **Site Administration → Actions → Runners → "Create new
   runner"**. The page shows a UUID and a registration token (the token is shown
   once — copy both).
2. Fill them into the matching entry under `forgejo_runners` in
   `ansible/group_vars/server.yml` (gitignored).
3. Re-run `deploy_services.yml`. Entries still on the `PASTE_*` placeholders get
   no `config.yml` and are not started, so they can be filled in one at a time.

### Runner ↔ host Podman: how it's wired

Each runner reaches the forge at `https://forge.lab.pacmag.cz/` — its public
name, which resolves to this host's own LAN address and hairpins back to Caddy's
published 443. The job containers it spawns take the same path, so one URL works
for the runner, for git clones and for registry pulls, with no `/etc/hosts` pins
anywhere.

To spawn job containers as siblings, the runner needs the host's rootful Podman
socket, which takes two things (both in `install_podman.yml`):

1. **DAC** — `podman.socket` gets a drop-in setting `SocketMode=0660` +
   `SocketGroup=podman-users` (GID 980); the runner Quadlet joins that GID via
   `GroupAdd=980`. The socket stays unreadable to everyone else.
2. **MAC** — the runner Quadlet sets `SecurityLabelType=container_runtime_t`.
   The socket is labeled `container_var_run_t`, which the default `container_t`
   is explicitly denied. Other containers keep running in `container_t`.

`sudo podman inspect forgejo-runner-1 --format '{{.ProcessLabel}}'` should show
`container_runtime_t`.

### Daily ops

```sh
sudo systemctl status 'forgejo-runner-*'
sudo journalctl -u forgejo-runner-1 -f   # or -u forgejo-runner-2
sudo podman ps --filter name=FORGEJO-ACTIONS-TASK   # job containers while a job runs
```

The admin Actions page
(<https://forge.lab.pacmag.cz/-/admin/actions/runners>) shows each runner's
`Idle`/`Active` status.

A deploy restarts a runner only when its unit or `config.yml` actually changed
*and* it was already running — restarting cancels the job in flight, so a runner
this run just started is left alone.

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
`/var/lib/homelab/<runner-name>/`. Only the `config.yml` is worth anything on
restore — the rest is Actions cache and workflow workdirs, regenerated on the
next run.

## Home Assistant

The host also advertises itself as `home.local` over mDNS (`avahi-daemon`), which
is how devices on the LAN find it. That is separate from the `home.lab.pacmag.cz`
vhost, which is the way in for browsers and the companion apps.

`configuration.yaml` and the rest of `/var/lib/homelab/homeassistant/` are edited
through the UI and are **not** managed by Ansible. One file is the exception,
because it is load-bearing for the ingress: `http.yaml` holds
`use_x_forwarded_for` plus the Podman bridge subnet in `trusted_proxies`, without
which Home Assistant rejects everything Caddy forwards. Ansible writes it, reading
the subnet from `podman network inspect homelab` so a rebuilt host that gets a
different one still works. `configuration.yaml` pulls it in with:

```yaml
http: !include http.yaml
```

That one line is the only part a bare-metal rebuild has to put back by hand.
`internal_url` also lives in `configuration.yaml`, but nothing breaks without it.

### First-time configuration

1. Open <https://home.lab.pacmag.cz> and complete the onboarding wizard.
2. Open <https://zigbee.lab.pacmag.cz> and pair devices from there.
3. Add the **MQTT** integration (Settings → Devices & services → Add
   integration → MQTT). Broker `127.0.0.1`, port `1883`, no credentials. Use the
   IPv4 literal — `localhost` resolves to `::1` first and Mosquitto only listens
   on IPv4 loopback. Zigbee2MQTT's discovery then publishes every paired device
   into Home Assistant automatically.

### Zigbee dongle firmware

The Sonoff Plus V2 here ships EmberZNet 6.x (EZSP v8). Z2M v2 dropped its legacy
`ezsp` driver, so `quadlet/zigbee2mqtt.container` is pinned to
`koenkk/zigbee2mqtt:1.42.0` — the last 1.x release that still ships it. Don't
raise that pin without first flashing the dongle to EmberZNet 7.x via
<https://darkxst.github.io/silabs-firmware-builder/> (pick **Sonoff ZBDongle-E
NCP**, latest tag); flashing wipes the network and requires re-pairing every
device.

### BLE to WiFi proxy

ESPHome runs only as a flashing tool — there is no ESPHome container. Flash and
configure ESP32 boards from <https://web.esphome.io/?dashboard_wizard> in Chrome.

1. Connect the ESP32 with a USB data cable and put it into bootloader mode.
2. Install the initial ESPHome firmware with the web flasher.
3. Use "Edit configuration" to add the BLE proxy:
   ```yaml
   esp32_ble_tracker:
     scan_parameters:
       interval: 1100ms
       window: 1100ms

   bluetooth_proxy:
   ```
4. Save, re-flash, then adopt the device via the auto-discovered ESPHome
   integration and enable BTHome.

Reception on the ESP32 dev kit is weak in practice; the Zigbee path below is the
more reliable one.

### Flashing Mi temperature and humidity sensors

Using <https://github.com/pvvx/ATC_MiThermometer>, from Chrome on Windows —
Android and Fedora struggle with Bluetooth. Open
<https://pvvx.github.io/ATC_MiThermometer/TelinkMiFlasher.html>, tick "Get
advertising MAC", filter `LYWSD03`, connect, and do Activation. Save the Token
and Bind Key somewhere safe, then either:

* **Zigbee** (preferred) — flash the Zigbee Custom Firmware, remove and reinsert
  the battery, bridge the two pins next to the battery for 10 seconds to enter
  pairing mode, and enable "Permit join" in Zigbee2MQTT.
* **BLE** — flash `ATC_v48.bin`, filter `ATC`, reconnect, name it `MI<INDEX>`,
  and set a PIN (write it on paper under the sensor cover).

New devices appear under the MQTT or BTHome integration in Home Assistant.

### HACS

<https://hacs.xyz/docs/use/download/download/#to-download-hacs>. Installs into
`/var/lib/homelab/homeassistant/custom_components/`; run the install script via
`podman exec -it homeassistant bash`, then `sudo systemctl restart
homeassistant`. Better Thermostat (<https://better-thermostat.org/>) is installed
this way — install the UI too, then restart.

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
