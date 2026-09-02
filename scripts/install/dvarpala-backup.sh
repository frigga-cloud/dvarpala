#!/bin/bash
#
# Nightly backup of everything on this machine that cannot be recreated.
#
# Two things qualify. The certificate authority, because every .ovpn ever
# issued carries its certificate and will trust no other - losing it means
# every person needs a new file before anyone can connect again. And the
# database, because it holds who exists, what they may reach, and the audit
# trail. Certificates can at least be reissued; an audit trail cannot.
#
# This writes to local disk, which protects against somebody deleting the
# wrong thing. It does NOT protect against losing the machine - for that a
# copy has to leave it. See the note at the end of this file.

set -uo pipefail

DEST=/var/backups/dvarpala
KEEP_DAYS=14
STAMP=$(date +%F)

mkdir -p "$DEST"
chmod 700 "$DEST"

# The database. Piped straight into gzip so the uncompressed copy never
# touches the disk.
if sudo -u postgres pg_dump dvarpala 2>/dev/null | gzip > "$DEST/db-$STAMP.sql.gz"; then
    # A dump that failed halfway still leaves a file, so check it is readable
    # rather than trusting that it exists.
    if gzip -t "$DEST/db-$STAMP.sql.gz" 2>/dev/null; then
        logger -t dvarpala-backup "database backed up: $(du -h "$DEST/db-$STAMP.sql.gz" | cut -f1)"
    else
        logger -t dvarpala-backup "ERROR: database dump is corrupt, removing"
        rm -f "$DEST/db-$STAMP.sql.gz"
    fi
else
    logger -t dvarpala-backup "ERROR: pg_dump failed"
fi

# The certificate authority. Small, and unchanging - but the one thing that
# cannot be recreated at all.
tar -czf "$DEST/certs-$STAMP.tar.gz" -C /opt/dvarpala certs 2>/dev/null && \
    logger -t dvarpala-backup "certificates backed up"

chmod 600 "$DEST"/*.gz 2>/dev/null

# Keep a fortnight. Long enough to notice something went wrong weeks ago,
# short enough not to fill the disk.
find "$DEST" -name "*.gz" -mtime +$KEEP_DAYS -delete 2>/dev/null

# NOTE: these copies live on the machine they protect. If the machine is lost
# they go with it. Take a copy off it as well:
#
#   scp -i <key> ubuntu@<server>:/var/backups/dvarpala/\* ./
