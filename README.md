# callcenter-service

Standalone Go extraction of the `CallCenter` module from a Laravel
(nwidart/laravel-modules) monolith. Central-only, read-mostly reporting
surface over call-center/queue data, meant to run alongside FreePBX/pbx-worker
without the weight of the full Laravel app.

This repo was scaffolded **before** the Laravel reference code was available,
so its original data shapes, column names and endpoint contracts were
assumptions. The Laravel source has since been read and those assumptions
validated — the schema, pbx-worker gateway and HMAC layers are now corrected
against it. The HTTP/handler layer has **not** been redesigned yet.

**Read [docs/extraction-plan.md](docs/extraction-plan.md) first.** It has the
validation findings, what is fixed, what remains, and the open decisions that
block the remaining work.

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
  apires/           {"data": ...} / {"data": ..., "meta": ...} response envelope (NOTE: CallCenter has no *Resource.php classes — this envelope was invented, and whether to adopt a {status, data, meta} shape is open decision D1 in docs/extraction-plan.md)
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

# Report filters mirror PbxReportGateway::buildParams(): naive local
# "Y-m-d H:i:s" dates (NOT RFC3339), a seconds-of-day sub-range, and
# repeated queues/agents params.
curl 'http://localhost:8080/api/v1/call-center/answered-calls?date_from=2026-09-20%2000:00:00&date_to=2026-09-20%2023:59:59'
curl http://localhost:8080/api/v1/call-center/unanswered-calls
curl http://localhost:8080/api/v1/call-center/agent-events
curl 'http://localhost:8080/api/v1/call-center/calls/search?caller_id=%2B60123456789'

# :id is the call's uniqueid; the response is an ARRAY of timeline rows.
curl http://localhost:8080/api/v1/call-center/calls/1758358812.101
```

The callback route (`POST .../unanswered-calls/:id/callback`) is **not
functional yet** — its request shape does not match the real Laravel Action,
and the Laravel endpoint it needs does not exist. See §6 of
[docs/extraction-plan.md](docs/extraction-plan.md).

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

## Design decisions

- ~~**Queues/agents come from qstats directly**~~ — **disproven by the
  reference code.** There are no `queue_names`/`queue_agents` tables (the
  real ones are `qname`/`qagent`), and Laravel sources queue/agent names via
  pbx-worker's `queue-names`/`agent-names` actions, not SQL. The handler
  still reads qstats today; switching it is open decision D8.
- **Answered/unanswered calls, agent events, call search/detail come from
  pbx-worker**, matching those action names 1:1. Confirmed correct.
- **pbx-worker client is behind an interface** (`gateway/pbxworker.ReportsClient`)
  with an HMAC (v2) and a JWT (v3, stub-only) implementation, selected by
  `PBX_WORKER_MODE`. Confirmed correct — the real gateway is uniformly HMAC
  with no per-action auth switching, so there is no per-action variation to
  model.
- **Handlers call the real repository/gateway layers**, not hardcoded stub
  JSON — "stub data" instead comes from the seeded local MySQL and the mock
  pbx-worker/Laravel servers, so the full request path (including HMAC
  signing) is actually exercised locally, not bypassed.
- **Service-to-service auth is HMAC**, and the scheme is now confirmed
  byte-for-byte against `Modules/SoftPhone/app/Support/HmacSigner.php`.
  Note the asymmetry: **body hash is hex, signature is base64**. A regression
  test pins this (`internal/security/hmacsig/hmacsig_test.go`) — an earlier
  version signed in hex, which would have failed every request in both
  directions.
- **End-user auth does not exist yet.** HMAC is service-to-service only; a
  browser cannot use it. See §4.2 of the extraction plan.

## Status

**Resolved** against the Laravel reference code:

1. **qstats schema** — corrected. Real tables are `qname`/`qagent`/`qevent`
   (+ `queue_stats`, `queue_stats_mv`, `recordings`), with their real primary
   keys and columns. Note `recordings.uniqueid` is a **string** PK, and
   `qevent` is a lookup table, not an event log. The schema is **externally
   owned** (no migrations exist in the Laravel app) — columns beyond what the
   Eloquent models declare are still unverified.
2. **pbx-worker response shapes** — corrected for all 7 actions. There is
   **no pagination anywhere** on that API, and the envelope carries a
   `status` field the client must check.
3. **HMAC wire details** — confirmed and corrected (see Design decisions).
4. **Route list** — there was nothing to confirm against: CallCenter's
   `routes/api.php` is empty and the whole feature is Livewire. The current
   routes are an invented design pending §5 of the extraction plan.

**Still open** — see [docs/extraction-plan.md](docs/extraction-plan.md) for
the full list with owners and blockers:

- Report semantics parity: timezone, 90-day clamp, short-abandon threshold,
  SLA interval, wrap-up, and row-level security (§4.3). **This is where
  silent numeric divergence will come from.**
- End-user authentication (§4.2) and the response envelope (D1).
- The callback write-path — both sides (§6).
- `call-detail` recording enrichment: `qstats.Repository.GetRecordingByUniqueID`
  exists but is not wired up; pbx-worker's `call-detail` rows already carry
  `recording_file`, so confirm whether the join is needed at all.
