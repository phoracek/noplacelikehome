# Dell

A small AlmaLinux 9 host running Home Assistant, Mosquitto, Zigbee2MQTT,
Dashy, Clouds over Czechoslovakia, Glances, PairDrop, Joplin Server,
Forgejo, and a Forgejo Actions runner as Podman containers managed by
systemd via Quadlet. Ansible bootstraps the host; everything else is a
`.container` unit deployed from this repo.

## Bring-up

Install AlmaLinux 9 from the minimal ISO. Pin the host's MAC to a static IP at
the DHCP server. Create an `admin` user and allow it to become root.

```sh
ssh-copy-id admin@192.168.0.248
```

Provision the host:

```sh
sudo dnf install ansible
cd ansible
ansible-playbook -i inventory.file -u admin   -K create_ansible_user.yml
ansible-playbook -i inventory.file -u ansible    update_dnf_packages.yml
ansible-playbook -i inventory.file -u ansible    install_dnf_automatic.yml
ansible-playbook -i inventory.file -u ansible    configure_network.yml
ansible-playbook -i inventory.file -u ansible    install_podman.yml
ansible-playbook -i inventory.file -u ansible    deploy_containers.yml
```

After `deploy_containers.yml`, the following systemd services are running on the host:

| Service | URL | Purpose |
|---------|-----|---------|
| `dashy.service`         | <http://home.local> | Dashboard linking everything below |
| `homeassistant.service` | <http://home.local:8123> | Home Assistant Core |
| `zigbee2mqtt.service`   | <http://home.local:8124> | Zigbee2MQTT web UI |
| `clouds-over-czechoslovakia-server.service` | <http://home.local:8125> | Clouds over Czechoslovakia (artifacts served by nginx) |
| `clouds-over-czechoslovakia.timer` | (every 10 min) | Regenerates the artifacts via the upstream image |
| `glances.service`       | <http://home.local:61208> | Host metrics |
| `pairdrop.service`      | <http://home.local:8127> | LAN file drop (AirDrop-style) |
| `joplin-server.service` | <http://home.local:22300> | Joplin Server (notes sync) |
| `forgejo.service`       | <http://home.local:8128> | Forgejo Git service |
| `forgejo-runner.service` | (worker, no UI) | Forgejo Actions runner |
| `mosquitto.service`     | `127.0.0.1:1883` (host-local) | MQTT broker |

The host advertises itself as `home.local` over mDNS via `avahi-daemon`,
so any LAN client with an mDNS resolver (Linux with `nss-mdns`, macOS,
Windows, Android, iOS) can reach the services by name. Falls back to
the host's IP if mDNS is unavailable.

## First-time configuration

1. Open <http://home.local:8123> and complete the HA onboarding wizard.
2. Open <http://home.local:8124> to access Zigbee2MQTT. The dongle is wired
   in via the Quadlet unit; pair devices from this UI.
3. In Home Assistant, add the **MQTT** integration (Settings → Devices &
   services → Add integration → MQTT). Broker: `127.0.0.1`. Port: `1883`. No
   credentials. (Use the IPv4 literal — `localhost` resolves to `::1` first
   and Mosquitto only listens on IPv4 loopback.) Z2M's MQTT discovery will
   then publish all paired devices into HA automatically.
4. Open <http://home.local:22300> and log in as `admin@localhost` /
   `admin`. Change the admin email and password, then create a regular user
   for sync. In each Joplin client: Tools → Options → Synchronisation →
   "Joplin Server (beta)" with URL `http://home.local:22300` and that user's
   credentials.
