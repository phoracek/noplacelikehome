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

The T470s is provisioned with Ansible: playbooks bootstrap the OS, install
Podman, and deploy applications. Applications run as rootful Podman containers
defined as systemd Quadlet units (`.container` files in this repo). Ansible
copies the Quadlet units to the host and enables the corresponding services.

## Remote access

MikroTik [Back To Home](https://help.mikrotik.com/docs/spaces/ROS/pages/197984280/Back+To+Home)
can be used to create a WireGuard tunnel to the router and connect to the lab
from outside the home network.

### Router admin from WAN

The router's firewall input chain allows SSH, Winbox, and WebFig from the WAN subnet so the router can be administered from the home network without being on the lab LAN.

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

## DHCP static leases

The T470s is assigned a static lease so it always gets 192.168.88.254:

```routeros
/ip dhcp-server lease add \
  mac-address=54:E1:AD:53:46:83 \
  address=192.168.88.254 \
  server=defconf
```

## Server access from the home network

With the static route in place, the T470s (192.168.88.254) is reachable directly by IP from the home network. The router's forward chain must allow the traffic through:

```routeros
/ip firewall filter add \
  chain=forward \
  in-interface=ether1 \
  dst-address=192.168.88.254 \
  protocol=tcp \
  dst-port=22 \
  action=accept \
  comment="Allow SSH to T470s from WAN" \
  place-before=0
```

## Routing lab traffic from the home network

Devices on the home network (192.168.0.0/24) need a static route to reach the lab LAN (192.168.88.0/24) via the router's WAN address (192.168.0.2).

### Linux (NetworkManager)

```bash
nmcli connection modify <connection-name> +ipv4.routes "192.168.88.0/24 192.168.0.2"
nmcli connection up <connection-name>
```
