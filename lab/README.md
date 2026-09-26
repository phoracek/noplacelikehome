# Lab

- OptiPlex: the `*.lab.pacmag.cz` services ([`optiplex/`](optiplex/README.md)).
- optiplex-2: dev box and Forgejo Actions runners
  ([`optiplex-2/`](optiplex-2/README.md)).
- rpi5: the [ramus](https://forge.lab.pacmag.cz) hardware bench, provisioned
  from the ramus repo (`tools/bench/deploy/ansible/`).

This README covers the network. rpi5 and optiplex-2 each sit in an isolated
`/24` behind a MikroTik.

```
                        [ internet ]
                              ▲
                 [ home router 192.168.0.1 ]
                              │
        ┌────────[ home LAN 192.168.0.0/24 ]────────┐
        │                                           │
  [ OptiPlex ] 192.168.0.252            (WAN, ether1) 192.168.0.2
    *.lab.pacmag.cz                            [ MikroTik ]
    (see optiplex/)                                 │
                         ┌──────────────────────────┴──────────────────┐
              (ether3) 192.168.89.1                         (ether4) 192.168.90.1
                         │                                             │
         [ bench net 192.168.89.0/24 ]             [ optiplex-2 net 192.168.90.0/24 ]
                         │                                             │
               [ rpi5 ] 192.168.89.2                     [ optiplex-2 ] 192.168.90.2
                  (DHCP, pinned lease)                        (DHCP, pinned lease)
```

| Host | Address | Provisioned from |
|------|---------|------------------|
| OptiPlex | `192.168.0.252` | `optiplex/ansible/` |
| MikroTik | `192.168.0.2` (WAN) / `192.168.89.1` / `192.168.90.1` | by hand, below |
| rpi5 | `192.168.89.2` | ramus repo |
| optiplex-2 | `192.168.90.2` | `optiplex-2/ansible/` |

The MikroTik is a client of the home router, so home-LAN traffic enters it on
the WAN side.

## Reaching the networks behind the MikroTik

Home-LAN clients need routes via `192.168.0.2`, per client or on the home
router. With NetworkManager:

```
sudo nmcli connection modify <conn> +ipv4.routes "192.168.89.0/24 192.168.0.2"
sudo nmcli connection modify <conn> +ipv4.routes "192.168.90.0/24 192.168.0.2"
sudo nmcli device reapply <iface>
```

The OptiPlex has the optiplex-2 route (`configure_network.yml`). optiplex-2
needs none.

labgrid clients SSH to the exporter by the Pi's hostname:

```
# ~/.ssh/config
Host rpi5
    HostName 192.168.89.2

Host optiplex-2
    HostName 192.168.90.2
    User admin
```

Access:

- Home LAN → bench net: SSH (22), the labgrid coordinator (20408, Pi only),
  ping. The coordinator has no authentication; this firewall is its access
  control. The rest of labgrid runs inside SSH.
- Bench net → anywhere: internet egress only.
- Home LAN → optiplex-2: everything; its firewalld decides.
- BackToHome VPN clients (`192.168.216.0/24`) → optiplex-2: SSH only.
- optiplex-2 → bench net: as the home LAN.
- optiplex-2 → home LAN: the OptiPlex (`192.168.0.252`) on 443 (Caddy: forge,
  registry) and 2222 (git over SSH) only.
- optiplex-2 → anywhere else: internet egress only.
- Bench net and optiplex-2 → the router: DHCP, DNS and ping only.

## MikroTik

Pasted into the RouterOS terminal (v7). The router is managed by hand.

### Shared rules

Management from the home LAN, which arrives on the WAN port:

```
/ip firewall filter add place-before=0 chain=input in-interface=ether1 src-address=192.168.0.0/24 protocol=tcp dst-port=22,80,443,8291 action=accept comment="Allow management from WAN subnet"
```

Each isolated network's port is *not* in the LAN interface list, which is what
opens router management (and the MAC server, neighbor discovery). Input rules
give it DHCP and DNS instead; without them it gets no address.

Rule order, with the home LAN on the WAN side:

- Forward rules go above `defconf: drop all from WAN not DSTNATed` and below
  `defconf: accept established,related`.
- A network's route to the home LAN leaves via WAN, so its →home drop goes
  before its egress accept.

`place-before` keeps that order; rules appended at the bottom never match. The
braces run each block as one script so `:local` survives.

### Bench (`ether3`)

A routed port with DHCP:

```
/interface bridge port remove [find interface=ether3]

/ip address add address=192.168.89.1/24 interface=ether3 comment="bench-host"

/ip pool add name=bench-host ranges=192.168.89.10-192.168.89.254
/ip dhcp-server add name=bench-host interface=ether3 address-pool=bench-host
/ip dhcp-server network add address=192.168.89.0/24 gateway=192.168.89.1 dns-server=192.168.89.1
/ip dhcp-server lease add address=192.168.89.2 mac-address=98:FE:54:0B:B1:0C server=bench-host comment="bench-host (rpi5)"
```

The lease pins the Pi (MAC `98:FE:54:0B:B1:0C`) to `.2`, outside the pool.

Router access:

```
{
/ip firewall filter
:local lanDrop [find comment="defconf: drop all not coming from LAN"]
add place-before=$lanDrop chain=input in-interface=ether3 protocol=udp dst-port=67 action=accept comment="bench-host: DHCP to the router"
add place-before=$lanDrop chain=input in-interface=ether3 protocol=udp dst-port=53 action=accept comment="bench-host: DNS to the router"
add place-before=$lanDrop chain=input in-interface=ether3 protocol=tcp dst-port=53 action=accept comment="bench-host: DNS over TCP to the router"
}
```

Forwarding:

```
{
/ip firewall filter
:local wanDrop [find comment="defconf: drop all from WAN not DSTNATed"]
add place-before=$wanDrop chain=forward src-address=192.168.0.0/24 dst-address=192.168.89.0/24 protocol=tcp dst-port=22 action=accept comment="bench-host: SSH from the home LAN"
add place-before=$wanDrop chain=forward src-address=192.168.0.0/24 dst-address=192.168.89.0/24 protocol=icmp action=accept comment="bench-host: ping from the home LAN"
add place-before=$wanDrop chain=forward src-address=192.168.0.0/24 dst-address=192.168.89.2 protocol=tcp dst-port=20408 action=accept comment="bench-host: Allow traffic to LabGrid coordinator"
add place-before=$wanDrop chain=forward dst-address=192.168.89.0/24 action=drop comment="bench-host: nothing else reaches the network"
add place-before=$wanDrop chain=forward src-address=192.168.89.0/24 dst-address=192.168.0.0/24 action=drop comment="bench-host: no Pi traffic to the home LAN"
add place-before=$wanDrop chain=forward src-address=192.168.89.0/24 out-interface-list=WAN action=accept comment="bench-host: internet egress"
add place-before=$wanDrop chain=forward src-address=192.168.89.0/24 action=drop comment="bench-host: no Pi traffic to other local networks"
}
```

The last drop closes any other local network to the bench.

### optiplex-2 (`ether4`)

As the bench, with optiplex-2's MAC `20:88:10:90:D8:0E`:

```
/interface bridge port remove [find interface=ether4]

/ip address add address=192.168.90.1/24 interface=ether4 comment="optiplex-2"

/ip pool add name=optiplex-2 ranges=192.168.90.10-192.168.90.254
/ip dhcp-server add name=optiplex-2 interface=ether4 address-pool=optiplex-2
/ip dhcp-server network add address=192.168.90.0/24 gateway=192.168.90.1 dns-server=192.168.90.1
/ip dhcp-server lease add address=192.168.90.2 mac-address=20:88:10:90:D8:0E server=optiplex-2 comment="optiplex-2"
```

Router access:

```
{
/ip firewall filter
:local lanDrop [find comment="defconf: drop all not coming from LAN"]
add place-before=$lanDrop chain=input in-interface=ether4 protocol=udp dst-port=67 action=accept comment="optiplex-2: DHCP to the router"
add place-before=$lanDrop chain=input in-interface=ether4 protocol=udp dst-port=53 action=accept comment="optiplex-2: DNS to the router"
add place-before=$lanDrop chain=input in-interface=ether4 protocol=tcp dst-port=53 action=accept comment="optiplex-2: DNS over TCP to the router"
}
```

Forwarding. The bench rules go above the bench's own drop:

```
{
/ip firewall filter
:local benchDrop [find comment="bench-host: nothing else reaches the network"]
:local wanDrop [find comment="defconf: drop all from WAN not DSTNATed"]
add place-before=$benchDrop chain=forward src-address=192.168.90.0/24 dst-address=192.168.89.0/24 protocol=tcp dst-port=22,20408 action=accept comment="optiplex-2: SSH and labgrid coordinator on the bench"
add place-before=$benchDrop chain=forward src-address=192.168.90.0/24 dst-address=192.168.89.0/24 protocol=icmp action=accept comment="optiplex-2: ping the bench"
add place-before=$wanDrop chain=forward src-address=192.168.0.0/24 dst-address=192.168.90.0/24 action=accept comment="optiplex-2: everything from the home LAN"
add place-before=$wanDrop chain=forward src-address=192.168.216.0/24 dst-address=192.168.90.0/24 protocol=tcp dst-port=22 action=accept comment="optiplex-2: SSH from BackToHome"
add place-before=$wanDrop chain=forward dst-address=192.168.90.0/24 action=drop comment="optiplex-2: nothing else reaches it"
add place-before=$wanDrop chain=forward src-address=192.168.90.0/24 dst-address=192.168.0.252 protocol=tcp dst-port=443 action=accept comment="optiplex-2: forge and Caddy vhosts on the OptiPlex"
add place-before=$wanDrop chain=forward src-address=192.168.90.0/24 dst-address=192.168.0.252 protocol=tcp dst-port=2222 action=accept comment="optiplex-2: git over SSH to the forge"
add place-before=$wanDrop chain=forward src-address=192.168.90.0/24 dst-address=192.168.0.0/24 action=drop comment="optiplex-2: no other traffic to the home LAN"
add place-before=$wanDrop chain=forward src-address=192.168.90.0/24 out-interface-list=WAN action=accept comment="optiplex-2: internet egress"
add place-before=$wanDrop chain=forward src-address=192.168.90.0/24 action=drop comment="optiplex-2: no traffic to other local networks"
}
```
