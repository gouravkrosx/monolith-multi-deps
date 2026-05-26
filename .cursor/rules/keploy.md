---
description: Guidance for working with Keploy API tests in this repo. Use when the user asks about keploy/, sandbox tests, mocks, suites, or API testing.
---

# Keploy API tests in this repo

This repo uses Keploy for API contract testing. Two adjacent file trees under `keploy/`:

**V1 source-of-truth** (committed, AI-authored / dev-edited):
- `keploy/api-tests/keploy-test-gen.yaml` — app-level config (app_id, base_url, sandbox flag).
- `keploy/api-tests/<resource>/test.yaml` — chained-CRUD HTTP flow per resource. Drives both `keploy test-gen run` (live) and `keploy sandbox local record/replay` (mocked).

**Sandbox captures** (produced by `keploy sandbox local record`, one subdir per V1 suite — suite name == the top-level `name:` field in the source test.yaml):
- `keploy/<SuiteName>/tests/<step>.yaml` — per-step OSS test cases. Committed.
- `keploy/<SuiteName>/config.yaml` — OSS TestSet config + `mockRegistry.mock: <sha256>` (the mock-hash CI needs to pull captured mocks from Keploy cloud). Committed.
- `keploy/<SuiteName>/mappings.yaml` — captured mock mappings. Committed.
- `keploy/<SuiteName>/mocks.yaml` — full mock content. **Gitignored** — uploaded to Keploy cloud by hash, fetched on replay.

**So the mock-hash CI needs lives at `keploy/<SuiteName>/config.yaml`, NOT inside `keploy/api-tests/<resource>/`.** The `api-tests/` tree is dev-authored source; the `<SuiteName>/` siblings are CLI-generated captures.

Note: `keploy/test-set-*/` directories (if present) belong to the legacy OSS record/replay flow — unrelated to the V1 API-tests surface above.

## Driving Keploy via the agent

The Keploy MCP server should already be wired into this editor. If not: run `keploy mcp-install` from this repo root (after `keploy login`).

Then ask the agent:
- *"run the Keploy API tests"* — replays sandbox tests against captured mocks.
- *"add a Keploy test for <endpoint>"* — generates a new resource test.
- *"the Keploy test is failing"* — surfaces the report + self-heal options.
- *"rerecord stale mocks"* — refreshes captures after a contract change.
- *"wire Keploy into CI"* — scaffolds the GitHub Actions workflow.

The MCP's `devloop_resolve_storage` tool MUST be the first call for any of the above — it resolves repo-mode vs cloud-mode storage before routing to the right tools. The MCP server enforces this server-side: cloud-mode tools (replay_sandbox_test, create_test_suite, list_branches, etc.) refuse calls against repo-mode apps with a redirect to the V1 surface.

## Routing CI requests in this repo (binding)

When the user asks anything CI-shaped — *"wire CI"*, *"put my tests in CI"*, *"add a GitHub Action"*, *"can we run these sandbox tests in CI"*, etc. — go DIRECTLY to `devloop_scaffold_ci`. Do NOT try the cloud-mode `scaffold_pipeline_workflow` first. This repo is V1 repo-mode; tests live on disk, the cloud tool is the wrong surface and the server-side gate will refuse it.

`devloop_scaffold_ci` requires TWO routing args before it can emit anything: `ci_mode` AND `setup_mode`.

### ci_mode — which test surface to replay

- **`ci_mode="sandbox-replay"`** — replay the captured suites (auth_CRUD, tasks_CRUD, …) against recorded mocks. Fast (~5–10s/run), no DB/Redis in CI, no flaky deps. Mocks need periodic rerecord if contracts change.
- **`ci_mode="api-tests"`** — run the V1 source under `keploy/api-tests/` against a live app + real DB/Redis. Slower (~30–60s/run), validates real downstream behavior, no rerecord needed.

**When BOTH trees are present (api-tests/ AND any captured `<SuiteName>/` dir with a config.yaml), you MUST ask the dev which to use — non-negotiable.** Even if their request said the word "sandbox" or "api-tests". Surface the two options verbatim, mark `sandbox-replay` as `(Recommended)`, wait for an explicit answer, then call `devloop_scaffold_ci` with the chosen `ci_mode`. When only one tree exists, route silently to that mode.

### Auth + mock-source — how CI actually works

V1 design: `keploy/<SuiteName>/mocks.yaml` is **gitignored**. The heavy mock content lives in Keploy's canonical pool keyed by content hash; only the hash (`config.yaml.mockRegistry.mock`) is committed. So after `actions/checkout` in CI, there is no local `mocks.yaml` on disk.

**That makes CI's behavior straightforward:**

- `KEPLOY_API_KEY` env var → **required**. The CLI fetches each suite's mock blob from the cloud pool by hash, and uploads reports back to the dashboard. Both round-trips need the API key. There is no "no-auth offline" CI shape for V1 — without the key, the fetch fails and the run can't start.
- Default mock-fetch behavior (no flag needed) → cloud pool by hash, automatic.
- `--skip-mock-download` → **do NOT pass in CI**. It tells the CLI "use only local mocks.yaml," which doesn't exist in a fresh CI checkout. The flag is for local dev (right after recording, when the file's already on disk).

