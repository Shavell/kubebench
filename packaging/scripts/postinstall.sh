#!/bin/sh
# Post-install hook of the kubebench .deb and .rpm packages (nfpm).
#
# dpkg calls it as `configure [<old-version>]`, rpm with the number of
# installed instances (1 = install, 2+ = upgrade). It creates the kubebench
# system user and /var/lib/kubebench, fixes the permissions of the env file
# and, when systemd is running, enables and starts the service on a fresh
# install or restarts it (if it was running) on upgrade.
#
# KUBEBENCH_SKIP_START=1 skips enable/start/restart: install.sh sets it to
# write the configuration before the first start. Without systemd (e.g. in
# containers) the service steps are skipped too.
set -e

case "$1" in
  configure) if [ -n "$2" ]; then action=upgrade; else action=install; fi ;;
  [0-9]*) if [ "$1" -gt 1 ]; then action=upgrade; else action=install; fi ;;
  *) exit 0 ;; # abort-upgrade, abort-remove, abort-deconfigure
esac

user=kubebench
data_dir=/var/lib/kubebench
env_file=/etc/kubebench/kubebench.env

if ! getent group "$user" >/dev/null 2>&1; then
  groupadd --system "$user"
fi
if ! getent passwd "$user" >/dev/null 2>&1; then
  shell=/usr/sbin/nologin
  [ -x "$shell" ] || shell=/sbin/nologin
  [ -x "$shell" ] || shell=/bin/false
  useradd --system --gid "$user" --home-dir "$data_dir" --no-create-home \
    --shell "$shell" --comment "KubeBench server" "$user"
fi

install -d -m 0750 -o "$user" -g "$user" "$data_dir"
# The package ships the env file as root:root because the group does not
# exist before this script runs. The file may hold the bootstrap password.
if [ -f "$env_file" ]; then
  chown "root:$user" "$env_file"
  chmod 0640 "$env_file"
fi

systemd=false
if [ -d /run/systemd/system ] && command -v systemctl >/dev/null 2>&1; then
  systemd=true
  systemctl daemon-reload || :
fi

if [ "${KUBEBENCH_SKIP_START:-0}" = 1 ]; then
  exit 0
fi

if [ "$action" = upgrade ]; then
  if $systemd; then
    systemctl try-restart kubebench.service || :
  fi
  exit 0
fi

if $systemd; then
  systemctl enable kubebench.service || :
  if systemctl start kubebench.service; then
    status="KubeBench is running on port 9876 (systemctl status kubebench)."
  else
    status="kubebench.service did not start; see: journalctl -u kubebench"
  fi
else
  if command -v runuser >/dev/null 2>&1; then
    as_user="runuser -u $user --"
  elif command -v sudo >/dev/null 2>&1; then
    as_user="sudo -u $user"
  else
    as_user="(as the $user user)"
  fi
  status="systemd is not running; start KubeBench in the foreground with:
  $as_user sh -c 'set -a; KUBEBENCH_DATA_DIR=$data_dir KUBEBENCH_DB=$data_dir/kubebench.db; . $env_file; cd $data_dir && exec /usr/bin/kubebench'"
fi

cat <<EOF

$status
  Configuration: $env_file
  Data:          $data_dir
Sign in as "admin" with KUBEBENCH_ADMIN_PASSWORD from the configuration, or
"admin" if it was not set before the first start; either way it must be
changed at the first sign-in.

EOF
