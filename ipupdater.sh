#!/bin/ash
set -e

if [ -z "$APIKEY" ]; then
  echo "\$APIKEY is not set!"
  exit 1
fi
if [ -z "$HOSTNAME" ]; then
  echo "\$HOSTNAME is not set!"
  exit 1
fi

curl -s https://${HOSTNAME}:${APIKEY}@dyn.dns.he.net/nic/update?hostname=${HOSTNAME}
sleep ${DELAY}
