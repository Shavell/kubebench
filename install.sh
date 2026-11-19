#!/usr/bin/env bash
# KubeBench installer for Linux servers and VMs.
#
#   curl -fsSL https://raw.githubusercontent.com/Shavell/kubebench/main/install.sh | sudo bash
#   curl -fsSL https://raw.githubusercontent.com/Shavell/kubebench/main/install.sh | sudo bash -s -- --version v0.8.2
#   sudo bash install.sh --offline ./kubebench-offline-0.8.2.tar.gz
#
# Installs the .deb or .rpm package (the tarball where neither fits) from a
# GitHub release or from local files, verifies SHA-256 checksums, writes
# /etc/kubebench/kubebench.env and starts kubebench.service. Running it again
# upgrades in place and keeps the configuration and the data. See --help.
set -Eeuo pipefail

readonly REPO="Shavell/kubebench"
readonly RELEASES="https://github.com/${REPO}/releases"
readonly SERVICE_USER=kubebench
readonly CONF_DIR=/etc/kubebench
readonly ENV_FILE=/etc/kubebench/kubebench.env
readonly KUBECONFIG_DEST=/etc/kubebench/kubeconfig
readonly DEFAULT_DATA_DIR=/var/lib/kubebench
readonly UNIT=kubebench.service
readonly TARBALL_BIN=/usr/local/bin/kubebench
readonly TARBALL_UNIT=/etc/systemd/system/kubebench.service
readonly DROPIN_DIR=/etc/systemd/system/kubebench.service.d
readonly DROPIN=/etc/systemd/system/kubebench.service.d/10-data-dir.conf

opt_version=latest
opt_method=auto
opt_offline=""
opt_port=""
opt_listen=""
opt_password=""
opt_kubeconfig=""
opt_data_dir=""
opt_start=true
opt_uninstall=false
opt_purge=false
opt_dry_run=false
opt_yes=false

arch=""               # amd64 | arm64
rpm_arch=""           # x86_64 | aarch64
artifact=""           # package or archive to install
artifact_method=""    # deb | rpm | tarball
workdir=""            # temporary download/extract directory
generated_password="" # printed once at the end

usage() {
  cat <<'EOF'
Install, upgrade or remove KubeBench on a Linux host (systemd optional).

Usage: install.sh [options]

  --version <vX.Y.Z|latest>  release to install (default: latest)
  --method <auto|deb|rpm|tarball>
                             auto: the existing install's method, else deb
                             (dpkg), rpm, or the tarball (default: auto)
  --offline <path>           install from local files instead of GitHub: an
                             offline bundle (.tar.gz or extracted directory),
                             a .deb/.rpm, or kubebench_<v>_linux_<arch>.tar.gz;
                             sets KUBEBENCH_UPDATE_MODE=off unless set
  --port <port>             PORT in the env file (default 9876)
  --listen <host:port>       KUBEBENCH_LISTEN_ADDR in the env file
  --admin-password <pw>      bootstrap admin password; without it a random one
                             is generated on a fresh install and printed once
  --kubeconfig <file>        copy to /etc/kubebench/kubeconfig and seed the
                             cluster inventory from it on the first start
  --data-dir <dir>           data directory (default /var/lib/kubebench)
  --no-start                 install and configure, but do not (re)start
  --uninstall                remove KubeBench (configuration and data stay)
  --purge                    with --uninstall: also delete configuration,
                             data and the kubebench user
  --dry-run                  print what would happen; change nothing
  -y, --yes                  do not ask for confirmation
  -h, --help                 show this help

Examples:
  curl -fsSL https://raw.githubusercontent.com/Shavell/kubebench/main/install.sh | sudo bash
  curl -fsSL .../install.sh | sudo bash -s -- --version v0.8.2 --port 8080
  sudo bash install.sh --offline ./kubebench-offline-0.8.2.tar.gz
  sudo bash install.sh --uninstall --purge --yes
EOF
}

log() { printf '==> %s\n' "$*" >&2; }
warn() { printf 'warning: %s\n' "$*" >&2; }
die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

# run executes a command, or only prints it with --dry-run.
run() {
  if $opt_dry_run; then
    printf '[dry-run]' >&2
    printf ' %q' "$@" >&2
    printf '\n' >&2
  else
    "$@"
  fi
}

