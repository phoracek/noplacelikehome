#!/usr/bin/env python3
"""List and edit A records in the WEDOS zone.

The lab's hostnames are plain A records pointing at private addresses. Editing
them in the WEDOS web UI is tedious and easy to get half-right; this does it in
one command and always commits the zone afterwards, which the UI makes a
separate step you can forget.

    ./wedos-dns.py list
    ./wedos-dns.py set lab.pacmag.cz 192.168.0.252            # add or repoint
    ./wedos-dns.py delete old.lab.pacmag.cz
    ./wedos-dns.py set lab.pacmag.cz 192.168.0.252 --apply    # actually do it

Dry-run by default: without --apply it prints the change and touches nothing.

Repointing a name that Caddy holds a certificate for is a cutover, not just a
DNS edit — stop serving the vhost on the old host and remove its stored
certificate first, or both machines renew the same name and fight over the
single shared _acme-challenge record. See clear-acme-challenge.py next door for
cleaning up after an interrupted challenge.

Credentials and the WAPI auth scheme are shared with clear-acme-challenge.py.
"""

import argparse
import hashlib
import json
import pathlib
import socket
import sys
import urllib.parse
import urllib.request
from datetime import datetime
from zoneinfo import ZoneInfo

BASE_URL = "https://api.wedos.com/wapi/json"

# WAPI authorises by source IP, and the allowlist holds the connection's IPv4
# address. On a dual-stack host Python prefers IPv6, so api.wedos.com is reached
# from the v6 address and every call is refused with code 2051 ("Access not
# allowed from this IP address"). Pin outbound connections to IPv4.
_getaddrinfo = socket.getaddrinfo
socket.getaddrinfo = lambda *a, **kw: [r for r in _getaddrinfo(*a, **kw) if r[0] == socket.AF_INET]
VARS_FILE = pathlib.Path(__file__).resolve().parent.parent / "ansible/group_vars/server.yml"

# WEDOS enforces this as the floor; asking for less is rejected.
DEFAULT_TTL = 300


def load_credentials():
    """Read wedos_username/wedos_password without a yaml dependency."""
    creds = {}
    for line in VARS_FILE.read_text().splitlines():
        for key in ("wedos_username", "wedos_password"):
            if line.startswith(key + ":"):
                creds[key] = line.split(":", 1)[1].strip().strip("\"'")
    missing = {"wedos_username", "wedos_password"} - creds.keys()
    if missing:
        sys.exit(f"{VARS_FILE} is missing: {', '.join(sorted(missing))}")
    return creds["wedos_username"], creds["wedos_password"]


def auth_string(user, password):
    hour = datetime.now(ZoneInfo("Europe/Prague")).hour
    pw = hashlib.sha1(password.encode()).hexdigest()
    return hashlib.sha1(f"{user}{pw}{hour:02d}".encode()).hexdigest()


def call(user, password, command, data=None):
    body = {"user": user, "auth": auth_string(user, password), "command": command, "clTRID": command}
    if data is not None:
        body["data"] = data
    payload = urllib.parse.urlencode({"request": json.dumps({"request": body})}).encode()
    req = urllib.request.Request(BASE_URL, data=payload,
                                 headers={"Content-Type": "application/x-www-form-urlencoded"})
    with urllib.request.urlopen(req, timeout=30) as resp:
        result = json.loads(resp.read())["response"]
    # 1000 = OK, 1001 = OK pending. Anything else is a real failure.
    if result.get("code") not in (1000, 1001):
        sys.exit(f"WAPI {command} failed: code={result.get('code')} result={result.get('result')}")
    return result


def relative_name(fqdn, zone):
    """WEDOS stores names relative to the zone; the apex is the empty string."""
    if fqdn == zone:
        return ""
    if not fqdn.endswith("." + zone):
        sys.exit(f"{fqdn} is not inside zone {zone}")
    return fqdn[: -len(zone) - 1]


def rows(user, password, zone):
    return call(user, password, "dns-rows-list", {"domain": zone})["data"]["row"]


def main():
    # --zone/--apply are declared on a shared parent so they're accepted AFTER the
    # subcommand, where they read naturally ("... set lab.pacmag.cz IP --apply").
    # Declared on the top-level parser alone, argparse would only accept them
    # before it.
    common = argparse.ArgumentParser(add_help=False)
    common.add_argument("--zone", default="pacmag.cz", help="WEDOS zone (default: pacmag.cz)")
    common.add_argument("--apply", action="store_true",
                        help="actually change the zone; otherwise dry-run")

    ap = argparse.ArgumentParser(description=__doc__, parents=[common],
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)

    sub.add_parser("list", parents=[common], help="print every record in the zone")

    p_set = sub.add_parser("set", parents=[common],
                           help="add an A record, or repoint an existing one")
    p_set.add_argument("fqdn")
    p_set.add_argument("address")
    p_set.add_argument("--ttl", type=int, default=DEFAULT_TTL)

    p_del = sub.add_parser("delete", parents=[common], help="delete every A record at a name")
    p_del.add_argument("fqdn")

    args = ap.parse_args()
    user, password = load_credentials()

    if args.cmd == "list":
        for row in sorted(rows(user, password, args.zone), key=lambda r: (r["rdtype"], r["name"])):
            name = row["name"] or "@"
            print(f"{name:<32} {row['rdtype']:<6} {row['ttl']:>6}  {row['rdata']}")
        return

    name = relative_name(args.fqdn, args.zone)
    existing = [r for r in rows(user, password, args.zone)
                if r["name"] == name and r["rdtype"] == "A"]

    if args.cmd == "delete":
        if not existing:
            print(f"Nothing to do — no A record at {args.fqdn}")
            return
        verb = "Deleting" if args.apply else "Would delete"
        print(f"{verb} {len(existing)} A record(s) at {args.fqdn}:")
        for row in existing:
            print(f"  id={row['ID']}  {row['rdata']}")
        if not args.apply:
            print("\nDry run. Re-run with --apply.")
            return
        for row in existing:
            call(user, password, "dns-row-delete", {"domain": args.zone, "row_id": row["ID"]})

    else:  # set
        if [r for r in existing if r["rdata"] == args.address] and len(existing) == 1:
            print(f"Nothing to do — {args.fqdn} already points at {args.address}")
            return
        verb = "Setting" if args.apply else "Would set"
        if existing:
            print(f"{verb} {args.fqdn}: {', '.join(r['rdata'] for r in existing)} -> {args.address}")
        else:
            print(f"{verb} {args.fqdn} -> {args.address} (new record)")
        if not args.apply:
            print("\nDry run. Re-run with --apply.")
            return
        # Replace rather than update: dns-row-update exists but takes the same
        # work, and deleting first guarantees a single A record at the name
        # instead of silently adding a second one that round-robins.
        for row in existing:
            call(user, password, "dns-row-delete", {"domain": args.zone, "row_id": row["ID"]})
        call(user, password, "dns-row-add", {
            "domain": args.zone, "name": name, "ttl": args.ttl,
            "type": "A", "rdata": args.address,
        })

    # WEDOS stages changes; nothing is live until the zone is committed.
    call(user, password, "dns-domain-commit", {"name": args.zone})
    print(f"\nCommitted {args.zone}. Publication to the four nameservers lags by a "
          f"minute or two, and resolvers hold the old answer for up to its TTL.")


if __name__ == "__main__":
    main()
