#!/usr/bin/env sh

# Bind-mounted host folders replace the directories created in the image. Make
# the runtime layout idempotent so a first install always has every required
# daemon and backup directory.
mkdir -p \
  /var/lib/pufferpanel/backups \
  /var/lib/pufferpanel/servers \
  /var/lib/pufferpanel/binaries \
  /var/lib/pufferpanel/cache \
  /var/log/pufferpanel

/pufferpanel/bin/pufferpanel db upgrade
exitCode=$?
[ $exitCode -eq 0 ] || [ $exitCode -eq 9 ] || exit $exitCode

exec /pufferpanel/bin/pufferpanel run
