#!/usr/bin/env bash
# OpenPaaS installer.
#
#   curl -fsSL https://raw.githubusercontent.com/diezo/OpenPaaS/main/install.sh | sudo bash
#
# Options (environment variables):
#   OP_VERSION=v0.1.0   install a specific release instead of the latest
#   OP_BINARY=./op      install a local binary (for development; skips download)
#   OP_REPO=owner/repo  install from a fork
#
# Safe to re-run: it upgrades op and skips anything already installed.
set -euo pipefail

OP_REPO="${OP_REPO:-diezo/OpenPaaS}"
OP_VERSION="${OP_VERSION:-latest}"
INSTALL_PATH="/usr/local/bin/op"

if [ -t 1 ]; then
  B=$'\033[1m'; G=$'\033[32m'; R=$'\033[31m'; C=$'\033[36m'; D=$'\033[2m'; N=$'\033[0m'
else
  B=""; G=""; R=""; C=""; D=""; N=""
fi
step() { printf '%s›%s %s\n' "$C" "$N" "$*"; }
ok()   { printf '%s✓%s %s\n' "$G" "$N" "$*"; }
die()  { printf '%s✗ %s%s\n' "$R" "$*" "$N" >&2; exit 1; }

LOG=/var/log/openpaas/install.log
quiet() { "$@" >>"$LOG" 2>&1 || die "Command failed: $* (see $LOG)"; }

# ---- checks ---------------------------------------------------------------
[ "$(id -u)" -eq 0 ] || die "Please run as root, e.g.: curl -fsSL <url> | sudo bash"

[ -r /etc/os-release ] || die "Unsupported system: /etc/os-release not found"
. /etc/os-release
case " ${ID:-} ${ID_LIKE:-} " in
  *" debian "*|*" ubuntu "*) ;;
  *) die "OpenPaaS currently supports Debian and Ubuntu only (found: ${PRETTY_NAME:-unknown})" ;;
esac

case "$(uname -m)" in
  x86_64|amd64)  ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "Unsupported CPU architecture: $(uname -m)" ;;
esac

command -v systemctl >/dev/null || die "systemd is required"

mkdir -p /var/log/openpaas
chmod 750 /var/log/openpaas
echo "===== install $(date -Is) =====" >>"$LOG"

printf '\n%sInstalling OpenPaaS%s %s(%s, %s)%s\n\n' "$B" "$N" "$D" "${PRETTY_NAME:-linux}" "$ARCH" "$N"

# ---- system packages ------------------------------------------------------
export DEBIAN_FRONTEND=noninteractive
step "Updating package lists"
quiet apt-get update -y

step "Installing nginx, certbot and tools"
quiet apt-get install -y --no-install-recommends \
  nginx certbot python3-certbot-nginx curl ca-certificates gnupg tar
quiet systemctl enable --now nginx
ok "nginx + certbot ready"

# ---- docker ---------------------------------------------------------------
if command -v docker >/dev/null 2>&1; then
  ok "Docker already installed ($(docker --version 2>/dev/null | cut -d, -f1))"
else
  step "Installing Docker (official repository)"
  # Ubuntu and its derivatives use Docker's ubuntu repo; everything else uses debian.
  case " ${ID} ${ID_LIKE:-} " in
    *" ubuntu "*) DOCKER_DISTRO=ubuntu ;;
    *)            DOCKER_DISTRO=debian ;;
  esac
  CODENAME="${UBUNTU_CODENAME:-${VERSION_CODENAME:-}}"
  [ -n "$CODENAME" ] || die "Couldn't determine the distro codename for Docker's repository"

  install -m 0755 -d /etc/apt/keyrings
  quiet curl -fsSL "https://download.docker.com/linux/${DOCKER_DISTRO}/gpg" -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/${DOCKER_DISTRO} ${CODENAME} stable" \
    > /etc/apt/sources.list.d/docker.list
  quiet apt-get update -y
  quiet apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  ok "Docker installed"
fi
quiet systemctl enable --now docker

# ---- op binary ------------------------------------------------------------
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if [ -n "${OP_BINARY:-}" ]; then
  [ -f "$OP_BINARY" ] || die "OP_BINARY not found: $OP_BINARY"
  cp "$OP_BINARY" "$TMP/op"
  ok "Using local binary $OP_BINARY"
else
  ASSET="op_linux_${ARCH}.tar.gz"
  if [ "$OP_VERSION" = latest ]; then
    BASE="https://github.com/${OP_REPO}/releases/latest/download"
  else
    BASE="https://github.com/${OP_REPO}/releases/download/${OP_VERSION}"
  fi
  step "Downloading op ($OP_VERSION)"
  curl -fsSL "$BASE/$ASSET" -o "$TMP/$ASSET" || die "Download failed: $BASE/$ASSET"
  curl -fsSL "$BASE/checksums.txt" -o "$TMP/checksums.txt" || die "Download failed: $BASE/checksums.txt"
  (cd "$TMP" && grep " ${ASSET}\$" checksums.txt | sha256sum -c --status) \
    || die "Checksum verification failed for $ASSET"
  tar -xzf "$TMP/$ASSET" -C "$TMP" op
  ok "Downloaded and verified"
fi

install -m 0755 "$TMP/op" "$INSTALL_PATH"
ok "Installed $("$INSTALL_PATH" version)"

mkdir -p /etc/openpaas
chmod 700 /etc/openpaas

# Upgrades: restart the API so it runs the new binary.
if systemctl is-enabled --quiet openpaas-api 2>/dev/null; then
  quiet systemctl restart openpaas-api
  ok "Restarted the deploy API"
fi

printf '\n%s✓ OpenPaaS installed.%s Next, run:\n\n    %ssudo op setup%s\n\n' "$G$B" "$N" "$B" "$N"
