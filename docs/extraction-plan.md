# CallCenter extraction — findings & delivery plan

Status of extracting the `CallCenter` reporting module out of the `heal-crm`
Laravel monolith into this standalone Go service, plus the plan for the work
that remains.

Everything below was validated by reading the real Laravel source in
`heal-crm` (not inferred from the spec). Where a claim rests on specific
code, the file is named so it can be re-checked.

---

## Status at a glance

| Area | State |
|---|---|
| qstats schema | **Fixed** — structs now match the real Eloquent models |
| pbx-worker gateway | **Fixed** — all 7 actions corrected |
| HMAC signing | **Fixed** — was hex, must be base64 |
| Report semantics (timezone, clamps, thresholds, RLS) | **Not started** — 6 hidden rules, see §4.3 |
| Response envelope | **Open decision** — see D1 |
| End-user auth | **Not started** — service has no end-user auth at all, see §4.2 |
| Reporting endpoints | **Not started** — nothing to port, see §1.1 and §5 |
| Callback write-path | **Not started** — Laravel endpoint does not exist yet, see §6 |
| Angular frontend | **Not started** — pattern chosen, see §7 |

---

## 1. What the validation found

### 1.1 There is no existing REST API to port

`Modules/CallCenter/routes/api.php` is **empty** — a comment saying so is the
whole file. The entire CallCenter feature is server-rendered **Livewire**
components mounted from `routes/web.php` under `auth`, `verified` and
`EnsureCentralAccess`.

The one genuine JSON endpoint in the module is the public wallboard
(`DisplayController@data`, token-gated, no auth middleware), which is
unrelated to the reporting screens.

**Consequence:** this service's REST surface is a *new design*, not a port.
There is no existing contract to match, so the endpoint list in §5 is derived
from the Livewire components' `#[Computed]` methods — one endpoint per
distinct query the UI actually makes.

### 1.2 qstats is not the reporting data source

The Eloquent models for `qstats` exist, but the reporting services barely use
them. Every report goes through
`Modules/CallCenter/app/Services/Pbx/PbxReportGateway.php` — an HMAC-signed
HTTP client against pbx-worker's `/api/reports.php`.

Direct `qstats` SQL in the real app appears only in health checks
(`Modules/Monitoring/app/Services/HealthChecks/CallCenterHealthCheck.php`,
`MysqlHealthCheck.php`) doing row counts and staleness checks.

`qstats` also has **no migrations anywhere** in the Laravel app — it is an
externally-owned schema living on the PBX server (`config/tenancy.php` says
so explicitly). Column lists beyond what the models declare are still
unverified; get them from a live `SHOW CREATE TABLE`, not from this repo.

**Consequence:** see decision D8 — the scaffold currently sources
queues/agents from qstats directly, which does *not* match how Laravel does
it.

### 1.3 The HMAC signature encoding was wrong

`Modules/SoftPhone/app/Support/HmacSigner.php` signs with
`base64_encode(hash_hmac('sha256', $canonical, $secret, true))`. The Go
scaffold used `hex.EncodeToString`. Same bytes, different serialisation —
every request in both directions would have failed signature verification,
100% of the time.

Everything else in the scheme matched exactly: canonical string
(`METHOD\nURI\nTIMESTAMP\nNONCE\nBODY_HASH`), field order, separator, SHA-256,
hex body-hash, header names, Unix-seconds timestamp, raw-bytes key handling,
300s tolerance.

