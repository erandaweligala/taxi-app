#!/usr/bin/env bash
# End-to-end smoke test of the running stack. Brings a handful of drivers online
# near a point, requests a ride, and walks the trip through its lifecycle.
#
# Usage: ./scripts/demo.sh   (requires the stack from `make up` and `jq`)
set -euo pipefail

API="${API:-http://localhost:8080}"

# A point in Lagos; drivers are scattered within a few hundred metres.
LAT=6.5244
LNG=3.3792

echo "==> Bringing 5 drivers online near (${LAT}, ${LNG})"
for i in 1 2 3 4 5; do
  dlat=$(awk "BEGIN{print ${LAT} + ($i-3)*0.001}")
  dlng=$(awk "BEGIN{print ${LNG} + ($i-3)*0.001}")
  curl -fsS -X POST "${API}/v1/drivers/driver-${i}/location" \
    -H 'content-type: application/json' \
    -d "{\"lat\":${dlat},\"lng\":${dlng},\"available\":true}" >/dev/null
  echo "    driver-${i} online at (${dlat}, ${dlng})"
done

echo "==> Waiting for the location worker to index them"
sleep 2

echo "==> Checking surge for the region"
curl -fsS "${API}/v1/surge?lat=${LAT}&lng=${LNG}" | jq .

echo "==> Requesting a ride for rider-1"
RESP=$(curl -fsS -X POST "${API}/v1/rides" \
  -H 'content-type: application/json' \
  -d "{\"rider_id\":\"rider-1\",\"lat\":${LAT},\"lng\":${LNG}}")
echo "${RESP}" | jq .

MATCHED=$(echo "${RESP}" | jq -r .matched)
if [ "${MATCHED}" != "true" ]; then
  echo "!! no driver matched — is the location worker running?"
  exit 1
fi

TRIP=$(echo "${RESP}" | jq -r .trip.id)
echo "==> Matched trip ${TRIP}; walking the lifecycle"
curl -fsS -X POST "${API}/v1/trips/${TRIP}/start"    | jq -c .
curl -fsS -X POST "${API}/v1/trips/${TRIP}/complete" | jq -c .

echo "==> Final trip state (from cache):"
curl -fsS "${API}/v1/trips/${TRIP}" | jq .

echo "==> Done. Tip: connect a socket to ws://localhost:8090/ws?user_id=rider-1"
echo "    before requesting a ride to watch live notifications."
