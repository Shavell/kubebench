# VM packaging

Files that turn the KubeBench binary into a system service. GoReleaser
(`.goreleaser.yaml`, `nfpms`) builds the `.deb` and `.rpm` packages from
them, and the release archives carry the unit and env file so that
`install.sh --method tarball` lays out the same files.

| File | Installed as |
|---|---|
| `systemd/kubebench.service` | `/usr/lib/systemd/system/kubebench.service` (tarball: `/etc/systemd/system/`, `ExecStart=/usr/local/bin/kubebench`) |
| `kubebench.env` | `/etc/kubebench/kubebench.env`, `root:kubebench 0640`, a conffile (`config|noreplace`): upgrades keep local edits |
| `scripts/postinstall.sh` | package post-install hook |
| `scripts/preremove.sh` | package pre-remove hook |
| `scripts/postremove.sh` | package post-remove hook |

The binary goes to `/usr/bin/kubebench`, and the documentation to
`/usr/share/doc/kubebench/`.

## Layout at runtime

- System user and group `kubebench` (no login shell, home `/var/lib/kubebench`).
- Data: `/var/lib/kubebench` (`kubebench:kubebench 0750`): `kubebench.db`
  (SQLite), Helm repository cache, chart uploads, external plugins.
- Configuration: `/etc/kubebench/kubebench.env`, read by systemd before it
  starts the service. Every variable is documented in the file.

## The service

`kubebench.service` runs as `kubebench` with a strict sandbox:
`ProtectSystem=strict` with `ReadWritePaths=/var/lib/kubebench` only,
`ProtectHome=read-only` (kubeconfigs in home directories stay readable if
the `kubebench` user may read them), `PrivateTmp`, `PrivateDevices`, no
capabilities, `NoNewPrivileges`, a `@system-service` syscall filter and
kernel/cgroup/clock protections (`systemd-analyze security kubebench`
rates it 1.5, "OK"). It restarts on failure.

A different data directory needs a drop-in, because the sandbox only allows
writes to `ReadWritePaths` (`install.sh --data-dir` writes it for you):

```ini
# /etc/systemd/system/kubebench.service.d/10-data-dir.conf
[Service]
Environment=KUBEBENCH_DATA_DIR=/srv/kubebench
Environment=KUBEBENCH_DB=/srv/kubebench/kubebench.db
WorkingDirectory=/srv/kubebench
ReadWritePaths=/srv/kubebench
```

## Package scripts

The same POSIX `sh` scripts serve dpkg and rpm, which call them with
different arguments (`configure <old-version>`, `remove`, `purge`, ... for
dpkg; the number of installed instances for rpm).

- **postinstall** creates the user, group and `/var/lib/kubebench`, hands
  the env file to group `kubebench`, and, when systemd is running, enables
  and starts the service on a fresh install or `try-restart`s it on upgrade.
  `KUBEBENCH_SKIP_START=1` in the environment skips the service steps;
  `install.sh` uses it to write the configuration before the first start.
- **preremove** stops and disables the service on removal (not on upgrade).
- **postremove** reloads systemd; only `dpkg --purge` (`apt purge`) also
  deletes `/var/lib/kubebench`, `/etc/kubebench` and the user. RPM has no
  purge: data, the saved configuration (`.rpmsave`) and the user stay.

Without systemd (containers, chroots) every service step is skipped and
postinstall prints the command to run the server in the foreground.

## Building and testing locally

```bash
make deb                       # snapshot .deb + .rpm into build/release/ (GoReleaser)
docker run --rm -v "$PWD/build/release:/pkg:ro" debian:trixie-slim bash -c \
  'dpkg -i /pkg/kubebench_*_amd64.deb && kubebench --version && id kubebench &&
   ls -la /etc/kubebench /var/lib/kubebench && dpkg -P kubebench'
docker run --rm -v "$PWD/build/release:/pkg:ro" fedora:44 bash -c \
  'rpm -i /pkg/kubebench-*.x86_64.rpm && kubebench --version && rpm -e kubebench'
```