Note the asymmetry that caused the bug: **body hash is hex, signature is
base64**. PHP's `hash()` without the raw flag returns hex; `hash_hmac(...,
true)` returns raw bytes which are then base64-encoded. A regression test
pins this (`internal/security/hmacsig/hmacsig_test.go`).

### 1.4 The reports carry hidden semantics

Six rules live in the Laravel query layer that change the *numbers* and are
invisible from the endpoint shape. They are listed in §4.3 because they are
work, not just findings. Missing any one of them means the Go service returns
plausible-looking figures that quietly disagree with Laravel's.

---

## 2. Scope

Decided: **reporting reads + the callback write**. RealtimeMonitor and the
CRUD screens stay in Laravel.

| In scope | Out of scope (stays Livewire) |
|---|---|
| Dashboard | RealtimeMonitor (live AMI control: pause/unpause/logout/redirect) |
| Answered calls | QueueGroups CRUD (+ `RecordLock` edit-locking) |
| Unanswered calls (+ callback action) | ScheduledReports CRUD (cron + email job) |
| Call search (+ call detail) | Settings CRUD UI |
| Agent performance | UserFilters CRUD UI |
| Distribution | |

Two out-of-scope items still have a hard dependency on this service:

- **Settings** — `sla.sla_interval`, `threshold.short_abandon_threshold` and
  `threshold.wrap_up` are read by every report service. The *editing UI* can
  stay in Laravel, but this service must **read the same live values** (D3).
- **UserFilters** — `call_center_user_filters` is a **security boundary**
  enforced inside the query layer, not just in the UI. Wherever the query
  runs, the restriction must run with it (D4). This one cannot be deferred
  the way the CRUD screens can.

---

## 3. Round 1 — completed

Corrections applied to the scaffold, verified with `go build`/`go vet`/`go test`:

- **HMAC** (`internal/security/hmacsig/hmacsig.go`) — signature switched to
  base64; regression test added with an independently-computed known-answer
  vector so it cannot silently revert to hex.
- **qstats** (`internal/db/qstats/{models,repository}.go`) — real table names
  (`qname`, `qagent`, `qevent`, not `queue_names`/`queue_agents`/`queue_events`),
  real primary keys (`queue_stats_id`, `agent_id`, `event_id`, `queue_id`,
  and `recordings.uniqueid` as a **string** PK), real column names (`queue`,
  `agent`, `event`, `filename`). Invented fields removed. `qevent` re-modelled
  as the lookup table it actually is, not an event log. `queue_stats_mv`
  stripped back to its three confirmed datetime columns.
- **pbx-worker gateway** (`internal/gateway/pbxworker/*`) — all 7 actions:
  `queue-names`/`agent-names` return `[]string`; list actions take
  `date_from`/`date_to` as naive `Y-m-d H:i:s` plus `seconds_start`/`seconds_end`
  and `queues[N]`/`agents[N]`; `call-search` gained `unique_id` and the
  `duration_operator`+`duration_seconds` pair; `call-detail` keys on
  `call_uniqueid` and returns an **array** of timeline rows. Pagination/`meta`
  removed (this API has none). Added the `status != "ok"` envelope check the
  real gateway performs.
- **Plumbing** — `interface.go`, `jwt_client.go`, `handlers/{handlers,calls,queues}.go`
  and the `mockpbxworker` fixtures updated to the corrected shapes.

---

## 4. Round 2 — backend foundation

Prerequisites for every endpoint. Do these before §5.

### 4.1 Response envelope

Current: `internal/apires` emits `{"data": ...}` / `{"data": ..., "meta": ...}`.

The Angular client pattern we are adopting (§7) expects a `status`
discriminator so it can branch on the body alone:

```
{ "status": "success", "data": <T>, "meta"?: {...}, "message"?: "..." }
{ "status": "error",   "message": "...", "code": 422, "errors"?: {"field": ["..."]} }
```

See D1. If adopted, `apires` gains `status` and an `errors` map for 422s.

### 4.2 End-user authentication

**This service currently has no end-user auth.** The only auth that exists is
HMAC, which is service-to-service (Laravel↔Go, Go↔pbx-worker). A browser
cannot sign requests that way, and should not hold a shared secret.

The proven pattern in the sibling `wevetel-bastion` service:

1. Login endpoint issues a **JWT carrying identity only** — `user_id`, `exp`,
   `iat`. No roles, no permissions in the token.
2. A separate authenticated endpoint returns the caller's resolved
   **authority** (permission strings, roles, super flag).
3. The client holds the JWT; the server re-validates on every request and is
   always the authority.

Why identity-only: a token carrying permissions keeps granting them after a
grant is revoked, until it expires.

Maps cleanly onto the existing Laravel permission vocabulary
(`call-center.dashboard.index`, `call-center.answered-calls.export`,
`call-center.unanswered-calls.callback`, …). The authority response is also
the natural place to carry the **UserFilters** restrictions (§4.3 rule 6).

Open: who issues and signs the JWT — Laravel or this service (D2).

### 4.3 Report semantics parity

Six rules that must be replicated exactly, or the numbers diverge:

| # | Rule | Source | Note |
|---|---|---|---|
| 1 | **Timezone is `Asia/Kuala_Lumpur`**, not UTC | `config/app.php`, `.env` | Filters are built as naive local strings (`"{date} 00:00:00"` / `"{date} 23:59:59"`) and sent to pbx-worker as `Y-m-d H:i:s` with **no offset**. Go defaulting to UTC shifts every day/month boundary by 8h and corrupts day/hour/day-of-week groupings. |
| 2 | **90-day range clamp** | `HasReportFilters::enforceDateRangeLimit()` | Silently pulls `startDate` forward, does **not** reject. Match the behaviour, not just the limit. |
| 3 | **Short-abandon suppression** | `threshold.short_abandon_threshold`, default `5` | Unanswered calls with `hold_time` below it are **dropped entirely** from every summary, grouping, detail and export. |
| 4 | **SLA interval** | `sla.sla_interval`, default `20` | Drives SLA% in the distribution/dashboard figures. |
| 5 | **Wrap-up time** | `threshold.wrap_up`, default `0` | Added to occupancy in `AgentReportService`. |
| 6 | **Row-level security** | `CallCenterUserFilter` via `HasReportFilters::effectiveQueues/effectiveAgents` | If a user has filter rows, their queue/agent selection is intersected with the allowed set; **an empty intersection falls back to the full allowed set, not to empty**. Security boundary — must run server-side wherever the query runs. |

Rules 3–5 read `call_center_settings` live (memoised per-request in PHP,
cleared on save). Source of truth for this service: D3.

### 4.4 Pagination

pbx-worker returns **no pagination on any action** — it hands back the whole
result set. Laravel paginates in-app (`LengthAwarePaginator`) for the detail
tabs only.

So: this service must paginate in-app too. Summary/grouped responses are
**keyed maps** (by queue/agent/date/hour), not lists, and are not paginated —
only the detail tabs are. The envelope's `meta` applies to the latter only.

---

## 5. Round 3 — reporting endpoints

One endpoint per distinct query the Livewire UI makes. Paths are proposals.

| Feature | Livewire source | Proposed endpoint | Shape |
|---|---|---|---|
| Dashboard | `DashboardSummaryService::summary()` | `GET /dashboard/summary` | object (answered/unanswered/distribution blocks) |
| Answered | `AnsweredCallReportService::summary()` | `GET /answered-calls/summary` | object |
| Answered | `::byGrouping($f, $grouping)` | `GET /answered-calls/grouped` | keyed map |
| Answered | `::detail()` | `GET /answered-calls` | paginated list |
| Answered | `AnsweredCallsExport` | `GET /answered-calls/export` | CSV (D6) |
| Unanswered | `UnansweredCallReportService::summary()` | `GET /unanswered-calls/summary` | object |
| Unanswered | `::byGrouping()` | `GET /unanswered-calls/grouped` | keyed map |
| Unanswered | `::detail()` | `GET /unanswered-calls` | paginated list |
| Unanswered | `UnansweredCallsExport` | `GET /unanswered-calls/export` | CSV (D6) |
| Unanswered | `RecordAdHocCallbackAttempt` | `POST /unanswered-calls/callback` | §6 |
| Call search | `CallSearchService::search()` | `GET /calls/search` | paginated list |
| Call search | `::callDetail($uniqueId)` | `GET /calls/{uniqueid}` | **array** of timeline rows |
| Agent perf. | `AgentReportService::summary()` | `GET /agent-performance/summary` | keyed map |
| Agent perf. | `::pauseBreakdown()` | `GET /agent-performance/pauses` | keyed map |
| Agent perf. | `::durationStats()` | `GET /agent-performance/durations` | keyed map |
| Distribution | `DistributionReportService::summary()` | `GET /distribution/summary` | keyed map |
| Distribution | `::grouped()` | `GET /distribution/grouped` | keyed map |
| Distribution | `::detail()` | `GET /distribution` | paginated list |
| Lookups | `PbxReportGateway::queueNames()` | `GET /queues` | `[]string` (D8) |
| Lookups | `PbxReportGateway::agentNames()` | `GET /agents` | `[]string` (D8) |

Shared filter params on every report endpoint, mirroring
`HasReportFilters`: `date_from`, `date_to`, `seconds_start`, `seconds_end`,
`queues[]`, `agents[]`, `queue_group_id`, plus `grouping` on the grouped
endpoints and `page`/`per_page`/`search` on the paginated ones.

**Two endpoints carry real algorithmic weight** and are not thin passthroughs:

- `AgentReportService::summary()` and `::pauseBreakdown()` replay the agent
  event stream **in order**, pairing open/close events (login/logoff,
  pause/unpause, connect/complete) per agent, with wrap-up handling and
  open-session edge cases. This is stateful sequential processing, not a SQL
  aggregate. Port it carefully and test it against Laravel's output on the
  same window.
- `DistributionReportService::dayHourHeatmap()` exists but is **not called**
  from the Livewire component. Confirm whether the Blade view calls it
  directly before deciding it needs an endpoint.

---

## 6. Round 4 — callback write-path

Today `UnansweredCalls/Index.php::callback(?string $phone)` does two things
on one click:

1. `app(RecordAdHocCallbackAttempt::class)->handle($phone, Auth::id())` —
   creates a `CallbackAttempt` row (`candidate_id: null`, `phone_number`,
   `agent_id`, `dialed_at: now()`, `outcome: Pending`). Result is discarded.
2. `$this->dispatch('softphone-callback', phone: OutboundDialFormatter::apply($phone))`
   — a **browser event** that drives the embedded softphone widget to dial.

A REST endpoint can replicate (1). It **cannot** replicate (2) — see D7.

### Laravel side (does not exist yet)

No `api/internal/` convention exists anywhere in the app — the scaffold's
`LARAVEL_CALLBACK_PATH` was invented. The convention to mirror is
`Modules/SoftPhone`:

- `POST /api/v1/softphone/pbx/event` → controller, guarded by
  `VerifyPbxWebhookHmac` middleware (the inbound mirror of `HmacSigner`:
  same 5 headers, same canonical string, base64 signature, 300s skew,
  `^[a-f0-9]{32}$` nonce, 600s replay TTL).

So, to build:

1. `Modules/Callback/routes/api.php` → `POST /v1/callback/attempts`
2. `RouteServiceProvider::mapApiRoutes()` (the Callback module only maps web
   routes today)
3. `VerifyCallCenterServiceHmac` middleware — copy of `VerifyPbxWebhookHmac`
   pointed at its own config/credentials
4. FormRequest: `phone_number` (required, string, max 64), `agent_id`
   (required, uuid, exists:users,id)
5. Controller calling the existing Action

Request `{"phone_number": "...", "agent_id": "<uuid>"}` →
`201 {"ok": true, "data": {...}}`.

### Go side

`internal/gateway/laravel/callback_client.go` is wrong in both directions:
`CallbackRequest` has **no `phone_number` field at all** (the one field that
matters), and carries `UnansweredCallID`/`QueueID`/`Note` which the Action
does not accept.

Also note the authorisation gap: the Livewire call site checks
`call-center.unanswered-calls.callback` before acting. HMAC only proves *the
request came from this service* — not that the acting agent is allowed to. So
either this service checks the caller's permission before calling Laravel, or
Laravel validates the `agent_id`'s permission server-side. Pick one and state
it; do not leave it to neither.

### Tenancy caveat

`CallbackAttempt` uses `BelongsToTenant` and tenant identification appears to
be host-based. This service must therefore call the **correct tenant
subdomain**, not one shared host. Confirm before implementing.

---

## 7. Round 5 — Angular frontend

No frontend exists. `heal-crm` is Livewire + Vite + Tailwind; there is no
Angular anywhere in it. The patterns below are taken from the sibling
`wevetel-bastion` service, which pairs a Go backend with an Angular SPA and
is the closest working precedent.

> The **visual and interaction** half of this — shell layout, logo placement,
> wording register, how drawers open, the search/filter toolbar, theme tokens
> — is specified separately in [ui-design-spec.md](ui-design-spec.md).

### Patterns to adopt

- **Angular 21 standalone components**, no NgModules. Signals for state,
  `ChangeDetectionStrategy.OnPush` throughout.
- **Folder layout**: `core/` (api, auth, authz, interceptors, models),
  `features/` (one folder per screen, each with its own `*.routes.ts`),
  `layout/` (sidebar + per-surface nav config), `shared/` (data-table,
  drawers, confirm dialog, page header, form helpers).
- **One API client** (`ApiService`) that unwraps the envelope, so no feature
  service repeats `.pipe(map(r => r.data))`. List params (`page`, `per_page`,
  `search`, `sort`, `order`, `filter[col]`) serialised in exactly one place.
- **Interceptors**: one attaches the bearer token, one handles 401 (logout),
  403 (re-resolve authority once, throttled), 422 (pass through as typed
  field errors for the form), 5xx-on-GET (error page).
- **AuthZ in three layers**: route guard (`requirePermission(...)`,
  OR-combined) + a resolver that waits for authority before first render;
  structural directive (`*hasPermission`) to hide controls; server enforcement
  underneath. The permission store is **fail-closed** — denies unless loaded.
  Permission strings are a generated union type, so a typo is a compile error.
- **Lazy routes** (`loadComponent` / `loadChildren`) per feature.
- **Non-blocking bootstrap** — authority loads via an `effect()`, not a
  blocking `APP_INITIALIZER`, so the shell can render loading/error/retry
  instead of freezing.

### Screens

One per §2 in-scope feature. Answered/Unanswered/Distribution each have a
summary view, a grouped view and a paginated detail table; Call search has a
list plus a detail drawer; Agent performance has three panels.

The shared filter bar (date range, seconds-of-day, queues, agents, queue
group, grouping) is one component reused by every screen — it is the
`HasReportFilters` trait's equivalent.

---

## 8. Round 6 — cutover

1. Run both surfaces against the same window and diff the figures — dashboard
   totals, SLA%, abandon counts, agent occupancy. The §4.3 rules are where
   divergence will show.
2. Only once they agree: point users at the new UI and retire the
   corresponding Livewire routes.
3. Out-of-scope screens (RealtimeMonitor, CRUD) stay where they are; do not
   orphan their nav entries.

---

## 9. Open decisions

Each blocks the round named. None should be guessed.

| # | Decision | Blocks |
|---|---|---|
| D1 | Adopt the `{status, data, meta}` envelope in `apires`? | §4.1, all of §5 |
| D2 | Who issues/signs the end-user JWT — Laravel or this service? Where does the authority payload come from? | §4.2 |
| D3 | How does this service read `call_center_settings` — shared DB access, or a Laravel internal endpoint? | §4.3 rules 3–5 |
| D4 | Same question for `call_center_user_filters` (security boundary — see §2) | §4.3 rule 6 |
| D5 | Callback endpoint path: `/api/v1/callback/attempts` (SoftPhone convention) vs the scaffold's invented `/api/internal/...` | §6 |
| D6 | CSV export: generated here, or left in Laravel? | §5 export rows |
| D7 | The softphone dial half of the callback action — how does the Angular app trigger the dial, given REST cannot dispatch the Livewire browser event? | §6, §7 |
| D8 | Queues/agents from the pbx-worker gateway (matches Laravel) or from qstats directly (current scaffold)? | §5 lookups |

---

## 10. Risks

- **Silent numeric divergence** — the §4.3 rules produce figures that look
  right and are wrong. This is the single biggest risk in the project;
  §8 step 1 exists specifically to catch it.
- **qstats schema is externally owned** — it can change without a migration
  in any repo here. Anything beyond the columns the Eloquent models declare
  is unverified.
- **Agent-performance event pairing** — stateful, order-dependent, and easy
  to get subtly wrong (open sessions at window edges, unmatched pauses).
- **RealtimeMonitor is push-based** (Laravel Echo broadcast, not polling). If
  it is ever pulled into scope, a stateless REST poll is a UX regression
  unless this service also gains a broadcast channel.
- **Tenancy** — the callback write path is tenant-scoped by host. Getting
  this wrong writes rows into the wrong tenant.

---

## Evidence index

Laravel (`heal-crm`):

- `Modules/CallCenter/routes/api.php` — empty; no REST API exists
- `Modules/CallCenter/routes/web.php` — the Livewire page mounts
- `Modules/CallCenter/app/Services/Pbx/PbxReportGateway.php` — the real report data source
- `Modules/CallCenter/app/Services/Reports/*.php` — the six report services
- `Modules/CallCenter/app/Livewire/Concerns/HasReportFilters.php` — shared filters, 90-day clamp, row-level security
- `Modules/CallCenter/app/Livewire/*/Index.php` — the screens and their `#[Computed]` queries
- `Modules/CallCenter/app/Models/*.php` — qstats table/PK/column truth
- `Modules/CallCenter/app/Support/DTO/ReportFilterData.php` — filter DTO
- `Modules/SoftPhone/app/Support/HmacSigner.php` — outbound signing
- `Modules/SoftPhone/app/Http/Middleware/VerifyPbxWebhookHmac.php` — inbound verification; the pattern §6 mirrors
- `Modules/Callback/app/Actions/RecordAdHocCallbackAttempt.php` — the write this service must trigger
- `config/tenancy.php`, `config/app.php`, `.env` — qstats ownership, app timezone

Precedent (`wevetel-bastion`):

- `web/src/app/core/auth/auth.service.ts` — identity-only JWT
- `web/src/app/core/authz/permission.store.ts` — fail-closed authority
- `web/src/app/core/authz/permission.guard.ts` — route guards + resolver
- `web/src/app/core/api/api.service.ts`, `api.types.ts` — envelope and list params
- `web/src/app/core/interceptors/*.ts` — token attach, error handling
- `web/src/app/app.ts` — shell, surface switching, non-blocking bootstrap