have() { command -v "$1" >/dev/null 2>&1; }
systemd_running() { [[ -d /run/systemd/system ]] && have systemctl; }

cleanup() {
  if [[ -n "$workdir" ]]; then rm -rf "$workdir"; fi
}

parse_args() {
  while (($#)); do
    local arg="$1" value=""
    if [[ "$arg" == --*=* ]]; then
      set -- "${arg%%=*}" "${arg#*=}" "${@:2}"
      arg="$1"
    fi
    case "$arg" in
      --version | --method | --offline | --port | --listen | --admin-password | --kubeconfig | --data-dir)
        (($# >= 2)) || die "$arg needs a value"
        value="$2"
        shift 2
        ;;
      *) shift ;;
    esac
    case "$arg" in
      --version) opt_version="$value" ;;
      --method) opt_method="$value" ;;
      --offline) opt_offline="$value" ;;
      --port) opt_port="$value" ;;
      --listen) opt_listen="$value" ;;
      --admin-password) opt_password="$value" ;;
      --kubeconfig) opt_kubeconfig="$value" ;;
      --data-dir) opt_data_dir="$value" ;;
      --no-start) opt_start=false ;;
      --uninstall) opt_uninstall=true ;;
      --purge) opt_purge=true ;;
      --dry-run) opt_dry_run=true ;;
      -y | --yes) opt_yes=true ;;
      -h | --help)
        usage
        exit 0
        ;;
      *) die "unknown option: $arg (see --help)" ;;
    esac
  done

  case "$opt_method" in
    auto | deb | rpm | tarball) ;;
    *) die "--method must be auto, deb, rpm or tarball" ;;
  esac
  if [[ "$opt_version" != latest && ! "$opt_version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$ ]]; then
    die "--version must look like v0.8.2 (or be \"latest\")"
  fi
  if [[ -n "$opt_port" ]] && { [[ ! "$opt_port" =~ ^[0-9]{1,5}$ ]] || ((10#$opt_port < 1 || 10#$opt_port > 65535)); }; then
    die "--port must be a TCP port (1-65535)"
  fi
  if [[ -n "$opt_listen" && ! "$opt_listen" =~ ^[^[:space:]\"\']*:[0-9]{1,5}$ ]]; then
    die "--listen must be host:port, e.g. 127.0.0.1:9876"
  fi
  [[ "$opt_password" != *$'\n'* ]] || die "--admin-password must be a single line"
  # The server's password policy; it refuses to start with anything else.
  if [[ -n "$opt_password" ]]; then
    ((${#opt_password} >= 8)) || die "--admin-password must be at least 8 characters"
    (($(printf '%s' "$opt_password" | wc -c) <= 72)) || die "--admin-password must be at most 72 bytes"
  fi
  if [[ -n "$opt_data_dir" ]]; then
    [[ "$opt_data_dir" =~ ^/[A-Za-z0-9._/-]+$ ]] || die "--data-dir must be an absolute path without spaces or quotes"
    opt_data_dir="${opt_data_dir%/}"
  fi
  if [[ -n "$opt_kubeconfig" && ! -r "$opt_kubeconfig" ]]; then
    die "cannot read --kubeconfig $opt_kubeconfig"
  fi
  if [[ -n "$opt_offline" && ! -e "$opt_offline" ]]; then
    die "--offline: $opt_offline does not exist"
  fi
  if $opt_purge && ! $opt_uninstall; then
    die "--purge only works together with --uninstall"
  fi
  return 0
}

# --- platform ----------------------------------------------------------------

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 rpm_arch=x86_64 ;;
    aarch64 | arm64) arch=arm64 rpm_arch=aarch64 ;;
    *) die "unsupported CPU architecture $(uname -m); KubeBench ships amd64 and arm64" ;;
  esac
}

macos_instructions() {
  local tag="$1" file
  file="kubebench_${tag#v}_darwin_$([[ "$(uname -m)" == arm64 ]] && echo arm64 || echo amd64).tar.gz"
  cat <<EOF
This installer sets up a Linux service. On macOS, run KubeBench from the
release archive as a local app against your kubeconfig:

  curl -fsSLO ${RELEASES}/download/${tag}/${file}
  tar -xzf ${file}
  KUBECONFIG=~/.kube/config ./kubebench      # then open http://localhost:9876

Sign in as admin/admin; you will be asked to choose a new password.
EOF
}

# deb_known: dpkg has a record of the package (installed or config-files).
deb_known() { have dpkg-query && dpkg-query -W kubebench >/dev/null 2>&1; }

# installed_method prints how KubeBench is installed: deb, rpm, tarball or none.
installed_method() {
  # shellcheck disable=SC2016 # ${db:Status-Status} is a dpkg-query format field
  if deb_known && [[ "$(dpkg-query -W -f='${db:Status-Status}' kubebench)" == installed ]]; then
    echo deb
  elif have rpm && rpm -q kubebench >/dev/null 2>&1; then
    echo rpm
  elif [[ -e "$TARBALL_BIN" || -e "$TARBALL_UNIT" ]]; then
    echo tarball
  else
    echo none
  fi
}

choose_method() {
  local current="$1"
  if [[ "$opt_method" != auto ]]; then
    echo "$opt_method"
  elif [[ "$current" != none ]]; then
    echo "$current"
  elif have dpkg; then
    echo deb
  elif have rpm; then
    echo rpm
  else
    echo tarball
  fi
}

binary_path() {
  if [[ "$1" == tarball ]]; then echo "$TARBALL_BIN"; else echo /usr/bin/kubebench; fi
}

# --- downloads and checksums -------------------------------------------------

fetch() { # url dest
  if have curl; then
    curl -fsSL --retry 3 --proto '=https' -o "$2" "$1"
  elif have wget; then
    wget -q -O "$2" "$1"
  else
    die "curl or wget is required to download releases (or use --offline)"
  fi
}

resolve_latest() {
  local url=""
  if have curl; then
    url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "${RELEASES}/latest")" || true
  elif have wget; then
    url="$(wget -q -S --spider --max-redirect=0 "${RELEASES}/latest" 2>&1 |
      awk '/^ *Location: / { print $2 }' | tr -d '\r' | tail -n 1)" || true
  else
    die "curl or wget is required to download releases (or use --offline)"
  fi
  [[ "$url" == */releases/tag/v* ]] || die "could not resolve the latest release from ${RELEASES}/latest"
  echo "${url##*/}"
}

sha256_of() {
  if have sha256sum; then
    sha256sum "$1" | awk '{ print $1 }'
  elif have shasum; then
    shasum -a 256 "$1" | awk '{ print $1 }'
  else
    die "sha256sum or shasum is required to verify downloads"
  fi
}

# artifact_name SUMS METHOD prints the release file for METHOD and this
# architecture, as listed in a GoReleaser checksums file.
artifact_name() {
  local pattern
  case "$2" in
    deb) pattern="^kubebench_.*_${arch}[.]deb\$" ;;
    rpm) pattern="^kubebench-.*[.]${rpm_arch}[.]rpm\$" ;;
    tarball) pattern="^kubebench_.*_linux_${arch}[.]tar[.]gz\$" ;;
  esac
  PATTERN="$pattern" awk '{ name = $2; sub(/^\*/, "", name) }
    name ~ ENVIRON["PATTERN"] { print name; exit }' "$1"
}

verify() { # file sums
  local name expected
  name="$(basename "$1")"
  expected="$(NAME="$name" awk '{ n = $2; sub(/^\*/, "", n) } n == ENVIRON["NAME"] { print $1; exit }' "$2")"
  [[ -n "$expected" ]] || die "$name is not listed in $2"
  [[ "$(sha256_of "$1")" == "$expected" ]] || die "SHA-256 mismatch for $name"
  log "verified SHA-256 of $name"
}

# --- locating the artifact ---------------------------------------------------

# resolve_artifact METHOD sets artifact and artifact_method, downloading and
# verifying the file when it comes from GitHub.
resolve_artifact() {
  if [[ -n "$opt_offline" ]]; then
    resolve_offline "$opt_offline" "$1"
  else
    resolve_online "$1"
  fi
}

resolve_online() {
  local method="$1" tag name
  if [[ "$opt_version" == latest ]]; then
    tag="$(resolve_latest)"
    log "latest release is $tag"
  else
    tag="v${opt_version#v}"
  fi
  log "downloading the checksums of KubeBench $tag"
  fetch "${RELEASES}/download/${tag}/checksums.txt" "$workdir/checksums.txt" ||
    die "release $tag not found (${RELEASES}/tag/${tag})"
  name="$(artifact_name "$workdir/checksums.txt" "$method")"
  [[ -n "$name" ]] || die "release $tag has no $method artifact for linux/$arch"
  artifact="$workdir/$name"
  artifact_method="$method"
  if $opt_dry_run; then
    log "[dry-run] would download and verify ${RELEASES}/download/${tag}/${name}"
    return 0
  fi
  log "downloading $name"
  fetch "${RELEASES}/download/${tag}/${name}" "$artifact"
  verify "$artifact" "$workdir/checksums.txt"
}

resolve_offline() {
  local path="$1" method="$2" sums
  if [[ -d "$path" ]]; then
    resolve_bundle_dir "$path" "$method"
    return 0
  fi
  case "$path" in
    *.deb) artifact_method=deb ;;
    *.rpm) artifact_method=rpm ;;
    *.tar.gz | *.tgz)
      # A release archive (kubebench_<v>_linux_<arch>.tar.gz) has the binary
      # at its root; anything else is an offline bundle.
      if [[ "$(basename "$path")" == kubebench_*_linux_*.tar.gz ]] ||
        tar -tzf "$path" kubebench >/dev/null 2>&1; then
        artifact_method=tarball
      else
        if [[ -f "$path.sha256" ]]; then
          verify "$path" "$path.sha256"
        else
          warn "no $(basename "$path").sha256 next to the bundle; only its packages are verified"
        fi
        log "extracting the packages of offline bundle $(basename "$path")"
        mkdir -p "$workdir/bundle"
        tar -xzf "$path" -C "$workdir/bundle" --wildcards '*/packages/*' 2>/dev/null ||
          tar -xzf "$path" -C "$workdir/bundle"
        resolve_bundle_dir "$workdir/bundle" "$method"
        return 0
      fi
      ;;
    *) die "--offline expects an offline bundle, a .deb, a .rpm or a kubebench_*_linux_*.tar.gz" ;;
  esac
  if [[ "$opt_method" != auto && "$opt_method" != "$artifact_method" ]]; then
    die "--method $opt_method does not match $(basename "$path")"
  fi
  artifact="$path"
  sums="$(dirname "$path")/checksums.txt"
  if [[ -f "$sums" ]]; then
    verify "$artifact" "$sums"
  else
    warn "no checksums.txt next to $(basename "$path"); skipping SHA-256 verification"
  fi
}