5. Open <http://home.local:8128>. The Forgejo install wizard appears with
   the values from the Quadlet unit pre-filled — submit it. Register the
   first account; it becomes site admin. See [Forgejo → Disable open
   registration](#disable-open-registration) for closing sign-ups
   afterwards.
6. Register a Forgejo Actions runner. Forgejo 15 / Runner 12 uses a
   YAML config file with persistent connection credentials. All host-side
   steps go through Ansible — no manual editing on the server.

   a. In Forgejo: Site Administration → Actions → Runners → "Create new
      runner". The page shows a UUID and a token (token is one-time
      view — copy both).
   b. On your workstation, create the secrets file from the example and
      paste the values:

      ```sh
      cp ansible/group_vars/server.yml.example ansible/group_vars/server.yml
      $EDITOR ansible/group_vars/server.yml
      ```

      `ansible/group_vars/server.yml` is gitignored — the token never
      enters the repo.
   c. Re-run the playbooks. `install_podman.yml` templates
      `/var/lib/homelab/forgejo-runner/config.yml` from
      `ansible/templates/forgejo-runner/config.yml.j2`;
      `deploy_containers.yml` starts the runner once the config is in
      place:

      ```sh
      ansible-playbook -i inventory.file -u ansible install_podman.yml
      ansible-playbook -i inventory.file -u ansible deploy_containers.yml
      ```

      The Quadlet's `WantedBy=` handles auto-start at boot;
      `systemctl enable` doesn't work on Quadlet-generated transient
      units.

   Adding more runners later: generate a new credential pair in Forgejo,
   add a second `(uuid, token)` block to `group_vars/server.yml`, clone
   the Quadlet unit with a new `ContainerName` and `Volume` path, add it
   to `quadlet_units`, and re-run the playbooks.

## Operating the stack

```sh
# Status
sudo systemctl status dashy homeassistant mosquitto zigbee2mqtt clouds-over-czechoslovakia-server glances pairdrop joplin-server forgejo forgejo-runner
sudo systemctl list-timers clouds-over-czechoslovakia.timer

# Logs
sudo journalctl -u homeassistant -f

# Edit the HA config
sudo $EDITOR /var/lib/homelab/homeassistant/configuration.yaml
sudo systemctl restart homeassistant

# Pull a newer image (or wait for podman-auto-update.timer)
sudo systemctl start podman-auto-update.service
```

All persistent state lives under `/var/lib/homelab/`; back up that directory
to capture every service's state in one go.

## Forgejo

Self-hosted Git on <http://home.local:8128>, backed by SQLite, with one
Forgejo Actions runner picking up CI jobs locally via the host's rootful
Podman socket.

### Storage layout

Everything Forgejo writes lives under `/var/lib/homelab/forgejo/`,
mounted into the container at three points:

| Host path | In-container path | What's in it |
|-----------|------------------|--------------|
| `forgejo/gitea` | `/data/gitea`     | `app.ini`, sqlite DB |
| `forgejo/git`   | `/data/git`       | bare repos + LFS objects |
| `forgejo/var`   | `/var/lib/gitea`  | runtime work dir (`GITEA_WORK_DIR`) |

The runner's persistent state (`config.yml` + workdir/cache) lives at
`/var/lib/homelab/forgejo-runner/`. Backing up `/var/lib/homelab/`
captures the full server + runner state.

### Runner ↔ host Podman: how it's wired

The runner has to talk to the host's Podman daemon so it can spawn job
containers as siblings. Three pieces make that work without disabling
any security layer:

1. **DAC** — `install_podman.yml` creates a system group
   `podman-users` (GID 980) and drops in
   `/etc/systemd/system/podman.socket.d/override.conf` setting
   `SocketMode=0660 + SocketGroup=podman-users`. The socket is no longer
   world-accessible. The runner Quadlet has `GroupAdd=980` so the
   in-container user is a supplementary member of that group.
2. **MAC** — the runner Quadlet sets
   `SecurityLabelType=container_runtime_t`. The Podman socket is
   labeled `container_var_run_t`, which the default `container_t`
   container domain is forbidden from touching. `container_runtime_t`
   is the domain Podman itself runs in and is permitted to access the
   socket. SELinux stays enforcing for every other container on the
   host.
3. **No-op for everyone else** — only `forgejo-runner.container` has
   `GroupAdd=` and `SecurityLabelType=`. Every other Quadlet keeps the
   default `container_t` and has no socket access.

`sudo podman inspect forgejo-runner --format '{{.ProcessLabel}}'` should
report `…:container_runtime_t:s0:c…` to confirm the type is applied.

### Daily ops

```sh
# Runner status
sudo systemctl status forgejo-runner
sudo journalctl -u forgejo-runner -f

# What the runner sees on the host while a job runs
sudo podman ps --filter name=ACT_       # job containers (sibling to runner)
sudo podman logs <container> -f         # tail a job

# Forgejo server logs
sudo journalctl -u forgejo -f
```

