#!/bin/ash
set -e

for envvar in \
  USERNAME \
  APIKEY \
  HOSTNAME_ID \
; do
  if [ -z "${!envvar}" ]; then
    echo "\$${envvar} is not set!"
    exit 1
  fi
done

curl https://${USERNAME}:${APIKEY}@ipv4.tunnelbroker.net/nic/update?hostname=${HOSTNAME_ID}
sleep ${DELAY}
