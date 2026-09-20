#!/usr/bin/env bash
set -euo pipefail

# Remote bootstrapper. Intended for: bash <(curl -fsSL URL)
# It only supports Debian/Ubuntu because it installs Docker using apt.
REPOSITORY="https://github.com/Alex11e/pufferpanel.git"
BRANCH="v3"
INSTALL_DIR="/opt/pufferpanel"

if [[ $EUID -eq 0 ]]; then SUDO=""; else SUDO="sudo"; fi
if ! command -v apt-get >/dev/null 2>&1; then
  echo "This installer currently supports Debian and Ubuntu only." >&2
  exit 1
fi
if [[ -n "$SUDO" ]] && ! command -v sudo >/dev/null 2>&1; then
  echo "Run this command as root or install sudo first." >&2
  exit 1
fi

$SUDO apt-get update
$SUDO apt-get install -y ca-certificates curl git
if ! command -v docker >/dev/null 2>&1; then
  echo "Installing Docker Engine and Docker Compose..."
  curl -fsSL https://get.docker.com | $SUDO sh
fi
if ! $SUDO docker compose version >/dev/null 2>&1; then
  echo "Docker Compose v2 is required but unavailable after Docker installation." >&2
  exit 1
fi

if [[ -d "$INSTALL_DIR/.git" ]]; then
  $SUDO git -C "$INSTALL_DIR" fetch origin "$BRANCH"
  $SUDO git -C "$INSTALL_DIR" checkout "$BRANCH"
  $SUDO git -C "$INSTALL_DIR" pull --ff-only origin "$BRANCH"
else
  $SUDO git clone --branch "$BRANCH" --single-branch "$REPOSITORY" "$INSTALL_DIR"
fi

$SUDO mkdir -p "$INSTALL_DIR/data/config" "$INSTALL_DIR/data/data" "$INSTALL_DIR/data/logs"
if [[ ! -f "$INSTALL_DIR/data/config/config.json" ]]; then
  $SUDO cp "$INSTALL_DIR/config.docker.json" "$INSTALL_DIR/data/config/config.json"
fi
$SUDO docker compose -f "$INSTALL_DIR/deploy/docker-compose.yml" up -d --build
echo "PufferPanel is ready at http://$(hostname -I | awk '{print $1}'):8080"