The admin Actions page (<http://home.local:8128/-/admin/actions/runners>)
shows the runner's `Idle`/`Active` status and timestamps of its last
contact.

### Running CI on a repo

Drop a workflow at `.forgejo/workflows/<name>.yml` in any repo. Minimal
example that runs `make check` in the runner-default `node:20-bookworm`
image (which has `make` and a Debian userspace):

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
      - run: make check
```

`runs-on: docker` matches the label the runner registers with
(`docker:docker://node:20-bookworm` — set in
`ansible/templates/forgejo-runner/config.yml.j2`). `actions/checkout@v4`
is resolved via `FORGEJO__actions__DEFAULT_ACTIONS_URL=https://code.forgejo.org`,
which mirrors the common actions.

### Container registry

Forgejo's built-in OCI registry is served on the same listener as the
web UI: `home.local:8128`. No Quadlet env to flip — packages are enabled
by default and land on the existing `forgejo/gitea` volume under
`/data/gitea/packages`. Per-repo images appear in the repo's **Packages**
tab.

Because the listener is plain HTTP, the host's Podman has to be told to
trust `home.local:8128` as insecure — `install_podman.yml` drops
`/etc/containers/registries.conf.d/home-local.conf` for exactly that.
Without it, the runner can't push (it builds via the mounted host
socket, so it's the host's Podman doing the talking), and `podman pull`
on the host or any other LAN machine also refuses.

Push from a workflow:

```yaml
name: image

on:
  push:

jobs:
  build:
    runs-on: docker
    steps:
      - uses: actions/checkout@v4
      - uses: docker/login-action@v3
        with:
          registry: home.local:8128
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/build-push-action@v6
        with:
          push: true
          tags: home.local:8128/${{ github.repository }}:${{ github.sha }}
```

`${{ secrets.GITHUB_TOKEN }}` is a Forgejo-issued, per-job token with
write access to the current repo's packages — the variable name is kept
verbatim for GitHub Actions compatibility (Forgejo Actions also exposes
the same value as `${{ secrets.FORGEJO_TOKEN }}` if the GitHub naming
grates).

Pull from any host on the LAN:

```sh
podman pull home.local:8128/<owner>/<repo>:<tag>
```

The pulling host needs the same `insecure = true` entry in its own
`/etc/containers/registries.conf.d/` (the Dell already has it).

### Opening a PR via the API

Generate a token in User Settings → Applications (scope: at least
`write:repository`) and POST to the Gitea-compatible REST API:

```sh
curl -sS -X POST \
  -H "Authorization: token $TOKEN" \
  -H "Content-Type: application/json" \
  http://home.local:8128/api/v1/repos/<owner>/<repo>/pulls \
  -d '{"title":"…","body":"…","head":"feature-branch","base":"main"}'
```

The `tea` CLI (`tea login add --url http://home.local:8128 --token …`)
wraps the same API for scripting (`tea pr create`, `tea pr merge`, …).

### Disable open registration

After the first user is created via the install wizard (they become
site admin), edit `app.ini` on the host:

```sh
# Path is whatever the wizard reported on its final screen — usually:
sudo $EDITOR /var/lib/homelab/forgejo/gitea/conf/app.ini
# Add or set under [service]:
#   DISABLE_REGISTRATION = true
sudo systemctl restart forgejo
```

### Adding another runner

1. Generate a new UUID/token pair in Forgejo admin UI.
2. Clone `quadlet/forgejo-runner.container` to
   `quadlet/forgejo-runner-2.container`; change `ContainerName=` and
   the data `Volume=` (the `/data` one) to a fresh host path like
   `/var/lib/homelab/forgejo-runner-2`.
3. Add `forgejo-runner-2` to `quadlet_units` in
   `deploy_containers.yml`.
4. Extend `ansible/group_vars/server.yml` with the new credential pair
   under different variable names (e.g. `forgejo_runner_2_uuid`,
   `forgejo_runner_2_token`).
5. Duplicate the template + start task in `install_podman.yml` and
   `deploy_containers.yml` with the new variables and paths.
