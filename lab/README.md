# Dell

A small AlmaLinux 9 host running Home Assistant, Mosquitto, Zigbee2MQTT,
Dashy, Clouds over Czechoslovakia, and Glances as Podman containers
managed by systemd via Quadlet. Ansible bootstraps the host; everything
else is a `.container` unit deployed from this repo.

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
| `mosquitto.service`     | `127.0.0.1:1883` (host-local) | MQTT broker |

The host advertises itself as `home.local` over mDNS via `avahi-daemon`,
so any LAN client with an mDNS resolver (Linux with `nss-mdns`, macOS,
Windows, Android, iOS) can reach the services by name. Falls back to
the host's IP if mDNS is unavailable. The host itself also runs
`nss-mdns` (pulled in from EPEL by `install_podman.yml`) so its own
libc resolves `home.local` back to itself — otherwise anything on the
host that dials a service by that name hits the LAN DNS server and
fails.

## First-time configuration

1. Open <http://home.local:8123> and complete the HA onboarding wizard.
2. Open <http://home.local:8124> to access Zigbee2MQTT. The dongle is wired
   in via the Quadlet unit; pair devices from this UI.
3. In Home Assistant, add the **MQTT** integration (Settings → Devices &
   services → Add integration → MQTT). Broker: `127.0.0.1`. Port: `1883`. No
   credentials. (Use the IPv4 literal — `localhost` resolves to `::1` first
   and Mosquitto only listens on IPv4 loopback.) Z2M's MQTT discovery will
   then publish all paired devices into HA automatically.

## Operating the stack

```sh
# Status
sudo systemctl status dashy homeassistant mosquitto zigbee2mqtt clouds-over-czechoslovakia-server glances
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
