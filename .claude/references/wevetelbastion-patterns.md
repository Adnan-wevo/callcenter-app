# WevetelBastion Architecture Patterns

Reference for how this codebase is organised, for skills and agents operating in it.

## Project shape

- **One Go module** (`wevetelbastion`, Go 1.23.4) with three binaries under `cmd/` sharing one `internal/` core, plus an Angular 21 app under `web/` (Spartan UI v1 + Tailwind v4).
- **Clean Architecture + DDD.** Dependencies point inward only: domain ← application ← infrastructure/transport.
- Deps pinned to Dec-2024 versions (a newer `golang.org/x/crypto` forces Go 1.25 - do not bump casually).

## Layer layout (`internal/`)

| Layer | Path | Imports | Holds |
|---|---|---|---|
| domain | `internal/domain` | stdlib, uuid, datatypes only | `entity/`, `valueobject/`, `event/`, `repository/` interfaces |
| application | `internal/application/<svc>` | domain | services, DTOs, `port.go`, `errors.go`, `job/` |
| infrastructure | `internal/platform` | application, domain | `postgres/`, `kubernetes/`, `vpn/`, `wire/` |
| transport | `internal/api`, `internal/surfaces/smtpd`, `internal/kernel/config` | application, domain, config | Gin handlers/middleware/ws, the SMTP intake |

Only `cmd/*` may import `internal/wire`. The router takes a `Deps` struct of concrete services, never a `*wire.App`, to keep infrastructure out of the transport import graph.

## Key files

| File | Role |
|---|---|
| `cmd/api/main.go` | API composition root; checks JWT_SECRET/BASTION_HOST, starts hub + expiry watcher + HTTP server |
| `cmd/wevetel/main.go` | cobra generator + operator CLI |
| `internal/application/port.go` | outbound ports: `K8sPort`, `MailPort`, `Authorizer`, `Clock` |
| `internal/application/errors.go` | sentinels: `ErrNotFound`, `ErrForbidden`, `ErrInvalidInput`, `ErrConflict` |
| `internal/domain/permission/catalogue.go` | the hand-authored catalogue: 89 identifiers, `module.function[.qualifier]` |
| `internal/domain/permission/set.go` | `Set.Can`/`CanAny` - the ONE implementation of the decision; `is_super` short-circuits it |
| `internal/platform/postgres/authz_query.go` | the only place authority becomes SQL (roles ∪ direct grants, UNION) |
| `internal/surfaces/routes/router.go` | the entire HTTP surface + the `routeGuards` permission table |
| `internal/kernel/response/response.go` | the `{status,data,meta}` / `{status,message,code,errors}` envelope + sentinel→HTTP mapping |
| `internal/kernel/validation/` | the single bind path: `Bind[T]`, `BindQuery[T]`, `ListQuery`, `Meta` |
| `internal/platform/kubernetes/pod_manager.go` | `SpawnPod` - the session pod spec |
| `internal/platform/vpn/secret_manager.go` | per-site VPN Secret writer |
| `internal/platform/postgres/schema/main.go` | Atlas external-schema loader (register new entities here) |
| `atlas.hcl` + `scripts/atlas-url.sh` | migration config + DB URL builder |

## Core dependencies

Gin (HTTP) · GORM v2 + PostgreSQL 16 · Atlas (migrations) · google/wire (DI) · golang-jwt (HS256) · go-playground/validator v10 (binding) · BubbleTea + Lipgloss + Wish (TUI) · cobra (CLI) · client-go (Kubernetes) · gorilla/websocket · Angular 21 (web).

## The HTTP surface (v2 shape)

`/api/v1/{open|secure}/{module}/{function}/{action}`, modules `me`, `acl`, `bastion`, `core`. Whether a route carries a token is answerable from the path alone rather than by finding which group it was registered on, so a route added to the wrong group announces itself.

- `/api/v1/open` is login + `core/health` only; `/healthz` is unversioned. Refresh is authenticated and lives at `/secure/me/auth/refresh`.
- Authentication is **structural**: JWT is on the `/api/v1/secure` group, so adding a route there authenticates it by construction.
- Authorization is **tabular**: `routeGuards` in `internal/surfaces/routes/router.go` holds one row per route and `guard()` mounts `RequireAnyPermission` from it, panicking on a route with no row.
- Websockets are `/ws/sessions` and `/ws/requests`, JWT + `wsActorBridge()` (the bridge republishes `auth.user_id`/`auth.perms` as `user_id`/`perms`; without it both streams 401 everyone).
- Envelope: `{"status":"success","data":…,"meta":…}` / `{"status":"error","message":…,"code":…,"errors":…}`. `errors` is a per-field map on 422 only.
- Rate limiting is per client IP (20 rps / burst 40) in front of authentication, because an unauthenticated login flood has no user to key on.
- The Swagger console at `/api/docs` is UNAUTHENTICATED and gated by `DOCS_ENABLED` (default off).

## The central flow: a session is a pod

`POST /access/request` → `PUT .../approve` (sets `expires_at`) → `POST /sessions` → `PodManager.SpawnPod` (OpenVPN → fail-closed tun gate → TigerVNC Xvnc → websockify/noVNC:6080 → Chrome kiosk at the FreePBX URL), wait for PodReady (~120s) → `AccessURL http://<BASTION_HOST>:<NodePort>/vnc.html`. Reaper (`SessionService.WatchExpiry`, 60s) deletes the pod before ending the record. Read `session/service.go` + `pod_manager.go` + `docker/entrypoint.sh` together.

## Cross-cutting rules

- Authorization is permission-based and resolved **live from the DB** every request, with **no cache** (a cached Set is authority that outlives its revocation). The JWT carries only `user_id`, `username` and the registered claims, so there is nothing in it to trust. The `RoleLevel` ladder was dropped in phase 11.
- A route's permission is what ADMITS; services scope results independently (ownership). The self-approval ban is a rule, not a capability: no grant overrides it.
- Boot fails closed on an empty `permissions` table or no active super-admin (`postgres.CheckAuthzInvariant`), as well as on an empty `JWT_SECRET`, `BASTION_HOST` or `CORS_ORIGINS`.
- Fail closed everywhere; single-record repo lookups return `(nil, error)`, never `(nil, nil)`.
- Secrets live only in k8s Secrets / gitignored files; DTOs strip credential fields; audit redacts secret keys recursively.
- Migrations are Atlas (models are the source of truth); never `AutoMigrate`.