6. Re-run both plays.

For >2 runners, refactor the playbooks to loop over a list of runner
dicts in `group_vars`; the current setup is intentionally simple for
one runner.

## Home assistant

### BLE to WiFi proxy

ESPHome runs only as a flashing tool — there's no ESPHome dashboard container.
Flash and configure ESP32 boards from <https://web.esphome.io/?dashboard_wizard>
in Chrome.

1. Connect the ESP32 to a USB data cable and put it into bootloader mode.
2. Use the web flasher to install the initial ESPHome firmware.
3. Use the web flasher's "Edit configuration" to add the BLE proxy:
   ```yaml
   esp32_ble_tracker:
     scan_parameters:
       interval: 1100ms
       window: 1100ms

   bluetooth_proxy:
   ```
4. Save and re-flash.
5. Adopt the device in Home Assistant via the auto-discovered ESPHome integration.
6. Enable BTHome from the HA Devices menu.

The ESP32 dev kit has weak Bluetooth reception in practice.

### Flash temperature and humidity sensor — BLE

Using <https://github.com/pvvx/ATC_MiThermometer>. Flash from Chrome on Windows
— Android and Fedora struggle with Bluetooth.

1. Get a Mi Temperature and Humidity Monitor 2.
2. Open <https://pvvx.github.io/ATC_MiThermometer/TelinkMiFlasher.html>.
3. Tick "Get advertising MAC".
4. Filter `LYWSD03`.
5. Connect.
6. Do Activation.
7. Copy the Token and Bind Key and save them somewhere safe.
8. Flash Custom Firmware `ATC_v48.bin`.
9. Start flashing.
10. Filter `ATC`.
11. Connect again.
12. Name it `MI<INDEX>`.
13. Set a PIN, write it on a piece of paper, and place it under the sensor cover.
14. Disconnect.

### Flash temperature and humidity sensor — Zigbee

The BLE proxy has weak signal reception. Convert sensors to the experimental
Zigbee firmware to use the Sonoff dongle instead.

1. Get a Mi Temperature and Humidity Monitor 2.
2. Open <https://pvvx.github.io/ATC_MiThermometer/TelinkMiFlasher.html>.
3. Connect.
4. Do Activation.
5. Copy the Token and Bind Key.
6. Flash the Zigbee Custom Firmware.
7. Start flashing.
8. Remove and reinsert the battery.
9. Bridge the two pins next to the battery for 10 seconds to enter pairing mode.
10. In Zigbee2MQTT (<http://home.local:8124>), enable "Permit join" and let
    the device pair. The MQTT discovery will surface it in Home Assistant.

### Add temperature and humidity sensor to HA

1. Go to **Settings → Devices & services**.
2. New devices appear under the MQTT or BTHome integration.
3. Add them to your dashboards.

### Create a graph of humidity and temperature

1. Create a new dashboard.
2. Add a Sensor card.
3. Select Humidity or Temperature from the sensor.

### Zigbee dongle firmware

The Sonoff Plus V2 here ships EmberZNet 6.x (EZSP v8). Z2M v2 dropped its
legacy `ezsp` driver, so `lab/quadlet/zigbee2mqtt.container` is pinned to
`koenkk/zigbee2mqtt:1.42.0` — the last 1.x release, which still ships the
`ezsp` driver. Don't raise that pin without first flashing the dongle to
EmberZNet 7.x via <https://darkxst.github.io/silabs-firmware-builder/>
(pick **Sonoff ZBDongle-E NCP**, latest tag); flashing wipes the network
and requires re-pairing every device.

### Install HACS

<https://hacs.xyz/docs/use/download/download/#to-download-hacs>

HACS installs into `/var/lib/homelab/homeassistant/custom_components/`. Run the
install script via `podman exec -it homeassistant bash`, then restart HA:

```sh
sudo systemctl restart homeassistant
```

### Install Better Thermostat

<https://better-thermostat.org/>

1. Install it from HACS.
2. Install the UI too.
3. Restart HA.

## TODO

- Put IoT devices on a dedicated VLAN.
- Expose Home Assistant from the main VLAN over the router.
- Give the printer a static IP and allow Home Assistant to reach it.
