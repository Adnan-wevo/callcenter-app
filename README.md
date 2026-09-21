# callcenter-service

Standalone Go extraction of the `CallCenter` module from a Laravel
(nwidart/laravel-modules) monolith. Central-only, read-mostly reporting
surface over call-center/queue data, meant to run alongside FreePBX/pbx-worker
without the weight of the full Laravel app.

This repo was scaffolded **before** the Laravel reference code was available.
Almost every data shape, column name and endpoint contract below is a
documented **ASSUMPTION** — grep for `ASSUMPTION` and `TODO` across the repo
before trusting any of it against production data. See [Open questions](#open-questions--todo-once-reference-code-lands)
for the consolidated list.

## Layout

```
cmd/
  server/          the actual service
  mockpbxworker/    local-only stand-in for pbx-worker v2, used by docker-compose
  mocklaravel/      local-only stand-in for the Laravel callback endpoint
  hmacsign/         CLI helper: prints signed headers for manual curl testing
internal/
  config/           env var loading
  middleware/       inbound HMAC verification (+ in-memory nonce replay guard)
  security/hmacsig/ shared HMAC-SHA256 signing/verification (used by every direction: in, out to pbx-worker, out to Laravel)
  gateway/pbxworker/ HTTP client to pbx-worker reports API — interface + HMAC (v2, live) + JWT (v3, stub) implementations
  gateway/laravel/  outbound client for the one write-path (ad-hoc callback attempt)
  db/qstats/        direct MySQL read access to the separate "qstats" database
  handlers/         Gin HTTP handlers + router
  apires/           {"data": ...} / {"data": ..., "meta": ...} response envelope, matching the existing Laravel API Resource convention
migrations/qstats/  schema + seed data for the LOCAL TESTING mysql container only
reference/          drop the real Laravel/pbx-worker source here when you have it
```

## Running locally

Everything runs in Docker — no Go/Node toolchain needed on the host.

```bash
docker compose up --build
```

This starts:
- `mysql` — local MySQL seeded with an assumed `qstats` schema (port 3307 on host)
- `mock-pbx-worker` — fakes the pbx-worker v2 reports API, returns fixture data, enforces the real HMAC scheme (port 8081)
- `mock-laravel` — fakes the Laravel callback-attempt endpoint the service writes back to (port 8092 on host, 8082 inside the container — 8082 was already taken by an existing local stack on this machine)
- `callcenter-service` — the actual service (port 8080)

Smoke test once it's up:

```bash
curl http://localhost:8080/healthz

# inbound auth is disabled by default in docker-compose.yml (INCOMING_AUTH_DISABLED=true)
# for local convenience — flip it off and use cmd/hmacsign once you want to
# test the real auth path:
curl http://localhost:8080/api/v1/call-center/queues
curl http://localhost:8080/api/v1/call-center/agents
curl http://localhost:8080/api/v1/call-center/answered-calls
curl http://localhost:8080/api/v1/call-center/unanswered-calls
curl http://localhost:8080/api/v1/call-center/agent-events
curl 'http://localhost:8080/api/v1/call-center/calls/search?status=ANSWERED'
curl http://localhost:8080/api/v1/call-center/calls/call-1001
curl -X POST http://localhost:8080/api/v1/call-center/unanswered-calls/call-1002/callback \
  -H 'Content-Type: application/json' \
  -d '{"queue_id":"2","agent_id":"101","note":"testing"}'
```

To test with inbound HMAC verification turned on: set
`INCOMING_AUTH_DISABLED: "false"` in `docker-compose.yml` for
`callcenter-service`, then generate headers with `cmd/hmacsign`. It needs
the Go toolchain, so run it inside a throwaway container: 

```bash
docker run --rm -v "$PWD":/app -w /app golang:1.22-alpine \
  go run ./cmd/hmacsign -key local-dev-key -secret local-dev-secret \
  -method GET -uri "/api/v1/call-center/queues"
```

It prints the five headers and a ready-to-run curl command.

### VS Code

Open the folder in VS Code, use the Go extension against the module as
normal. Since the toolchain isn't installed on the host, either:
- install Go locally for editor tooling (gopls, go vet, etc.), or
- use the [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) pointed at the `golang:1.22` image / this repo's Dockerfile for a fully containerized dev environment.

`go build ./...` and `go test ./...` both work fine inside
`docker run --rm -v "$PWD":/app -w /app golang:1.22-alpine go build ./...`
without any host install.

## Design decisions already made (confirm, don't re-litigate blindly)

- **Queues/agents come from qstats directly**, not via pbx-worker's
  `queue-names`/`agent-names` actions, since the `queue_names`/`queue_agents`
  tables exist specifically for this. pbx-worker's equivalent actions are
  still fully implemented in the gateway client (unused) in case the
  reference code shows Laravel actually proxies through pbx-worker instead.
- **Answered/unanswered calls, agent events, call search/detail come from
  pbx-worker**, matching those action names 1:1.
- **pbx-worker client is behind an interface** (`gateway/pbxworker.ReportsClient`)
  with an HMAC (v2) and a JWT (v3, stub-only) implementation, selected by
  `PBX_WORKER_MODE`, so swapping to v3 later shouldn't touch handler code.
- **Handlers call the real repository/gateway layers**, not hardcoded stub
  JSON — "stub data" instead comes from the seeded local MySQL and the mock
  pbx-worker/Laravel servers, so the full request path (including HMAC
  signing) is actually exercised locally, not bypassed.
- **Inbound auth reuses the same HMAC scheme** as the rest of the system for
  consistency, but this is explicitly *your* call to finalize — see below.

## Open questions / TODO once reference code lands

Grep for `ASSUMPTION` and `TODO` for the full, precise list (each is
commented at its exact location). Summary:

1. **qstats schema** (`internal/db/qstats/models.go`, `migrations/qstats/001_init.sql`) — table/column names, types, and which table backs which report are all guessed.
2. **pbx-worker response shapes** (`internal/gateway/pbxworker/types.go`) — field names/types for all 7 actions are guessed; whether responses use pagination `meta` at all is guessed.
3. **HMAC wire details** (`internal/security/hmacsig/hmacsig.go`) — timestamp format (assumed Unix seconds), URI form in the canonical string (assumed path+query, no host), and body-hash-of-empty-body behavior are all guessed.
4. **Callback write-path** (`internal/gateway/laravel/callback_client.go`) — the endpoint path, payload shape, and response shape are all placeholders. This used to be an in-process function call into the Callback module; the payload it received there is the source of truth once you have it.
5. **Inbound auth scheme for this service** — currently implemented as HMAC (reusing the same pattern) with a `INCOMING_AUTH_DISABLED` escape hatch for local dev. You explicitly asked to decide this yourself once you've weighed alternatives (JWT, shared secret) — nothing here is final.
6. **call-detail recording enrichment** (`internal/handlers/calls.go`) — assumed pbx-worker returns `recording_url` directly; may need to join against `qstats.recordings` instead (repository method already exists: `qstats.Repository.GetRecordingByCallID`, just not wired up).
7. **Exact route list** — the 8 routes under `/api/v1/call-center` are a reasonable guess from the described report functions; confirm against Laravel's actual `routes/` file for this module once it's available.
