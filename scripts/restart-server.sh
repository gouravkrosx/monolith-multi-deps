#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
pkill -f 'bin/server' 2>/dev/null || true
sleep 0.5
go build -o bin/server ./cmd/server
set -a; source .env; set +a
nohup ./bin/server > /tmp/taskmgr.log 2>&1 &
for _ in $(seq 1 30); do
  if curl -fs -m 1 http://localhost:8080/health >/dev/null 2>&1; then
    echo "server up"
    exit 0
  fi
  sleep 0.5
done
echo "server failed to come up" >&2
tail -50 /tmp/taskmgr.log >&2
exit 1
