#!/usr/bin/env python3
"""Delete stale _acme-challenge TXT records from the WEDOS zone.

Caddy removes its own challenge record when a challenge ends, so at rest these
names hold nothing. Kill Caddy mid-challenge — a restart, a redeploy — and the
cleanup never runs, leaving an orphan. Orphans are not harmless: Caddy's
propagation check is satisfied by finding *a* record at the name, so the next
attempt validates before its own token is published and fails, orphaning
another.

This is a recovery tool, not part of any deploy. Dry-run by default; pass
--apply to actually delete.

    ./clear-acme-challenge.py auth.lab.pacmag.cz            # show what would go
    ./clear-acme-challenge.py auth.lab.pacmag.cz --apply    # delete and commit

Credentials come from the same gitignored group_vars/server.yml Ansible uses.
The WAPI auth token is sha1(user + sha1(password) + zero-padded Europe/Prague
hour), matching ansible/files/caddy/caddy-wedos/request.go — including the
%02d padding fix, without which auth fails between 00:00 and 09:59.
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


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("fqdn", help="hostname whose challenge records to clear, e.g. auth.lab.pacmag.cz")
    ap.add_argument("--zone", default="pacmag.cz", help="WEDOS zone (default: pacmag.cz)")
    ap.add_argument("--apply", action="store_true", help="actually delete; otherwise dry-run")
    args = ap.parse_args()

    if not args.fqdn.endswith("." + args.zone):
        sys.exit(f"{args.fqdn} is not inside zone {args.zone}")

    # WEDOS stores names relative to the zone: _acme-challenge.auth.lab.pacmag.cz
    # in zone pacmag.cz is the row named "_acme-challenge.auth.lab".
    target = "_acme-challenge." + args.fqdn[: -len(args.zone) - 1]

    user, password = load_credentials()
    rows = call(user, password, "dns-rows-list", {"domain": args.zone})
    stale = [r for r in rows["data"]["row"]
             if r["name"] == target and r["rdtype"] == "TXT"]

    if not stale:
        print(f"Nothing to do — no TXT records at {target}.{args.zone}")
        return

    print(f"{'Deleting' if args.apply else 'Would delete'} {len(stale)} TXT record(s) "
          f"at {target}.{args.zone}:")
    for row in stale:
        print(f"  id={row['ID']}  {row['rdata']}")

    if not args.apply:
        print("\nDry run. Re-run with --apply to delete and commit the zone.")
        return

    for row in stale:
        call(user, password, "dns-row-delete", {"domain": args.zone, "row_id": row["ID"]})
    # WEDOS stages changes; nothing is live until the zone is committed.
    call(user, password, "dns-domain-commit", {"name": args.zone})
    print(f"\nDeleted {len(stale)} record(s) and committed {args.zone}. "
          f"Publication to the four nameservers lags by a minute or two.")


if __name__ == "__main__":
    main()