resolve_bundle_dir() {
  local dir="$1" method="$2" packages name
  packages="$(find "$dir" -maxdepth 2 -type d -name packages -print -quit)"
  [[ -n "$packages" ]] || packages="$dir"
  [[ -f "$packages/checksums.txt" ]] || die "no checksums.txt in $packages"
  name="$(artifact_name "$packages/checksums.txt" "$method")"
  [[ -n "$name" && -f "$packages/$name" ]] || die "the offline bundle has no $method package for linux/$arch"
  artifact="$packages/$name"
  artifact_method="$method"
  verify "$artifact" "$packages/checksums.txt"
}

# --- installing --------------------------------------------------------------

ensure_user() {
  getent group "$SERVICE_USER" >/dev/null 2>&1 || run groupadd --system "$SERVICE_USER"
  if ! getent passwd "$SERVICE_USER" >/dev/null 2>&1; then
    local shell=/usr/sbin/nologin
    [[ -x "$shell" ]] || shell=/sbin/nologin
    [[ -x "$shell" ]] || shell=/bin/false
    run useradd --system --gid "$SERVICE_USER" --home-dir "$DEFAULT_DATA_DIR" --no-create-home \
      --shell "$shell" --comment "KubeBench server" "$SERVICE_USER"
  fi
}

