#!/usr/bin/env bash
# Seed script for Keploy recording: taskmanager-crud
# Exercises the REST surface in CRUD order, chaining ids from responses.
# Requires: curl, jq. App at $BASE with fresh DB (bootstrap admin:admin12345).
set -euo pipefail
BASE="${BASE:-http://localhost:8080}"

req() { # method path token data
  local method=$1 path=$2 token=${3:-} data=${4:-}
  local args=(-s -X "$method" "$BASE$path" -H 'Content-Type: application/json')
  [ -n "$token" ] && args+=(-H "Authorization: Bearer $token")
  [ -n "$data" ] && args+=(-d "$data")
  curl "${args[@]}"
  sleep 0.5
}

echo "== health =="
req GET /health

echo "== register users (distinct data per request) =="
ALICE=$(req POST /api/v1/auth/register '' '{"username":"alice_seed","email":"alice_seed@example.com","password":"alicepass123","external_id":1}')
ALICE_ID=$(echo "$ALICE" | jq -r .id)
BOB=$(req POST /api/v1/auth/register '' '{"username":"bob_seed","email":"bob_seed@example.com","password":"bobpass123","external_id":2}')
BOB_ID=$(echo "$BOB" | jq -r .id)
CAROL=$(req POST /api/v1/auth/register '' '{"username":"carol_seed","email":"carol_seed@example.com","password":"carolpass123"}')
CAROL_ID=$(echo "$CAROL" | jq -r .id)
echo "alice=$ALICE_ID bob=$BOB_ID carol=$CAROL_ID"

echo "== logins =="
ALICE_TOKEN=$(req POST /api/v1/auth/login '' '{"username":"alice_seed","password":"alicepass123"}' | jq -r .access_token)
ADMIN_TOKEN=$(req POST /api/v1/auth/login '' '{"username":"admin","password":"admin12345"}' | jq -r .access_token)
# negative case: bad password -> 401
req POST /api/v1/auth/login '' '{"username":"alice_seed","password":"wrong-password"}'

echo "== users =="
req GET /api/v1/me "$ALICE_TOKEN"                       # triggers external API + redis cache
req GET '/api/v1/users?limit=10&offset=0' "$ADMIN_TOKEN"
req GET "/api/v1/users/$BOB_ID" "$ALICE_TOKEN"
req PATCH "/api/v1/users/$CAROL_ID/role" "$ADMIN_TOKEN" '{"role":"admin"}'

echo "== tasks CRUD =="
TASK=$(req POST /api/v1/tasks "$ALICE_TOKEN" '{"title":"Fix login bug","description":"Users cannot login on mobile","priority":"high","due_date":"2026-06-10T15:00:00Z"}')
TASK_ID=$(echo "$TASK" | jq -r .id)
echo "task=$TASK_ID"
req GET '/api/v1/tasks?limit=50&offset=0' "$ALICE_TOKEN"
req GET "/api/v1/tasks/$TASK_ID" "$ALICE_TOKEN"
req PATCH "/api/v1/tasks/$TASK_ID" "$ALICE_TOKEN" '{"status":"in_progress","priority":"urgent"}'

echo "== assignment + history =="
req POST "/api/v1/tasks/$TASK_ID/assign" "$ALICE_TOKEN" "{\"user_id\":\"$BOB_ID\"}"
req GET "/api/v1/tasks/$TASK_ID/history" "$ALICE_TOKEN"
req DELETE "/api/v1/tasks/$TASK_ID/assign/$BOB_ID" "$ALICE_TOKEN"

echo "== deletes =="
req DELETE "/api/v1/tasks/$TASK_ID" "$ALICE_TOKEN"
req DELETE "/api/v1/users/$CAROL_ID" "$ADMIN_TOKEN"     # deactivate carol

echo "== seed done =="
