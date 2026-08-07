# Bench host

The network slot for the [ramus](https://forge.lab.pacmag.cz) hardware bench: a
Raspberry Pi 5 (`rpi5`) hanging off the MikroTik's third port in its own
isolated `/24`. The machine itself — the labgrid coordinator and exporter, the
bench services, host bootstrap and automatic updates — is provisioned from the
ramus repo (`tools/bench/deploy/ansible/`); this directory documents only the
network around it.

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

The access model:

- Home LAN → bench net: SSH (22), the labgrid coordinator (20408) and ping,
  nothing else. The coordinator has no TLS and no authentication, so this
  firewall *is* its access control. Everything else labgrid needs — OpenOCD,
  RTT, MIDI, firmware sync — rides inside the SSH connection, so no other
  port is open.
- Bench net → anywhere: internet egress only (out the WAN port, masqueraded,
  then through the home router). It cannot initiate traffic to the home LAN.

Because the bench net lives behind the MikroTik, home-LAN clients need a
static route `192.168.89.0/24 via 192.168.0.2` (per client, or once on the
home router).

labgrid clients also SSH to the exporter by its registered name, which is the
Pi's hostname — so each client machine wants:

```
# ~/.ssh/config
Host rpi5
    HostName 192.168.89.2
```

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
pool, so the address survives lease churn and pool exhaustion. If the board is
ever replaced, this is the one line to update.

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
add place-before=$wanDrop chain=forward src-address=192.168.0.0/24 dst-address=192.168.89.0/24 protocol=tcp dst-port=22,20408 action=accept comment="bench-host: SSH and labgrid coordinator from the home LAN"
add place-before=$wanDrop chain=forward src-address=192.168.0.0/24 dst-address=192.168.89.0/24 protocol=icmp action=accept comment="bench-host: ping from the home LAN"
add place-before=$wanDrop chain=forward dst-address=192.168.89.0/24 action=drop comment="bench-host: nothing else reaches the Pi network"
add place-before=$wanDrop chain=forward src-address=192.168.89.0/24 dst-address=192.168.0.0/24 action=drop comment="bench-host: no Pi traffic to the home LAN"
add place-before=$wanDrop chain=forward src-address=192.168.89.0/24 out-interface-list=WAN action=accept comment="bench-host: internet egress"
add place-before=$wanDrop chain=forward src-address=192.168.89.0/24 action=drop comment="bench-host: no Pi traffic to other local networks"
```

The final drop is the catch-all: anything the Pi sends that did not leave via
the WAN port is denied, so a local network added to the router later is closed
to the bench until a rule says otherwise.

The Pi can still talk to the router itself (DHCP, DNS — that's the input
chain, not forward). Restricting that further wasn't a goal here.
