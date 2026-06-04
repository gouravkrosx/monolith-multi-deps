#!/usr/bin/env bash
# Seed script for Keploy recording: taskmanager-grpc
# Drives the gRPC API (:9090) via cmd/grpcclient: login → create → list → get →
# update → assign → unassign → delete. Requires users alice_seed/bob_seed in DB
# (created by taskmanager-crud/seed.sh) and bin/grpcclient built.
set -euo pipefail
BIN="${BIN:-./bin/grpcclient}"
ADDR="${ADDR:-localhost:9090}"

tok() { jq -r '.access_token // .AccessToken'; }

echo "== gRPC login =="
ALICE_LOGIN=$($BIN login -addr "$ADDR" -username alice_seed -password alicepass123)
echo "$ALICE_LOGIN"
ALICE_TOKEN=$(echo "$ALICE_LOGIN" | tok)
sleep 0.5
BOB_LOGIN=$($BIN login -addr "$ADDR" -username bob_seed -password bobpass123)
BOB_ID=$(echo "$BOB_LOGIN" | jq -r '.user_id // .UserID // .UserId')
sleep 0.5

echo "== gRPC task CRUD =="
TASK=$($BIN create -addr "$ADDR" -token "$ALICE_TOKEN" -title "gRPC demo task" -desc "created over grpc" -priority high -due "2026-06-20T12:00:00Z")
echo "$TASK"
TASK_ID=$(echo "$TASK" | jq -r '.id // .ID // .Id')
sleep 0.5
$BIN list   -addr "$ADDR" -token "$ALICE_TOKEN" -limit 50 -offset 0; sleep 0.5
$BIN get    -addr "$ADDR" -token "$ALICE_TOKEN" -id "$TASK_ID"; sleep 0.5
$BIN update -addr "$ADDR" -token "$ALICE_TOKEN" -id "$TASK_ID" -status in_progress -priority urgent; sleep 0.5

echo "== gRPC assign/unassign =="
$BIN assign   -addr "$ADDR" -token "$ALICE_TOKEN" -id "$TASK_ID" -user "$BOB_ID"; sleep 0.5
$BIN unassign -addr "$ADDR" -token "$ALICE_TOKEN" -id "$TASK_ID" -user "$BOB_ID"; sleep 0.5

echo "== gRPC delete =="
$BIN delete -addr "$ADDR" -token "$ALICE_TOKEN" -id "$TASK_ID"; sleep 0.5

echo "== grpc seed done =="
