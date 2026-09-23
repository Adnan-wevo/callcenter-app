# SoftPhone ↔ CallCenter Flow Parity Audit

Comparison of the heal-crm "login-to-logout" SoftPhone/CallCenter flow against
the current `callcenter-app` implementation (Go backend in `internal/`,
Angular frontend in `web/src/app/`). Purpose: track exactly what's left to
reach flow/UX parity with heal-crm on this app's own stack (Angular + Go,
no Laravel/Livewire).

Audited: 2026-09-22.

## Stage-by-stage status

| Stage | Status | Evidence | Missing |
|---|---|---|---|
| **1. Login / seat / token / SIP register** | PARTIAL | `internal/softphone/softphone.go` `BootstrapFor()`, `internal/handlers/softphone.go` (`SoftphoneBootstrap`, `GET /secure/softphone/bootstrap`), JWT auth (`internal/middleware/jwt.go`, `internal/auth/auth.go`). Frontend registers via `web/src/app/engines/sip/webrtc/jssip-engine.ts`. | No seat/licence gate at all — no `AgentSeatRegistry`, no seat-limit 403, no "blocked" screen. No shift-hours/grace-minutes clock, no auto-logout-at-shift-end. Bootstrap is a plain REST call; no token caching/invalidation-on-extension-edit. |
| **2. Steady-state call arrival (dual pipeline)** | PARTIAL, by design divergence | `internal/handlers/calllog.go` (`CreateCallLog`/`MarkCallAnswered`/`FinalizeCallLog`/`DetectQueue`) is a direct port of `Softphone::createCallLog()` et al. `internal/calllog/repository.go` has the merge/de-dup logic. | No webhook ingestion pipeline (no `/webhook` route, no `PbxEventController` equivalent, no `EventEnvelope`/`call_events` table, no `active_queue_calls` projection). No realtime broadcast layer — `softphone.service.ts` polls `GET /queues`/`/agents`/`/live-calls` via `setInterval` instead of server push (own comment: *"Not sockets... without heal-crm's Echo/Reverb machinery"*). No agent call state machine — browser state is authoritative here, opposite of heal-crm's server-authoritative design. |
| **2a. In-call actions** | PARTIAL | `web/src/app/engines/sip/domain/sip-engine.ts`: hold/resume/mute/dtmf/blind-transfer implemented. | Attended transfer explicitly throws `"not implemented yet"` (`jssip-engine.ts:195-199`). No conference (no Web Audio API mixing). No video calling (`mediaConstraints: { video: false }` hardcoded). No in-call whisper/consult-call UI. |
| **2b. Supervisor actions** | **DONE** — most complete section, matches heal-crm's async pattern | `router.go:89-92` gates `/queue/redirect`, `/queue/pickup`, `/queue/spy` behind `PermSoftphoneSupervise`; `web/src/app/layout/softphone/supervisor-panel.ts`, `queue-tab.ts:88-106`. REST → PBX-worker Submit/Poll (`internal/gateway/pbxcontrol/client.go`), matching heal-crm's "submit → poll command_id to terminal state". | Nothing structural. |
| **2c. Reconnection / token refresh** | PARTIAL / UNCLEAR | JsSIP owns WS reconnect (free, library behavior). | No `_refreshToken()`-equivalent 401-triggered re-auth flow found in `softphone.service.ts`. Moot until stage 1's token-caching exists, but currently no documented recovery path for a lapsed JWT. |
| **2d. Directory / Phonebook, click-to-dial** | PARTIAL | `contacts-tab.ts` — internal directory merging pbx-worker live agent/peer status with click-to-dial. | No external Phonebook CRUD, no caller-ID enrichment lookup (`PhonebookController::lookup()` equivalent). Only the internal coworker directory half exists. |
| **3. Wrap-up / disposition / vertical-module bridge** | **NOT STARTED** | — | Zero `case_call`/`case_note`/`soap`/disposition feature anywhere. No Call Workspace Panel, no Caller/CaseCall/CaseNote model, no "vertical module" concept (no analogue needed since this is standalone — but something must replace it). No `wrapUpActive`-style flag, no `IncomingCallHandler`-equivalent bridge. Biggest architectural open question, not just a missing feature. |
| **4. Shift end** | **NOT STARTED** | — | No shift clock, no heartbeat endpoint, no `shift_ended` signal. Depends entirely on stage 1. |
| **5. Logout (4 variants)** | 1 of 4 present | `queue-tab.ts` `logout()` → `phone.queueLogout()` maps only to heal-crm's **5c "queue logout"** (break, not sign-out). | No global-logout listener (5a), no seat release (no seat exists), no SIP-account-switch flow (5d — likely N/A, schema allows only one extension per user by design), no forced-eviction-on-shift-exhausted path (5b). |
| **6. Known dead code (doc §6)** | **Avoided** | `router.go` never ported `/queue/transfer`, `/queue/whisper`, `/queue/conference` stub routes. Supervisor panel goes straight through the REST `/queue/spy` path — the one heal-crm's *working* code actually uses, not its dead escalate-to-whisper/barge UI pattern. | No risk currently observed. |

