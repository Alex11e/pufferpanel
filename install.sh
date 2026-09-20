#!/usr/bin/env sh
set -eu

# Installs the panel from a checked-out copy of this repository. It keeps all
# mutable panel data under ./data so upgrading the checkout does not lose data.
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
COMPOSE="$ROOT/deploy/docker-compose.yml"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required. Install Docker Engine and the Docker Compose plugin first." >&2
  exit 1
fi
if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose v2 is required." >&2
  exit 1
fi

mkdir -p "$ROOT/data/config" "$ROOT/data/data" "$ROOT/data/logs"
if [ ! -f "$ROOT/data/config/config.json" ]; then
  cp "$ROOT/config.docker.json" "$ROOT/data/config/config.json"
fi

docker compose -f "$COMPOSE" up -d --build
echo "PufferPanel is available at http://localhost:8080"
