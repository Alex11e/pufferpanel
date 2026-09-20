#!/usr/bin/env sh
set -eu

# Installs the panel from a checked-out copy of this repository. It keeps all
# mutable panel data under ./data so upgrading the checkout does not lose data.
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
COMPOSE="$ROOT/deploy/docker-compose.yml"

ask_yes_no() {
  printf '%s [i/N]: ' "$1" >&2
  read -r answer </dev/tty || return 1
  case "$answer" in [iI]|[iI][gG][eE][nN]|[yY]|[yY][eE][sS]) return 0 ;; *) return 1 ;; esac
}

remove_if_confirmed() {
  label="$1" target="$2"
  if [ -e "$target" ] && ask_yes_no "Törlöd ezt: $label ($target)?"; then rm -rf -- "$target"; else echo "Megőrizve: $label"; fi
}

create_user() {
  ask_yes_no "Létrehozol most egy PufferPanel felhasználót?" || return 0
  printf 'Felhasználónév: ' >&2; read -r username </dev/tty
  printf 'E-mail cím: ' >&2; read -r email </dev/tty
  printf 'Jelszó: ' >&2; stty -echo </dev/tty; read -r password </dev/tty; stty echo </dev/tty; printf '\n' >&2
  printf 'Jelszó újra: ' >&2; stty -echo </dev/tty; read -r confirm </dev/tty; stty echo </dev/tty; printf '\n' >&2
  if [ -z "$username" ] || [ -z "$email" ] || [ -z "$password" ]; then echo "A felhasználónév, e-mail cím és jelszó kötelező." >&2; return 1; fi
  if [ "$password" != "$confirm" ]; then echo "A két jelszó nem egyezik." >&2; return 1; fi
  echo "1) Normál felhasználó"; echo "2) Adminisztrátor"; printf 'Szerepkör: ' >&2; read -r role </dev/tty
  case "$role" in
    1) docker exec pufferpanel /pufferpanel/bin/pufferpanel user add --name "$username" --email "$email" --password "$password" ;;
    2) docker exec pufferpanel /pufferpanel/bin/pufferpanel user add --name "$username" --email "$email" --password "$password" --admin ;;
    *) echo "Érvénytelen szerepkör." >&2; return 1 ;;
  esac
  unset password confirm
  echo "Felhasználó létrehozva: $username"
}

uninstall_panel() {
  if command -v docker >/dev/null 2>&1 && ask_yes_no "Leállítod és eltávolítod a PufferPanel konténert?"; then docker compose -f "$COMPOSE" down --remove-orphans 2>/dev/null || true; fi
  if command -v docker >/dev/null 2>&1 && ask_yes_no "Törlöd a pufferpanel-custom:latest Docker image-et?"; then docker image rm pufferpanel-custom:latest 2>/dev/null || true; fi
  remove_if_confirmed "konfiguráció" "$ROOT/data/config"
  remove_if_confirmed "szerveradatok és mentések" "$ROOT/data/data"
  remove_if_confirmed "naplók" "$ROOT/data/logs"
  if [ -d "$ROOT" ] && ask_yes_no "Törlöd a panel programfájljait? (A meg nem erősített data elemek megmaradnak.)"; then find "$ROOT" -mindepth 1 -maxdepth 1 ! -name data -exec rm -rf -- {} +; fi
}

echo "1) PufferPanel eltávolítása"
echo "2) PufferPanel telepítése vagy frissítése"
printf 'Választás: ' >&2
read -r INSTALL_ACTION </dev/tty
case "$INSTALL_ACTION" in 1) uninstall_panel; exit 0 ;; 2) ;; *) echo "Érvénytelen választás." >&2; exit 1 ;; esac

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required. Install Docker Engine and the Docker Compose plugin first." >&2
  exit 1
fi
if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose v2 is required." >&2
  exit 1
fi

mkdir -p "$ROOT/data/config" "$ROOT/data/data/backups" "$ROOT/data/data/servers" "$ROOT/data/data/binaries" "$ROOT/data/data/cache" "$ROOT/data/logs"
if [ ! -f "$ROOT/data/config/config.json" ]; then
  cp "$ROOT/config.docker.json" "$ROOT/data/config/config.json"
fi

if ! docker compose -f "$COMPOSE" up -d --build --wait --wait-timeout 120; then
  echo "A panel nem indult el. Az alábbi napló segít megtalálni a hibát:" >&2
  docker compose -f "$COMPOSE" ps >&2 || true
  docker compose -f "$COMPOSE" logs --tail 200 pufferpanel >&2 || true
  exit 1
fi
create_user
echo "PufferPanel is available at http://localhost:8080"