When the AI scaffolds a CI workflow for V1, do not offer the dev a "--useLocalMock without cloud auth" option. It would produce a workflow that can't run (no local mocks + no way to fetch them).

**Flag-name detail**: if you do ever want the skip-cloud-fetch behavior locally, the V1 surface (`keploy sandbox local replay`) uses `--skip-mock-download`. The legacy cloud-mode surface (`keploy test sandbox`, `keploy test integration-test`) uses `--useLocalMock`. Same intent, different flag name on each command — don't cross them.

### setup_mode — how the dev's app actually runs

- **`setup_mode="native"`** — app is a binary or shell script the workflow runs directly via `keploy sandbox local replay -c "./bin/server"`. No Docker for the app (deps may still be in compose). For Go + time-freezing, the workflow patches the local Go toolchain and rebuilds with `-tags=faketime` to a **separate** binary path (e.g. `./bin/server.replay`) — prod path is never touched. For non-Go + time-freezing, Keploy's CLI auto-injects the LD_PRELOAD shim; just pass `--freezeTime` and no extra setup is needed.
- **`setup_mode="docker"`** — app boots via `docker compose up <svc>` inside CI. For Go + time-freezing, the workflow generates a `Dockerfile.keploy` (faketime build) loaded via `docker-compose.keploy.yml`. For non-Go + time-freezing, the override sets `LD_PRELOAD` env on the app service. Requires `app_service` arg.

**Pick setup_mode from the dev's prior record_sandbox `app_command`:** if it was a binary path or shell script, native. If it was `docker compose up ...`, docker. If both are credible (Dockerfile + a native binary build both exist), YOU MUST ASK — don't guess. Cross-wiring (Docker scaffold for a native dev or vice versa) produces a workflow that can't run.

## Time-freezing is REPLAY-ONLY (non-negotiable)

`Dockerfile.keploy` (the `-tags=faketime` Go build) and `docker-compose.keploy.yml` (LD_PRELOAD + faketime build override) exist for ONE reason: making expired recorded JWTs validate at replay time. They MUST NEVER be used when recording.

Concretely:
- **Record (`keploy sandbox local record`)**: app runs from the dev's PROD artifact — prod `Dockerfile` via plain `docker compose up`, OR the prod native binary at the prod path (e.g. `./bin/server`). NEVER from a faketime build. The faketime binary will produce wrong timestamps in captured mocks and corrupt the recording.
- **Replay (`keploy sandbox local replay --freezeTime`)**: app runs from a SEPARATE faketime artifact — `Dockerfile.keploy` via `docker compose -f docker-compose.yml -f docker-compose.keploy.yml up`, OR a separate native binary at a sibling path (e.g. `./bin/server.replay`).

### Go native, strict rule

Go reads `time.Now()` via the vDSO and bypasses LD_PRELOAD entirely, so the runtime shim Keploy uses for Node/Python/Java/Ruby does NOT work for Go. The fix is build-time: `go build -tags=faketime`. But the faketime binary MUST go to a SEPARATE output path. Overwriting `./bin/server` (prod) with a faketime build silently corrupts every subsequent record run; the symptom surfaces on the NEXT capture, not the current replay. Two binaries:

- `./bin/server`           — prod build, used by record. NEVER rebuild with `-tags=faketime`.
- `./bin/server.replay`    — faketime build, used by replay. Gitignored as a build artifact.

### Non-Go native

Just pass `--freezeTime` to `keploy sandbox local replay`. Keploy injects the LD_PRELOAD shim into the app process automatically — no separate binary, no Dockerfile change.

### If a JWT-bearing replay fails with `401 token expired`

Surface to the dev with this EXACT ordering — do not reorder, do not invent options:

**A. Enable time-freezing (Recommended)** — re-run the replay with `--freezeTime`. For Go native: build a separate faketime binary at `<prod>.replay` and pass it via `-c`. For Go docker: use `Dockerfile.keploy` + `docker-compose.keploy.yml` override. For non-Go (any setup): just `--freezeTime` — Keploy auto-injects the LD_PRELOAD shim. This is THE fix; Keploy's time-freezing was designed for exactly this case. Existing recording untouched.

B. Re-record the suites — only useful when a contract has *actually* changed (mock-mismatch dominant). For pure JWT-expiry, re-recording is wasteful: 2 minutes later the new tokens are also stale. Fallback, not primary.

**Banned "fixes" — do not propose:**
- Templatize the JWT / extract-from-login / substitute-per-replay → not a Keploy concept; misroutes the dev.
- Bump `JWT_EXPIRY` env var to `8760h` / never-expire → masks the problem at the wrong layer.
- Set the runner's system clock / use libfaketime directly → bypasses Keploy's mock alignment.
- Edit the recorded `keploy/<SuiteName>/` files to "refresh" tokens → the recording is the truth; clock-rewind on replay is what validates it.
- **Don't blindly relay the Keploy CLI's `Time freezing could not be verified` error**. That message says "patch your Dockerfile" but it's wrong for native setups (your app may not even have a Dockerfile). If you see that error, route to option **A** above — pick the native or docker variant based on how the dev actually runs the app (`./bin/server` → native; `docker compose up` → docker). Never recommend patching the dev's prod Dockerfile in-place — use a sibling `Dockerfile.keploy` if docker, or a sibling `.replay` binary if native.
