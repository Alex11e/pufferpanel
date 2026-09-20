#!/usr/bin/env bash
set -euo pipefail

# Remote bootstrapper. Intended for: bash <(curl -fsSL URL)
# It only supports Debian/Ubuntu because it installs Docker using apt.
REPOSITORY="https://github.com/Alex11e/pufferpanel.git"
BRANCH="v3"
INSTALL_DIR="/opt/pufferpanel"

ask_yes_no() {
  local answer
  printf '%s [i/N]: ' "$1" >/dev/tty
  read -r answer </dev/tty || return 1
  case "$answer" in [iI]|[iI][gG][eE][nN]|[yY]|[yY][eE][sS]) return 0 ;; *) return 1 ;; esac
}

remove_if_confirmed() {
  local label="$1" target="$2"
  if [[ -e "$target" ]] && ask_yes_no "Törlöd ezt: $label ($target)?"; then
    $SUDO rm -rf -- "$target"
    echo "Törölve: $label"
  else
    echo "Megőrizve: $label"
  fi
}

create_user() {
  local username email password confirm role
  ask_yes_no "Létrehozol most egy PufferPanel felhasználót?" || return 0

  printf 'Felhasználónév: ' >/dev/tty
  read -r username </dev/tty
  printf 'E-mail cím: ' >/dev/tty
  read -r email </dev/tty
  printf 'Jelszó: ' >/dev/tty
  read -r -s password </dev/tty
  printf '\nJelszó újra: ' >/dev/tty
  read -r -s confirm </dev/tty
  printf '\n' >/dev/tty

  if [[ -z "$username" || -z "$email" || -z "$password" ]]; then
    echo "A felhasználónév, e-mail cím és jelszó kötelező." >&2
    return 1
  fi
  if [[ "$password" != "$confirm" ]]; then
    echo "A két jelszó nem egyezik." >&2
    return 1
  fi

  echo "1) Normál felhasználó"
  echo "2) Adminisztrátor"
  printf 'Szerepkör: ' >/dev/tty
  read -r role </dev/tty
  case "$role" in
    1) $SUDO docker exec pufferpanel /pufferpanel/bin/pufferpanel user add --name "$username" --email "$email" --password "$password" ;;
    2) $SUDO docker exec pufferpanel /pufferpanel/bin/pufferpanel user add --name "$username" --email "$email" --password "$password" --admin ;;
    *) echo "Érvénytelen szerepkör." >&2; return 1 ;;
  esac
  unset password confirm
  echo "Felhasználó létrehozva: $username"
}

uninstall_panel() {
  echo "PufferPanel teljes eltávolítás — minden elemhez külön megerősítés kell."
  if command -v docker >/dev/null 2>&1 && ask_yes_no "Leállítod és eltávolítod a PufferPanel konténert?"; then
    $SUDO docker compose -f "$INSTALL_DIR/deploy/docker-compose.yml" down --remove-orphans 2>/dev/null || true
  fi
  if command -v docker >/dev/null 2>&1 && ask_yes_no "Törlöd a pufferpanel-custom:latest Docker image-et?"; then
    $SUDO docker image rm pufferpanel-custom:latest 2>/dev/null || true
  fi
  remove_if_confirmed "konfiguráció" "$INSTALL_DIR/data/config"
  remove_if_confirmed "szerveradatok és mentések" "$INSTALL_DIR/data/data"
  remove_if_confirmed "naplók" "$INSTALL_DIR/data/logs"
  if [[ -d "$INSTALL_DIR" ]] && ask_yes_no "Törlöd a panel programfájljait? (A meg nem erősített data elemek megmaradnak.)"; then
    find "$INSTALL_DIR" -mindepth 1 -maxdepth 1 ! -name data -exec $SUDO rm -rf -- {} +
  fi
  if [[ -d "$INSTALL_DIR/data" ]] && [[ -z "$(find "$INSTALL_DIR/data" -mindepth 1 -print -quit)" ]]; then
    $SUDO rmdir "$INSTALL_DIR/data" 2>/dev/null || true
  fi
  if [[ -d "$INSTALL_DIR" ]] && [[ -z "$(find "$INSTALL_DIR" -mindepth 1 -print -quit)" ]]; then
    $SUDO rmdir "$INSTALL_DIR" 2>/dev/null || true
  fi
  echo "Az eltávolítás befejeződött."
}

if [[ $EUID -eq 0 ]]; then SUDO=""; else SUDO="sudo"; fi
if ! command -v apt-get >/dev/null 2>&1; then
  echo "This installer currently supports Debian and Ubuntu only." >&2
  exit 1
fi
if [[ -n "$SUDO" ]] && ! command -v sudo >/dev/null 2>&1; then
  echo "Run this command as root or install sudo first." >&2
  exit 1
fi

echo "1) PufferPanel eltávolítása"
echo "2) PufferPanel telepítése vagy frissítése"
printf 'Választás: ' >/dev/tty
read -r INSTALL_ACTION </dev/tty
case "$INSTALL_ACTION" in
  1) uninstall_panel; exit 0 ;;
  2) ;;
  *) echo "Érvénytelen választás." >&2; exit 1 ;;
esac

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
	$SUDO git -C "$INSTALL_DIR" remote set-url origin "$REPOSITORY"
	$SUDO git -C "$INSTALL_DIR" fetch origin "$BRANCH"
  $SUDO git -C "$INSTALL_DIR" checkout "$BRANCH"
  $SUDO git -C "$INSTALL_DIR" pull --ff-only origin "$BRANCH"
else
  $SUDO git clone --branch "$BRANCH" --single-branch "$REPOSITORY" "$INSTALL_DIR"
fi

$SUDO mkdir -p "$INSTALL_DIR/data/config" "$INSTALL_DIR/data/data/backups" "$INSTALL_DIR/data/data/servers" "$INSTALL_DIR/data/data/binaries" "$INSTALL_DIR/data/data/cache" "$INSTALL_DIR/data/logs"
if [[ ! -f "$INSTALL_DIR/data/config/config.json" ]]; then
  $SUDO cp "$INSTALL_DIR/config.docker.json" "$INSTALL_DIR/data/config/config.json"
fi

if ! $SUDO docker compose -f "$INSTALL_DIR/deploy/docker-compose.yml" up -d --build --wait --wait-timeout 120; then
  echo "A panel nem indult el. Az alábbi napló segít megtalálni a hibát:" >&2
  $SUDO docker compose -f "$INSTALL_DIR/deploy/docker-compose.yml" ps >&2 || true
  $SUDO docker compose -f "$INSTALL_DIR/deploy/docker-compose.yml" logs --tail 200 pufferpanel >&2 || true
  exit 1
fi
create_user
echo "PufferPanel is ready at http://$(hostname -I | awk '{print $1}'):8080"
