# Keploy API-test onboarding — session transcript

A run-through of generating, verifying, mutation-proving, and CI-wiring Keploy V1 API tests for this repo (a Go / Gin monolith with MySQL + Redis + an external HTTP dep, JWT-protected routes).

- **App**: `monolith-multi-deps` (Keploy app id `69f43a9e05c7c7820b5fedc1`)
- **Framework**: Gin (Go 1.22)
- **Base URL**: `http://localhost:8080`
- **Storage mode**: `repo` (tests live in `keploy/api-tests/` under git)

---

## 1. Detection

Repo layout the agent inspected:

```
internal/handlers/router.go   # gin.Engine wiring
internal/handlers/{auth,user,task}_handler.go
internal/models/models.go     # Task / User / TaskAssignment / TaskHistory
internal/{service,repository,middleware,auth,cache,external,...}
cmd/server/main.go            # native Go binary, ./bin/server
docker-compose.yml            # MySQL + Redis only (no app service)
scripts/run-server.sh, scripts/restart-server.sh
```

Routes detected under `/api/v1/`:

| Resource | Routes |
|---|---|
| auth | `POST /auth/register`, `POST /auth/login` |
| users | `GET /me`, `GET /users`, `GET /users/:id`, admin: `PATCH /users/:id/role`, `DELETE /users/:id` |
| tasks | `POST /tasks`, `GET /tasks`, `GET /tasks/:id`, `PATCH /tasks/:id`, `DELETE /tasks/:id`, `GET /tasks/:id/history`, `POST /tasks/:id/assign`, `DELETE /tasks/:id/assign/:user_id` |

**Top-1 pick**: `tasks` — full CRUD, persistent state in MySQL, rich response body (`id`, `title`, `status`, `priority`, `owner_id`, `created_at`, `updated_at`). Auth routes used as a prelude.

---

## 2. Seed suite — `tasks_CRUD`

Authored `keploy/api-tests/tasks/test.yaml`:

- `register_user` — POST `/auth/register`, asserts `status_code_in: ["201","409"]` so re-runs are idempotent
- `login_user` — POST `/auth/login`, extracts `token` + `user_id`, asserts `$.user.role`, `$.user.is_active`
- `create_task` — POST `/tasks` (Bearer `{{token}}`), extracts `task_id`, asserts `$.title`, `$.status="pending"`, `$.priority="medium"`, `$.owner_id={{user_id}}`
- `get_task` — GET `/tasks/{{task_id}}`, asserts `$.id`, `$.status`
- `update_task` — PATCH `/tasks/{{task_id}}` (`{"status":"in_progress","priority":"high"}`), asserts state transitions
- `delete_task` — DELETE `/tasks/{{task_id}}`, asserts `$.status="deleted"`

All value/type assertions use `json_strict_equal` (DeepEqual) so they catch wire-shape regressions (number↔string, bool↔string, scalar↔array).

### First run — green

```
=== Test Results ===
Total: 1 | Passed: 1 | Failed: 0 | Flaky: 0 | Duration: 103ms

[PASS] tasks_CRUD (103ms)
    ✓ register_user   HTTP 201   49ms
    ✓ login_user      HTTP 200   41ms
    ✓ create_task     HTTP 201    4ms
    ✓ get_task        HTTP 200    1ms
    ✓ update_task     HTTP 200    4ms
    ✓ delete_task     HTTP 200    2ms
```

---

## 3. Mutation demo — `tasks_CRUD` caught 3/3 ✓

Three temporary edits applied + reverted to prove the assertions catch real bugs.

### Mutation 1 — VALUE change

`internal/handlers/task_handler.go:140`

```diff
- c.JSON(http.StatusOK, gin.H{"status": "deleted"})
+ c.JSON(http.StatusOK, gin.H{"status": "removed"})
```

Caught:
```
FAIL delete_task (HTTP 200, 2ms)
   assertion json_strict_equal: expected "deleted", got "\"removed\""
```

### Mutation 2 — JSON-tag rename

`internal/models/models.go:29`

```diff
- Status      string     `json:"status"`
+ Status      string     `json:"state"`
```

