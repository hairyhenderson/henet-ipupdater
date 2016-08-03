#!/bin/ash
set -e

if [ -z "$USERNAME" ]; then
  echo "\$USERNAME is not set!"
  exit 1
fi
if [ -z "$APIKEY" ]; then
  echo "\$APIKEY is not set!"
  exit 1
fi
if [ -z "$HOSTNAME_ID" ]; then
  echo "\$HOSTNAME_ID is not set!"
  exit 1
fi

curl -s https://${USERNAME}:${APIKEY}@ipv4.tunnelbroker.net/nic/update?hostname=${HOSTNAME_ID}
sleep ${DELAY}
