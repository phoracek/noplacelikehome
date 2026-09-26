# Lab

Three machines. An OptiPlex on the home LAN runs the service stack — everything
under `*.lab.pacmag.cz` — and is documented in [`optiplex/`](optiplex/README.md).
A Raspberry Pi 5 (`rpi5`) is the [ramus](https://forge.lab.pacmag.cz) hardware
bench, and a second OptiPlex (`optiplex-2`) is a Forgejo Actions runner host.
Both hang off a MikroTik, each in its own isolated `/24`.

This README is the shared part: the network they all sit on, and all it takes
to cover the bench and optiplex-2's network slot. The Pi itself — the labgrid
coordinator and exporter, the bench services, host bootstrap and automatic
updates — is provisioned from the ramus repo (`tools/bench/deploy/ansible/`);
nothing in this repo touches it.

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
| OptiPlex | `192.168.0.252` | `optiplex/ansible/` — see [`optiplex/README.md`](optiplex/README.md) |
| MikroTik | `192.168.0.2` (WAN) / `192.168.89.1` (bench) / `192.168.90.1` (optiplex-2) | manual, RouterOS — see below |
| rpi5 | `192.168.89.2` | ramus repo, `tools/bench/deploy/ansible/` |
| optiplex-2 | `192.168.90.2` | manual for now (network slot only) |

The MikroTik is not the main home router — it hangs off the home LAN as a
client at `192.168.0.2` (its WAN port), with the home router at `192.168.0.1`
in front of it. Traffic from the home LAN therefore enters the MikroTik
through its *WAN* side, which is what dictates the firewall rule placement
below.

## Reaching the networks behind the MikroTik

Both networks live behind the MikroTik, so home-LAN clients need static routes
to them via `192.168.0.2` — per client, or once on the home router. On a
NetworkManager client (`<conn>` is the profile on the home LAN, `<iface>` its
device):

```
sudo nmcli connection modify <conn> +ipv4.routes "192.168.89.0/24 192.168.0.2"
sudo nmcli connection modify <conn> +ipv4.routes "192.168.90.0/24 192.168.0.2"
sudo nmcli device reapply <iface>
```

`modify` only changes the saved profile; `reapply` is what puts the routes on
the live interface. `ip route get 192.168.90.2` should then say
`via 192.168.0.2`.

The OptiPlex gets its bench route through `configure_network.yml`, which is
what lets the Forgejo Actions runners there drive the bench. optiplex-2 needs
no route of its own: its default gateway is the MikroTik, which is directly
attached to the bench net.

labgrid clients also SSH to the exporter by its registered name, which is the
Pi's hostname — so each client machine wants:

```
# ~/.ssh/config
Host rpi5
    HostName 192.168.89.2

Host optiplex-2
    HostName 192.168.90.2
    User admin
```

The access model:

- Home LAN → bench net: SSH (22), the labgrid coordinator (20408, on the Pi
  only) and ping, nothing else. The coordinator has no TLS and no
  authentication, so this firewall *is* its access control. Everything else
  labgrid needs — OpenOCD, RTT, MIDI, firmware sync — rides inside the SSH
  connection, so no other port is open.
- Bench net → anywhere: internet egress only (out the WAN port, masqueraded,
  then through the home router). It cannot initiate traffic to the home LAN or
  to optiplex-2.
- Home LAN → optiplex-2: everything. It is a server whose services are meant
  to be reached from the LAN; which ports actually answer is up to its own
  firewalld.
- optiplex-2 → bench net: the same as the home LAN — SSH, the coordinator and
  ping — so runners there can drive the bench.
- optiplex-2 → home LAN: HTTPS to the OptiPlex (`192.168.0.252:443`) only.
  That is Caddy, so the forge — `forge.lab.pacmag.cz` resolves to that
  address — its container registry and the other vhosts; the runners can't
  register, clone or pull without it. Nothing else on the home LAN.
- optiplex-2 → anywhere else: internet egress only.
- Bench net and optiplex-2 → the router itself: DHCP, DNS and ping only. The
  management interfaces (WinBox, WebFig, SSH, MAC-level access) are closed to
  them.

## MikroTik

Everything below is pasted into the RouterOS terminal (v7 syntax). The router
itself stays manually managed — Ansible touches neither the Pi nor optiplex-2's
network.

### Shared rules

The router is administered from the home LAN, which arrives on the WAN port and
would otherwise hit `defconf: drop all not coming from LAN`. One input rule at
the very top of the filter opens the management ports to it:

```
/ip firewall filter add place-before=0 chain=input in-interface=ether1 src-address=192.168.0.0/24 protocol=tcp dst-port=22,80,443,8291 action=accept comment="Allow management from WAN subnet"
```

Each isolated network gets a routed port of its own that is deliberately *not*
in the LAN interface list. Membership there is what the default configuration
trusts for management: the input-chain rule `defconf: drop all not coming from
LAN` is what keeps an interface away from WinBox, WebFig and the router's SSH,
and the MAC server and neighbor discovery are enabled on the LAN list only, so
MAC-Telnet/MAC-WinBox don't answer outside it either. Instead each network gets
exactly the router services it needs, DHCP and DNS, through input rules on its
port placed above that drop. Anything else it sends to the router falls
through to the LAN drop; ping still works via the earlier `defconf: accept
ICMP`. Without these accepts the same drop silently eats DHCP and DNS — the
host comes up with no address.

Isolation between the networks is enforced with explicit forward-chain rules.
Placement constraints, all consequences of the home LAN sitting on the WAN side:

- Every block must sit *above* `defconf: drop all from WAN not DSTNATed`, or
  that rule eats the home LAN's traffic before the accepts are reached (while
  ping to the router's own address on that network still works — that's the
  input chain — which makes the failure look like a host problem, not a
  rule-order problem).