Caught (cascading across 3 steps — every step that read `$.status` from the Task struct):
```
FAIL create_task — $.status: expected "pending", got "<missing>"
FAIL get_task    — $.status: expected "pending", got "<missing>"
FAIL update_task — $.status: expected "in_progress", got "<missing>"
PASS delete_task (delete's status field is from gin.H, not the Task struct)
```

### Mutation 3 — Wire-type change

`internal/handlers/task_handler.go:140`

```diff
- c.JSON(http.StatusOK, gin.H{"status": "deleted"})
+ c.JSON(http.StatusOK, gin.H{"status": []string{"deleted"}})
```

Caught (this is the regression `json_equal` would have missed):
```
FAIL delete_task (HTTP 200, 2ms)
   assertion json_strict_equal: expected "deleted", got "[\"deleted\"]"
```

All three mutations reverted; `git diff --stat -- internal/` returned empty. Re-run of the baseline:

```
Total: 1 | Passed: 1 | Failed: 0 | Flaky: 0 | Duration: 0ms
```

---

## 4. Second suite — `users_CRUD`

Authored `keploy/api-tests/users/test.yaml` (9 steps). Self-register is locked to the `"user"` role, so the suite uses **two regular users** and asserts the admin-gating contract (403 from admin-only paths).

Flow:

1. `register_actor` — create user A (status_code_in [201,409])
2. `login_actor` — login A, extract `token` + `actor_id`
3. `register_subject` — create user B
4. `login_subject` — extract `subject_id` (stable target for the admin-route asserts)
5. `get_me` — `GET /me`, asserts `$.id == actor_id`, `$.role == "user"`, `$.is_active == true`
6. `list_users` — `GET /users`, asserts `json_contains $.users '"username":"keploy_users_actor"'`
7. `get_subject` — `GET /users/{subject_id}`
8. `patch_role_forbidden` — `PATCH /users/{subject_id}/role` as a non-admin → asserts `403`
9. `deactivate_forbidden` — `DELETE /users/{subject_id}` as a non-admin → asserts `403`

Result:

```
[PASS] users_CRUD (182ms)
    ✓ register_actor       HTTP 201
    ✓ login_actor          HTTP 200
    ✓ register_subject     HTTP 201
    ✓ login_subject        HTTP 200
    ✓ get_me               HTTP 200
    ✓ list_users           HTTP 200
    ✓ get_subject          HTTP 200
    ✓ patch_role_forbidden HTTP 403
    ✓ deactivate_forbidden HTTP 403
```

Combined run of both suites: `Total: 2 | Passed: 2 | Failed: 0`.

> Note: `auth` did not get its own suite — `register` + `login` are already exercised as preludes in both `tasks_CRUD` and `users_CRUD`.

### Schema gotcha

`json_path` requires an `expected` field even when only checking presence. Use `json_contains` (substring) or `json_strict_equal` (exact-typed) instead — those are what the validator accepts.

---

## 5. Sandbox record + replay

Captured outbound calls (MySQL, Redis, external HTTP) as mocks so CI can replay without booting real deps.

Command:

```bash
sudo -E keploy sandbox local record \
  --test-dir keploy/api-tests \
  --app-url http://localhost:8080 \
  -c './scripts/run-server.sh' \
  --progress-file /tmp/keploy-mcp-devloop-record.ndjson \
  --cloud-app-id 69f43a9e05c7c7820b5fedc1
```

Terminal NDJSON event:

```json
{"phase":"done","data":{"ok":true,"passed_suites":2,"passed_steps":15,
 "failed_suites":0,"failed_steps":0,"mocks_uploaded":2,
 "test_sets":["tasks_CRUD","users_CRUD"]}}
```

On-disk layout that landed:

```
keploy/
  api-tests/                 # V1 source (committed; what AI edits)
    tasks/test.yaml
    users/test.yaml
  tasks_CRUD/                # captured sandbox test-set (committed)
    config.yaml
    mocks.yaml
    tests/
  users_CRUD/                # captured sandbox test-set (committed)
    config.yaml
    mocks.yaml
    tests/
  reports/                   # gitignored
```

