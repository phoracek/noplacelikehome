# Bench host

A Raspberry Pi 5 running Raspberry Pi OS Lite (64-bit, headless), hanging off
the MikroTik's third port in its own isolated `/24`. The home LAN can SSH to it;
the Pi network can reach the internet and nothing else — it cannot initiate
traffic toward the home LAN or any other local network.

For now the Ansible here only bootstraps the `ansible` user and turns on
automatic updates. Services come later.

The MikroTik is not the main home router — it hangs off the home LAN as a
client at `192.168.0.2` (its WAN port), with the home router at `192.168.0.1`
in front of it. Traffic from the home LAN therefore enters the MikroTik
through its *WAN* side, which is what dictates the firewall rule placement
below.

```
                     [ internet ]
                          ▲
              [ home router 192.168.0.1 ]
                          │
              [ home LAN 192.168.0.0/24 ]      ← clients need a route:
                          │                      192.168.89.0/24 via 192.168.0.2
        (WAN, ether1) 192.168.0.2
                    [ MikroTik ]
              (ether3) 192.168.89.1
                          │
           [ bench-host net 192.168.89.0/24 ]
                          │
             [ rpi5 ] 192.168.89.2 (DHCP, pinned lease)
```

Home LAN → Pi is allowed for SSH (and ping) only; the Pi network can egress to
the internet (out the WAN port, masqueraded, then through the home router) and
initiate nothing else — not to the home LAN, not to the MikroTik's own
`192.168.88.0/24` LAN.

Because the bench net lives behind the MikroTik, home-LAN clients need a
static route `192.168.89.0/24 via 192.168.0.2` (per client, or once on the
home router), same as the existing one for `192.168.88.0/24`.

## MikroTik

Everything below is pasted into the RouterOS terminal (v7 syntax). The router
itself stays manually managed — Ansible only touches the Pi.

On the default configuration `ether3` is a port of the LAN bridge. First make
it a routed port of its own, give it the bench-host address, and serve DHCP on it:

```
/interface bridge port remove [find interface=ether3]

/ip address add address=192.168.89.1/24 interface=ether3 comment="bench-host"

/ip pool add name=bench-host ranges=192.168.89.10-192.168.89.254
/ip dhcp-server add name=bench-host interface=ether3 address-pool=bench-host
/ip dhcp-server network add address=192.168.89.0/24 gateway=192.168.89.1 dns-server=192.168.89.1
/ip dhcp-server lease add address=192.168.89.2 mac-address=98:FE:54:0B:B1:0C server=bench-host comment="bench-host (rpi5)"
```

The static lease pins the Pi (MAC `98:FE:54:0B:B1:0C`, printed by
`/ip dhcp-server lease print` after its first boot) to `.2` — outside the
pool, so the inventory address survives lease churn and pool exhaustion. If
the board is ever replaced, this is the one line to update.

Add `ether3` to the LAN interface list — without this the default input-chain
rule (`drop all not coming from LAN`) silently eats the Pi's DHCP and DNS
requests to the router:

```
/interface list member add list=LAN interface=ether3
```

The isolation is enforced with explicit forward-chain rules. Two placement
constraints, both consequences of the home LAN sitting on the WAN side:

- The whole block must sit *above* `defconf: drop all from WAN not DSTNATed`,
  or that rule eats the home LAN's SSH before the accepts are reached (while
  ping to `192.168.89.1` still works — that's the input chain — which makes
  the failure look like a Pi problem, not a rule-order problem).
- The Pi's route to the home LAN goes *out the WAN port*, so a bare
  "accept egress out WAN" would let the Pi reach the home LAN. The explicit
  Pi→home drop must come before the egress accept.

It must also stay *below* `defconf: accept established,related`, so return
traffic keeps flowing. `place-before` handles all of this:

```
/ip firewall filter
:local wanDrop [find comment="defconf: drop all from WAN not DSTNATed"]
add place-before=$wanDrop chain=forward src-address=192.168.0.0/24 dst-address=192.168.89.0/24 protocol=tcp dst-port=22 action=accept comment="bench-host: SSH from the home LAN"
add place-before=$wanDrop chain=forward src-address=192.168.0.0/24 dst-address=192.168.89.0/24 protocol=icmp action=accept comment="bench-host: ping from the home LAN"
add place-before=$wanDrop chain=forward dst-address=192.168.89.0/24 action=drop comment="bench-host: nothing else reaches the Pi network"
add place-before=$wanDrop chain=forward src-address=192.168.89.0/24 dst-address=192.168.0.0/24 action=drop comment="bench-host: no Pi traffic to the home LAN"
add place-before=$wanDrop chain=forward src-address=192.168.89.0/24 out-interface-list=WAN action=accept comment="bench-host: internet egress"
add place-before=$wanDrop chain=forward src-address=192.168.89.0/24 action=drop comment="bench-host: drop the rest (incl. the 192.168.88.0/24 LAN)"
```

The final drop also covers Pi → `192.168.88.0/24`: that traffic leaves via the
LAN bridge, not the WAN port, so the egress accept doesn't match it.

The Pi can still talk to the router itself (DHCP, DNS — that's the input
chain, not forward). Restricting that further wasn't a goal here.

## First boot

Flash Raspberry Pi OS Lite (64-bit) with Raspberry Pi Imager and use its OS
customisation dialog — the box is headless, so this is the only chance to get
SSH in without plugging in a monitor:

- hostname: `rpi5`
- a username of your choice with a password — whatever you pick goes into
  `bootstrap_user` in `group_vars/server.yml`
- paste your public key (the one `ssh_public_key` in `group_vars/server.yml`
  points at — it must be usable non-interactively, i.e. passphrase-free or
  agent-loaded), enable SSH
- no Wi-Fi — the box lives on `ether3`

Boot it on `ether3` — the static lease hands it `.2` right away — and check
`ssh <bootstrap_user>@192.168.89.2` answers from the home LAN.

## Provisioning

Copy `group_vars/server.yml.example` to `group_vars/server.yml` (gitignored),
set `bootstrap_user` to the username chosen in Imager and `ssh_public_key` to
the key the `ansible` user should accept.

```sh
cd ansible
ansible-playbook -i inventory.file           -K create_ansible_user.yml
ansible-playbook -i inventory.file -u ansible   update_apt_packages.yml
ansible-playbook -i inventory.file -u ansible   install_unattended_upgrades.yml
```

The first run rides on the Imager-created `bootstrap_user` (`-K` for its sudo
password); everything after uses the passwordless `ansible` user it creates.

## Automatic updates

`install_unattended_upgrades.yml` installs `unattended-upgrades` and drops two
files into `/etc/apt/apt.conf.d/`:

- `20auto-upgrades` — turns on the daily list refresh and upgrade run
  (executed by `apt-daily.timer` and `apt-daily-upgrade.timer`).
- `52unattended-upgrades-bench-host` — adds the Raspberry Pi archive to the upgrade
  origins (the packaged defaults only cover Debian, and the kernel and firmware
  come from `archive.raspberrypi.com` — without this they'd never auto-update),
  and enables an automatic reboot at 04:00 when an update requires one.

To check it's actually running:

```sh
# What the next run would do
sudo unattended-upgrade --dry-run --debug

# What past runs did
less /var/log/unattended-upgrades/unattended-upgrades.log
```
