#!/bin/sh
set -eu

curl --show-error \
  --request POST \
  --header 'Content-Type: application/json' \
  --data '{"event_id":"evt-42","order_id":"ord-42","asset":{"id":"asset-9","seller_id":"seller-3","state":"ready"},"buyer_update":{"buyer_id":"buyer-8","confirmed":true}}' \
  "http://localhost:${PORT:-8080}"