### Replay verification (mocks served, no live deps)

```bash
sudo -E keploy sandbox local replay \
  --test-dir keploy/api-tests \
  --app-url http://localhost:8080 \
  -c './scripts/run-server.sh' \
  --skip-mock-download \
  --cloud-app-id 69f43a9e05c7c7820b5fedc1
```

Result:

``` 
COMPLETE TESTRUN SUMMARY.
    Total tests: 17    Passed: 17    Failed: 0    Time taken: 10.21 s

    Test Suite Name      Total  Passed  Failed  Time
    "tasks_CRUD"          7      7      0      5.09 s
    "users_CRUD"          10     10     0      5.12 s

sandbox-local: ✓ tasks_CRUD (7 steps)
sandbox-local: ✓ users_CRUD (10 steps)
Time freezing verification completed successfully.
```

(The step count is higher than the V1 source because each chained step gets exploded into one classic test-case per HTTP call inside the captured test-set.)

---

## 6. CI scaffolding — native-Go workflow

Repo isn't dockerized (no app `Dockerfile`, `docker-compose.yml` only manages MySQL + Redis), so the docker-based scaffold the tool emits doesn't fit. Wrote a **native-Go workflow** instead — builds a separate faketime binary at `./bin/server.replay`, never touches the prod path `./bin/server`.

File: `.github/workflows/keploy.yml`

```yaml
name: Keploy sandbox replay
on:
  pull_request:
    branches: [main]
  workflow_dispatch:

jobs:
  keploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Install Keploy
        run: curl -sL https://keploy.io/ent/install.sh | sudo bash

      - name: Patch Go toolchain for faketime
        run: |
          set -eu
          HELPER=/usr/lib/keploy/go_freeze_time_amd64
          [ -f "$HELPER" ] || HELPER=/usr/local/lib/keploy/go_freeze_time_amd64
          if [ ! -f "$HELPER" ]; then
            echo "::error::go_freeze_time_amd64 not found after install"
            exit 1
          fi
          sudo "$HELPER"

      - name: Build replay binary (faketime build — never overwrites prod path)
        run: |
          mkdir -p bin
          go build -tags=faketime -o ./bin/server.replay ./cmd/server

      - name: Run Keploy sandbox replay
        env:
          REST_PORT: '8080'
          GRPC_PORT: '9090'
          LOG_LEVEL: info
          JWT_SECRET: super-secret-change-me
          JWT_EXPIRY: 30m
          MYSQL_HOST: 127.0.0.1
          MYSQL_PORT: '3306'
          MYSQL_USER: taskuser
          MYSQL_PASSWORD: taskpass
          MYSQL_DB: taskmanager
          REDIS_HOST: 127.0.0.1
          REDIS_PORT: '6379'
          REDIS_PASSWORD: ''
          REDIS_DB: '0'
          EXTERNAL_USER_API: https://jsonplaceholder.typicode.com
          KEPLOY_API_KEY: ${{ secrets.KEPLOY_API_KEY }}
        run: |
          sudo -E keploy sandbox local replay \
            --test-dir keploy/api-tests \
            --app-url http://localhost:8080 \
            -c "./bin/server.replay" \
            --skip-mock-download \
            --cloud-app-id 69f43a9e05c7c7820b5fedc1 \
            --ci \
            --freezeTime

      - name: Upload Keploy reports
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: keploy-results
          path: keploy/reports/
```

### Why these choices

- **Separate `server.replay` binary** — the Go faketime build reads time from `/tmp/keploy_time.txt`. If you overwrite `./bin/server` with the faketime binary, any future `keploy sandbox local record` against it captures wrong timestamps and corrupts the entire recording. The playbook rule is non-negotiable: prod binary path is sacred.
- **`--skip-mock-download`** — the mocks are committed in `keploy/tasks_CRUD/` and `keploy/users_CRUD/`, so CI doesn't need to round-trip to the canonical pool.
- **`--freezeTime`** — required because the recorded requests carry JWTs. Wall-clock between record and any PR's CI run is unbounded; without time-freezing, every authenticated step 401s.
- **No MySQL/Redis services in CI** — sandbox replay intercepts outbound calls at the eBPF syscall layer (incl. TCP connect), so the app boots without real deps being reachable. Mocks serve everything.
- **`JWT_SECRET` hardcoded in env** — must match the value used at record time. The recorded JWT is signed with this secret; replay just re-verifies the same signature against the mocked auth dep.

