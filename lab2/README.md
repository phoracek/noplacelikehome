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
  eth1 ── WAN: 192.168.0.0/24  (DHCP from home network)
  eth2 ── LAN: 192.168.88.0/24  (GW 192.168.88.1)
               │
               ▼
        [ T470s server ]  (DHCP lease from router)
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
