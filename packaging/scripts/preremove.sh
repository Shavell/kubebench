#!/bin/sh
# Pre-remove hook of the kubebench .deb and .rpm packages (nfpm).
#
# Stops and disables the service when the package is removed: dpkg calls it
# as `remove`, rpm with 0 remaining instances. Upgrades leave the service
# running; the new package's postinstall restarts it.
set -e

case "$1" in
  remove | 0) ;;
  *) exit 0 ;; # upgrade, deconfigure, failed-upgrade, rpm upgrade (1)
esac

if [ -d /run/systemd/system ] && command -v systemctl >/dev/null 2>&1; then
  systemctl disable --now kubebench.service || :
fi
