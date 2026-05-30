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

Provision the host:

```sh
sudo dnf install ansible
cd ansible
ansible-playbook -i inventory.file -u petr    -K create_ansible_user.yml
ansible-playbook -i inventory.file -u ansible    update_dnf_packages.yml
ansible-playbook -i inventory.file -u ansible    install_dnf_automatic.yml
ansible-playbook -i inventory.file -u ansible    install_podman.yml
ansible-playbook -i inventory.file -u ansible    deploy_containers.yml
```

After `deploy_containers.yml` the following services are running:

| Service | URL | Purpose |
|---------|-----|---------|
| `caddy.service` | <http://t470s.lab.pacmag.cz> | Reverse proxy |

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
  dst-port=22,80 \
  action=accept \
  comment="Allow SSH and HTTP to T470s from WAN" \
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

## Remote access

MikroTik [Back To Home](https://help.mikrotik.com/docs/spaces/ROS/pages/197984280/Back+To+Home)
can be used to create a WireGuard tunnel to the router and connect to the lab
from outside the home network.