# remove_program METHOD removes an install's program files; configuration
# and data stay.
remove_program() {
  case "$1" in
    deb) run dpkg --remove kubebench ;;
    rpm) run rpm --erase kubebench ;;
    tarball)
      if systemd_running; then run systemctl disable --now "$UNIT" || true; fi
      run rm -f "$TARBALL_BIN" "$TARBALL_UNIT"
      ;;
  esac
}

# The packages must not start the service before install.sh has written the
# configuration (bootstrap password, port, kubeconfig): KUBEBENCH_SKIP_START.
install_package() {
  case "$1" in
    deb)
      have dpkg || die "dpkg is not available; try --method tarball"
      run env KUBEBENCH_SKIP_START=1 DEBIAN_FRONTEND=noninteractive \
        dpkg --force-confdef --force-confold --install "$artifact"
      ;;
    rpm)
      have rpm || die "rpm is not available; try --method tarball"
      run env KUBEBENCH_SKIP_START=1 rpm --upgrade --replacepkgs "$artifact"
      ;;
  esac
}

# install_tarball lays the archive out like the packages: binary, unit, env
# file, user and data directory (the binary goes to /usr/local/bin).
install_tarball() {
  local src="$workdir/tarball" file
  run mkdir -p "$src"
  run tar -xzf "$artifact" -C "$src"
  if ! $opt_dry_run; then
    for file in kubebench packaging/systemd/kubebench.service packaging/kubebench.env; do
      [[ -f "$src/$file" ]] || die "$(basename "$artifact") has no $file"
    done
  fi
  ensure_user
  run install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_USER" "$DEFAULT_DATA_DIR"
  run install -m 0755 "$src/kubebench" "$TARBALL_BIN"
  if $opt_dry_run; then
    log "[dry-run] would install $TARBALL_UNIT with ExecStart=$TARBALL_BIN"
  else
    mkdir -p "$(dirname "$TARBALL_UNIT")"
    sed "s#^ExecStart=/usr/bin/kubebench#ExecStart=$TARBALL_BIN#" \
      "$src/packaging/systemd/kubebench.service" >"$src/kubebench.service"
    install -m 0644 "$src/kubebench.service" "$TARBALL_UNIT"
  fi
  run install -d -m 0755 "$CONF_DIR"
  if [[ -e "$ENV_FILE" ]]; then
    log "keeping the existing $ENV_FILE"
  else
    run install -m 0640 -o root -g "$SERVICE_USER" "$src/packaging/kubebench.env" "$ENV_FILE"
  fi
}

