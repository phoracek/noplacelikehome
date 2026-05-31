#!/usr/bin/env bash
# Daily backup of the homelab container data. Tars + gzips the persistent
# service state under /var/lib/homelab into a dated archive, then prunes
# archives older than the retention window. Run by homelab-backup.timer.
set -euo pipefail

SRC_PARENT=/             # tar -C / so the archive holds relative paths
SRC_REL=var/lib/homelab
DEST=/var/backups/homelab
RETENTION_DAYS=14
STAMP="$(date +%F)"      # YYYY-MM-DD
ARCHIVE="$DEST/homelab-$STAMP.tar.gz"

install -d -m 0700 "$DEST"

# Compress persistent service data. -p preserves ownership/modes (root + the
# uid/gid 1000 Forgejo dirs) so a restore is faithful. Exclude the Caddy build
# dir (transient xcaddy artifacts, rebuilt from the repo on deploy). Write to a
# .tmp file then rename so an interrupted run never leaves a truncated archive
# that looks complete.
tar -czpf "$ARCHIVE.tmp" \
    --exclude="$SRC_REL/caddy/build" \
    -C "$SRC_PARENT" "$SRC_REL"
mv -f "$ARCHIVE.tmp" "$ARCHIVE"

# Prune archives older than the retention window.
find "$DEST" -maxdepth 1 -type f -name 'homelab-*.tar.gz' \
    -mtime +"$RETENTION_DAYS" -delete

echo "Backup written: $ARCHIVE ($(du -h "$ARCHIVE" | cut -f1))"