- A network's route to the home LAN goes *out the WAN port*, so a bare
  "accept egress out WAN" would let it reach the home LAN. The explicit
  →home drop must come before the egress accept.
- They must also stay *below* `defconf: accept established,related`, so return
  traffic keeps flowing.

The blocks below use `place-before` to satisfy all of this. The braces make
each block run as one script, so its `:local` survives from line to line.
Rules added without `place-before` land at the bottom of the chain, below the
defconf drops, and never match.

### Bench (`ether3`)

On the default configuration `ether3` is a port of the LAN bridge. First make
it a routed port of its own, give it the bench address, and serve DHCP on it:

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
pool, so the address survives lease churn and pool exhaustion. If the board is
ever replaced, this is the one line to update.

Router access, DHCP and DNS only:

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

The final drop is the catch-all: anything the Pi sends that did not leave via
the WAN port is denied, so a local network added to the router later is closed
to the bench until a rule says otherwise.

### optiplex-2 (`ether4`)

Same shape as the bench: take `ether4` off the bridge, give it the network's
address, serve DHCP with a pinned lease for optiplex-2 (MAC
`20:88:10:90:D8:0E`):

```
/interface bridge port remove [find interface=ether4]

/ip address add address=192.168.90.1/24 interface=ether4 comment="optiplex-2"

/ip pool add name=optiplex-2 ranges=192.168.90.10-192.168.90.254
/ip dhcp-server add name=optiplex-2 interface=ether4 address-pool=optiplex-2
/ip dhcp-server network add address=192.168.90.0/24 gateway=192.168.90.1 dns-server=192.168.90.1
/ip dhcp-server lease add address=192.168.90.2 mac-address=20:88:10:90:D8:0E server=optiplex-2 comment="optiplex-2"
```

Router access, DHCP and DNS only:

```
{
/ip firewall filter
:local lanDrop [find comment="defconf: drop all not coming from LAN"]
add place-before=$lanDrop chain=input in-interface=ether4 protocol=udp dst-port=67 action=accept comment="optiplex-2: DHCP to the router"
add place-before=$lanDrop chain=input in-interface=ether4 protocol=udp dst-port=53 action=accept comment="optiplex-2: DNS to the router"
add place-before=$lanDrop chain=input in-interface=ether4 protocol=tcp dst-port=53 action=accept comment="optiplex-2: DNS over TCP to the router"
}
```

Forwarding. The two bench rules go above the bench's own `nothing else
reaches the network` drop — below it they would never match — and the rest
follows the same pattern as the bench block, with the OptiPlex HTTPS accept
slotted in ahead of the →home drop:

```
{
/ip firewall filter
:local benchDrop [find comment="bench-host: nothing else reaches the network"]
:local wanDrop [find comment="defconf: drop all from WAN not DSTNATed"]
add place-before=$benchDrop chain=forward src-address=192.168.90.0/24 dst-address=192.168.89.0/24 protocol=tcp dst-port=22,20408 action=accept comment="optiplex-2: SSH and labgrid coordinator on the bench"
add place-before=$benchDrop chain=forward src-address=192.168.90.0/24 dst-address=192.168.89.0/24 protocol=icmp action=accept comment="optiplex-2: ping the bench"
add place-before=$wanDrop chain=forward src-address=192.168.0.0/24 dst-address=192.168.90.0/24 action=accept comment="optiplex-2: everything from the home LAN"
add place-before=$wanDrop chain=forward dst-address=192.168.90.0/24 action=drop comment="optiplex-2: nothing else reaches it"
add place-before=$wanDrop chain=forward src-address=192.168.90.0/24 dst-address=192.168.0.252 protocol=tcp dst-port=443 action=accept comment="optiplex-2: forge and Caddy vhosts on the OptiPlex"
add place-before=$wanDrop chain=forward src-address=192.168.90.0/24 dst-address=192.168.0.0/24 action=drop comment="optiplex-2: no other traffic to the home LAN"
add place-before=$wanDrop chain=forward src-address=192.168.90.0/24 out-interface-list=WAN action=accept comment="optiplex-2: internet egress"
add place-before=$wanDrop chain=forward src-address=192.168.90.0/24 action=drop comment="optiplex-2: no traffic to other local networks"
}
```

Traffic from optiplex-2 to the OptiPlex leaves the WAN port and is masqueraded,
so the OptiPlex sees it coming from `192.168.0.2`.