# --- configuration -----------------------------------------------------------

# env_quote renders a value in double quotes that systemd and sh read alike.
env_quote() {
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  s="${s//\$/\\\$}"
  s="${s//\`/\\\`}"
  printf '"%s"' "$s"
}

# env_get KEY prints the active value of KEY in the env file, unquoted.
env_get() {
  [[ -f "$ENV_FILE" ]] || return 0
  KEY="$1" awk 'index($0, ENVIRON["KEY"] "=") == 1 {
      v = substr($0, length(ENVIRON["KEY"]) + 2)
      if (v ~ /^".*"$/ || v ~ /^\047.*\047$/) v = substr(v, 2, length(v) - 2)
      value = v
    }
    END { printf "%s", value }' "$ENV_FILE"
}

# env_set KEY VALUE replaces the active or commented-out KEY line (keeping
# its documentation in place) or appends one.
env_set() {
  local key="$1" line tmp
  line="$key=$(env_quote "$2")"
  if $opt_dry_run; then
    log "[dry-run] would set $key in $ENV_FILE"
    return 0
  fi
  tmp="$(mktemp "$ENV_FILE.XXXXXX")"
  KEY="$key" LINE="$line" awk '
    index($0, ENVIRON["KEY"] "=") == 1 || index($0, "#" ENVIRON["KEY"] "=") == 1 {
      if (!done) print ENVIRON["LINE"]
      done = 1
      next
    }
    { print }
    END { if (!done) print ENVIRON["LINE"] }' "$ENV_FILE" >"$tmp"
  chown "root:$SERVICE_USER" "$tmp"
  chmod 0640 "$tmp"
  mv -f "$tmp" "$ENV_FILE"
}

# data_dir_current prints the data directory of the existing install.
data_dir_current() {
  local dir=""
  if [[ -f "$DROPIN" ]]; then
    dir="$(sed -n 's/^Environment=KUBEBENCH_DATA_DIR=//p' "$DROPIN" | tail -n 1)"
  fi
  echo "${dir:-$DEFAULT_DATA_DIR}"
}

configure_data_dir() {
  local dir="$1"
  if [[ "$dir" == "$DEFAULT_DATA_DIR" ]]; then
    if [[ -f "$DROPIN" ]]; then run rm -f "$DROPIN"; fi
    return 0
  fi
  log "data directory: $dir"
  run install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_USER" "$dir"
  if $opt_dry_run; then
    log "[dry-run] would write $DROPIN"
    return 0
  fi
  mkdir -p "$DROPIN_DIR"
  cat >"$DROPIN" <<EOF
# Written by install.sh --data-dir: the unit's sandbox only allows writes here.
[Service]
Environment=KUBEBENCH_DATA_DIR=$dir
Environment=KUBEBENCH_DB=$dir/kubebench.db
WorkingDirectory=$dir
ReadWritePaths=$dir
EOF
  chmod 0644 "$DROPIN"
}

