#!/bin/bash
#
# Installed as /usr/local/bin/dvarpala-cli.
#
# The real binary lives in /opt/dvarpala/bin and needs two things every time:
# the config file, and to run as the dvarpala user, since that is who owns the
# certificates and may read the database password. Expecting an administrator
# to remember
#
#   sudo -u dvarpala /opt/dvarpala/bin/dvarpala-cli --config /opt/dvarpala/config/environment.yaml user list
#
# is how a tool ends up unused. Every message the installer prints, and every
# instruction in the documentation, says "dvarpala-cli" - so that is what needs
# to exist.

set -uo pipefail

BIN=/opt/dvarpala/bin/dvarpala-cli
CONFIG=/opt/dvarpala/config/environment.yaml
USER=dvarpala

[[ -x "$BIN" ]] || { echo "dvarpala-cli: $BIN is not installed" >&2; exit 1; }

# Already the right user: run it directly, so this works where sudo is not
# installed at all.
if [[ "$(id -un)" == "$USER" ]]; then
    [[ -r "$CONFIG" ]] || { echo "dvarpala-cli: cannot read $CONFIG" >&2; exit 1; }
    exec "$BIN" --config "$CONFIG" "$@"
fi

# Deliberately not checked from here. The config is readable only by the
# service user, which is the point of it - it holds the database password.
# Testing it as the caller reports "cannot read" for a file that is perfectly
# readable by the user this is about to become.

command -v sudo >/dev/null || {
    echo "dvarpala-cli: must run as $USER (sudo is not installed)" >&2
    exit 1
}

# --config is passed here rather than left to the caller. Anyone who wants a
# different one can still say so; the last --config on the line wins.
exec sudo -u "$USER" "$BIN" --config "$CONFIG" "$@"