---

## 7. Action items before CI runs green

1. **Provision a CI-only API key** in the Keploy dashboard:
   - Open https://app.keploy.io/settings/api-keys
   - Click "Create API Key", name it `keploy-ci-<owner>-<repo>` (distinct so `keploy mcp-install` rotations don't blow it away), scopes `read,write`, TTL 365 days (or Never)
   - Plant it as a GitHub secret — **do not paste it into chat**:
     ```bash
     gh secret set KEPLOY_API_KEY -R <owner>/<repo>
     # paste kep_* at the prompt
     ```

2. **Commit + push** the new files:
   ```
   keploy/api-tests/tasks/test.yaml
   keploy/api-tests/users/test.yaml
   keploy/tasks_CRUD/{config.yaml,mocks.yaml,tests/}
   keploy/users_CRUD/{config.yaml,mocks.yaml,tests/}
   .github/workflows/keploy.yml
   ```
   The captured `keploy/<SuiteName>/` dirs MUST be committed — sandbox replay reads `mocks.yaml` from there to serve outbound calls. `keploy/reports/` is already gitignored.

---

## 8. Day-2 operations cheat sheet

| Want to… | Command |
|---|---|
| Add a new resource test | Edit `keploy/api-tests/<resource>/test.yaml`, then re-record mocks for that suite (`--suite-name`) |
| Run a suite locally against live deps | `keploy test-gen run --test-dir keploy/api-tests --suite <Name>_CRUD --base-url http://localhost:8080 --ci` |
| Re-record mocks after a contract change | `sudo -E keploy sandbox local record --test-dir keploy/api-tests --app-url http://localhost:8080 -c ./scripts/run-server.sh --cloud-app-id 69f43a9e05c7c7820b5fedc1` |
| Replay locally with mocks (mirrors CI) | `sudo -E keploy sandbox local replay --test-dir keploy/api-tests --app-url http://localhost:8080 -c ./bin/server.replay --skip-mock-download --freezeTime` |
| Prove a test catches a real bug | Apply a temporary mutation to the handler, run `keploy test-gen run …`, revert — see Section 3 for the mutation playbook |

### Gotchas

- **Never overwrite `./bin/server` with `-tags=faketime`.** Faketime binaries write wrong timestamps into captured mocks; re-recording with a faketime binary corrupts the entire recording with no recovery short of starting over. Always build the faketime binary to a sibling path (e.g. `./bin/server.replay`).
- **`JWT_SECRET` must match record-time.** If you change it in your local `.env` and re-record, also update the workflow env. Otherwise replays will 401 even with `--freezeTime`.
- **Re-runs against a dirty DB are fine.** The seed users `keploy_tasks_user`, `keploy_users_actor`, `keploy_users_subject` use `status_code_in: ["201","409"]` on register so the first run creates them, subsequent runs see 409 and move on. Tasks themselves have no unique-content constraint, so each run just creates fresh ones.

---

## 9. Final session state

| Artifact | Status |
|---|---|
| `keploy/api-tests/tasks/test.yaml` | ✓ green (6 steps) |
| `keploy/api-tests/users/test.yaml` | ✓ green (9 steps) |
| Mutation proof on `tasks_CRUD` | ✓ 3/3 caught (value, JSON-tag rename, wire-type) |
| `keploy/tasks_CRUD/` mocks | ✓ captured, uploaded |
| `keploy/users_CRUD/` mocks | ✓ captured, uploaded |
| Sandbox replay | ✓ 17/17 pass, time-freezing OK |
| `.github/workflows/keploy.yml` | ✓ written (native Go, faketime binary, no docker dependency) |
| `KEPLOY_API_KEY` GitHub secret | ⏳ dev to provision |
| Files committed + pushed | ⏳ dev to commit |