configure() {
  local data_dir="$1" db_exists=false
  if ! $opt_dry_run; then
    [[ -f "$ENV_FILE" ]] || die "$ENV_FILE is missing after the install"
    chown "root:$SERVICE_USER" "$ENV_FILE"
    chmod 0640 "$ENV_FILE"
  fi
  configure_data_dir "$data_dir"
  if [[ -n "$opt_port" ]]; then env_set PORT "$opt_port"; fi
  if [[ -n "$opt_listen" ]]; then env_set KUBEBENCH_LISTEN_ADDR "$opt_listen"; fi
  if [[ -n "$opt_kubeconfig" ]]; then
    run install -m 0640 -o root -g "$SERVICE_USER" "$opt_kubeconfig" "$KUBECONFIG_DEST"
    env_set KUBECONFIG "$KUBECONFIG_DEST"
  fi
  # An offline install has no route to GitHub: no update checks, unless the
  # operator chose a mode in the env file before.
  if [[ -n "$opt_offline" && -z "$(env_get KUBEBENCH_UPDATE_MODE)" ]]; then
    env_set KUBEBENCH_UPDATE_MODE off
  fi

  # The bootstrap password only matters while there is no database yet.
  [[ -e "$data_dir/kubebench.db" ]] && db_exists=true
  if [[ -n "$opt_password" ]]; then
    env_set KUBEBENCH_ADMIN_PASSWORD "$opt_password"
    if $db_exists; then
      warn "the database already exists, so --admin-password has no effect; change the password in the UI"
    fi
  elif [[ -z "$(env_get KUBEBENCH_ADMIN_PASSWORD)" ]] && ! $db_exists; then
    generated_password="$(random_password)"
    env_set KUBEBENCH_ADMIN_PASSWORD "$generated_password"
  fi
}

