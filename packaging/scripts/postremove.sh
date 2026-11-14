#!/bin/sh
# Post-remove hook of the kubebench .deb and .rpm packages (nfpm).
#
# dpkg calls it as `remove`, then `purge` for `dpkg --purge`/`apt purge`;
# rpm calls it with 0 remaining instances on erase. Only a Debian purge
# deletes the data (/var/lib/kubebench), the configuration (/etc/kubebench)
# and the kubebench user; rpm has no purge, so they stay there.
set -e

case "$1" in
  purge) purge=true ;;
  remove | 0) purge=false ;;
  *) exit 0 ;; # upgrade, failed-upgrade, abort-*, disappear, rpm upgrade (1)
esac

if [ -d /run/systemd/system ] && command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || :
fi

if $purge; then
  rm -rf /var/lib/kubebench /etc/kubebench
  if getent passwd kubebench >/dev/null 2>&1; then
    userdel kubebench || echo "kubebench: could not delete the kubebench user" >&2
  fi
  # userdel usually takes the user's private group with it.
  if getent group kubebench >/dev/null 2>&1; then
    groupdel kubebench || echo "kubebench: could not delete the kubebench group" >&2
  fi
fi