## UI/UX checklist

| UI element | Present? | Where |
|---|---|---|
| Queue Management panel (pickup/redirect) | Yes | `queue-tab.ts` / `.html` |
| Supervisor monitor/whisper/barge | Yes | `supervisor-panel.ts` |
| Contacts/internal directory + click-to-dial | Yes | `contacts-tab.ts` / `.html` |
| Call history tab | Yes | `history-tab.ts` / `.html` |
| Raw call logs tab | Yes | `logs-tab.ts` / `.html` |
| External Phonebook (saved numbers, caller-ID lookup) | No | — |
| Call Workspace / wrap-up disposition form | **No** | — |
| Attended-transfer UI | Partial (type exists, engine throws) | `jssip-engine.ts` |
| Conference merge UI | No | — |
| Video call UI | No | — |
| Seat-blocked / "not registered" screen | No | — |
| Shift-ending banner | No | — |

## Punch list (dependency order)

1. **Decide wrap-up/disposition scope.** No "vertical module" system exists in this standalone app, so decide: build a minimal built-in disposition form (caller notes + outcome) directly in the softphone panel, or explicitly scope it out as future CRM integration.
2. **Seat/licence gate** (stage 1) — blocks shift-end and any seat-limited pricing. Currently anyone with a SIP extension row registers, unlimited.
3. **Shift clock + heartbeat + auto-logout** (stage 4) — depends on #2.
4. **Realtime transport decision.** Current client-poll design works but won't scale past a handful of agents polling pbx-worker directly, and is why there's no server-side agent-state-machine broadcast. Decide: keep polling (fine for MVP, already documented as a deliberate simplification) or add a WS/SSE layer server-side feeding an `active_queue_calls`-style projection.
5. **Attended transfer** — quick win, type already exists, `jssip-engine.ts:199` just needs the implementation.
6. **Token refresh / 401 recovery** — needs a decision alongside #2 (seat/token caching); currently absent.
7. **External phonebook + caller-ID lookup** — lower priority, additive.
8. **Conference + video calling** — largest single UI/engine feature gap; sequence after attended transfer since conference reuses the same consult-call primitive.
9. **Logout variant parity (5a/5b/5d)** — once seat exists (#2), wire seat-release + SIP-unregister into the app's generic logout. 5d (SIP account switch) may not apply at all since the schema is one-extension-per-user by design.

## Source

Audit performed against a heal-crm architecture walkthrough doc (Laravel/Livewire + JsSIP + Go PBX Worker) pasted into the session on 2026-09-22, cross-checked file-by-file against this repo's `internal/` and `web/src/app/` trees plus `reference/wevetel-pbx-worker/old-backend-app-docs/`.