random_password() {
  local chars=""
  while ((${#chars} < 20)); do
    chars+="$(head -c 64 /dev/urandom | LC_ALL=C tr -dc 'A-Za-z0-9')"
  done
  printf '%s' "${chars:0:20}"
}

# --- service -----------------------------------------------------------------

# healthy BIN reports whether the running server answers `kubebench
# healthcheck`, which reads the same env file and probes /healthz on the
# listen address (loopback when it listens on all interfaces).
healthy() {
  (
    set +u -a
    # shellcheck disable=SC1090 # the env file is written by this installer
    . "$ENV_FILE"
    exec "$1" healthcheck
  ) >/dev/null 2>&1
}

start_service() {
  local bin="$1" data_dir="$2" as_user i
  if ! systemd_running; then
    as_user="runuser -u $SERVICE_USER --"
    have runuser || as_user="sudo -u $SERVICE_USER"
    log "systemd is not running here; start KubeBench in the foreground with:"
    printf "  %s sh -c 'set -a; KUBEBENCH_DATA_DIR=%s KUBEBENCH_DB=%s/kubebench.db; . %s; cd %s && exec %s'\n" \
      "$as_user" "$data_dir" "$data_dir" "$ENV_FILE" "$data_dir" "$bin" >&2
    return 0
  fi
  run systemctl daemon-reload
  if ! $opt_start; then
    log "not starting the service (--no-start); start it with: systemctl enable --now kubebench"
    return 0
  fi
  run systemctl enable "$UNIT"
  run systemctl restart "$UNIT"
  $opt_dry_run && return 0
  log "waiting for KubeBench to become healthy"
  for ((i = 0; i < 30; i++)); do
    if systemctl is-active --quiet "$UNIT" && healthy "$bin"; then
      return 0
    fi
    sleep 1
  done
  die "kubebench.service is not healthy; see: journalctl -u kubebench -n 50"
}

summary() {
  local method="$1" bin="$2" data_dir="$3" version listen port host scheme=http
  version="$("$bin" --version 2>/dev/null || echo "kubebench (unknown version)")"
  listen="$(env_get KUBEBENCH_LISTEN_ADDR)"
  port="$(env_get PORT)"
  if [[ -n "$listen" ]]; then
    port="${listen##*:}"
    host="${listen%:*}"
  fi
  case "${host:-}" in
    "" | 0.0.0.0 | "[::]") host="$(hostname -I 2>/dev/null | awk '{ print $1 }')" || host="" ;;
  esac
  if [[ -n "$(env_get KUBEBENCH_TLS_CERT)" ]]; then scheme=https; fi
  cat <<EOF

$version installed ($method).
  URL:           $scheme://${host:-localhost}:${port:-9876}
  Configuration: $ENV_FILE
  Data:          $data_dir
EOF
  if systemd_running; then
    echo "  Logs:          journalctl -u kubebench -f"
  fi
  if [[ -n "$generated_password" ]]; then
    cat <<EOF

Sign in as "admin" with this generated password (shown only once; it stays
in $ENV_FILE until you remove it), then choose a new one:

  $generated_password

EOF
  fi
}

# --- uninstall ---------------------------------------------------------------

confirm() {
  local answer=""
  $opt_yes && return 0
  if ! (exec </dev/tty) 2>/dev/null; then
    die "$1 Re-run with --yes to confirm without a terminal."
  fi
  printf '%s [y/N] ' "$1" >/dev/tty
  read -r answer </dev/tty || true
  [[ "$answer" =~ ^[Yy]([Ee][Ss])?$ ]] || die "aborted"
}

uninstall() {
  local method data_dir
  method="$(installed_method)"
  data_dir="$(data_dir_current)"
  if $opt_purge; then
    confirm "This deletes $CONF_DIR, $data_dir and the $SERVICE_USER user. Continue?"
  fi
  if [[ "$method" == none ]]; then
    log "KubeBench is not installed"
  else
    log "removing the KubeBench $method install"
  fi
  if [[ "$method" != none ]]; then remove_program "$method"; fi
  # Also purges a package that was removed earlier without --purge.
  if $opt_purge && deb_known; then run dpkg --purge kubebench; fi
  if [[ -d "$DROPIN_DIR" ]]; then run rm -rf "$DROPIN_DIR"; fi
  if systemd_running; then run systemctl daemon-reload; fi
  if $opt_purge; then
    run rm -rf "$CONF_DIR" "$data_dir" "$DEFAULT_DATA_DIR"
    if getent passwd "$SERVICE_USER" >/dev/null 2>&1; then run userdel "$SERVICE_USER"; fi
    if getent group "$SERVICE_USER" >/dev/null 2>&1; then run groupdel "$SERVICE_USER"; fi
    log "KubeBench, its configuration and its data are removed"
  else
    log "configuration ($CONF_DIR) and data ($data_dir) are kept; --purge deletes them"
  fi
}

# --- main --------------------------------------------------------------------

main() {
  local tag current method data_dir bin
  parse_args "$@"

  case "$(uname -s)" in
    Linux) ;;
    Darwin)
      tag="$opt_version"
      if [[ "$tag" == latest ]]; then tag="$(resolve_latest)"; else tag="v${tag#v}"; fi
      macos_instructions "$tag"
      exit 0
      ;;
    *) die "unsupported OS $(uname -s); see ${RELEASES} for the archives" ;;
  esac
  if ((EUID != 0)) && ! $opt_dry_run; then
    die "run as root, e.g.: curl -fsSL .../install.sh | sudo bash"
  fi
  detect_arch

  if $opt_uninstall; then
    uninstall
    return 0
  fi

  workdir="$(mktemp -d)"
  trap cleanup EXIT

  current="$(installed_method)"
  resolve_artifact "$(choose_method "$current")"
  method="$artifact_method"
  data_dir="${opt_data_dir:-$(data_dir_current)}"

  if [[ "$current" != none && "$current" != "$method" ]]; then
    log "replacing the $current install with the $method one (configuration and data stay)"
    remove_program "$current"
  fi
  log "installing $(basename "$artifact") ($method)"
  if [[ "$method" == tarball ]]; then
    install_tarball
  else
    install_package "$method"
  fi

  bin="$(binary_path "$method")"
  if ! $opt_dry_run; then
    "$bin" --version >/dev/null || die "$bin does not run on this host"
  fi
  configure "$data_dir"
  start_service "$bin" "$data_dir"
  if $opt_dry_run; then
    log "dry run complete; nothing was changed"
  else
    summary "$method" "$bin" "$data_dir"
  fi
}

main "$@"
