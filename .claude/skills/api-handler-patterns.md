---
name: api-handler-patterns
description: Gin handler conventions for WevetelBastion - binding, actor, response envelope, error mapping, route registration. Read before adding an API endpoint.
---

# API Handler Patterns

Handlers are thin: bind the request, take the actor from the authenticated context, call **exactly one** service method, map the result onto the shared envelope. See `internal/surfaces/<surface>/handler/sites.go` and `users.go`.

## Handler shape

```go
func NewSiteHandler(siteSvc *site.SiteService) *SiteHandler { ... }

// Store handles POST /api/v1/secure/bastion/sites/store.
func (h *SiteHandler) Store(c *gin.Context) {
    actorID := middleware.MustActorID(c)
    if actorID == uuid.Nil {   // 500 already written and chain aborted
        return
    }

    dto, ok := validation.Bind[site.CreateSiteDTO](c)
    if !ok {                   // 422 + field map already written and aborted
        return
    }

    created, err := h.siteSvc.Register(c.Request.Context(), actorID, dto)
    if err != nil {
        response.FromServiceError(c, err)

        return
    }

    response.Created(c, site.NewSiteResponse(created))
}
```

A collection reads its window through `validation.ListQuery` and answers `response.Paginated`:

```go
q, ok := validation.ListQuery(c)
if !ok { return }

sites, total, err := h.siteSvc.List(c.Request.Context(), actorID, q)
...
response.Paginated(c, site.NewSiteResponses(sites), validation.Meta(q, total))
```

## Rules

| Concern | Convention |
|---|---|
| Actor | `middleware.MustActorID(c)` returns `uuid.Nil` and has already written 500 + aborted; `middleware.ActorUser(c)` / `ActorPerms(c)` return `(v, ok)`. **Never** take the actor from body/query/path. A missing actor is a 500, not a 401: it means the route was mounted without JWT, which is a wiring bug and should be loud. |
| Bind | `validation.Bind[T](c)` for a body, `validation.BindQuery[T](c)`, `validation.ListQuery(c)` for a collection. Each writes 422 with a per-field map and **aborts** on failure, so the early `return` is the contract and the abort is the safety net. |
| Why one binder | There used to be four per-handler bind helpers and two validator instances, and only two of the four ran the DTO's `validate` tags. An unenforced tag reads exactly like an enforced one, so the gap was invisible. Binding is now the only way in (`internal/kernel/validation/validation.go`). |
| Path params | `pathUUID(c, "id")` in `internal/surfaces/<surface>/handler`; returns `(uuid, ok)` and has answered on failure. |
| Response mappers | Entities carry credential material and **never** reach the wire. Map through the application-layer DTO mapper: `site.NewSiteResponse` (no `FreePBXPass`/VPN `KeyData`), `user.NewUserResponse` (no password hash). |
| Success | `response.OK` / `Created` / `OKMessage` / `Paginated`. |
| Errors | `response.FromServiceError(c, err)` - `ErrForbidden`→403, `ErrNotFound`→404, `ErrInvalidInput`→400, `ErrConflict`→409, `*application.ValidationError` with fields→422, else 500. Compared with `errors.Is`, so it survives `%w` wrapping. Returns a **fixed generic message**, logs the real error: raw service errors wrap gorm/postgres text that discloses schema and row contents. |
| Empty lists | build with `make([]T, 0, n)` so JSON emits `[]` not `null`. |

## Envelope

`internal/kernel/response/response.go`:

```json
{ "status": "success", "data": {}, "meta": {}, "message": "" }
{ "status": "error", "message": "forbidden", "code": 403 }
{ "status": "error", "message": "validation failed", "code": 422, "errors": {"username": ["..."]} }
```

`data`, `meta` and `message` are omitted when empty. `errors` is populated for **422 only** and is keyed by the **wire** field name (the json tag, not the Go field, via the validator's `RegisterTagNameFunc`), because the client that must highlight the offending input knows the request it sent. It is the one failure body carrying detail, and every input to it is the caller's own request. Nothing derived from a database or filesystem error may be routed there.

`meta` is emitted only by `Paginated`, so a single-resource response never carries a meaningless page count.

Exception: `GET .../audit/export` returns raw `text/csv`, not the envelope.

## Route registration

The v2 surface is `/api/v1/{open|secure}/{module}/{function}/{action}`, modules `me`, `acl`, `bastion`, `core`. The split makes "is this route authenticated" answerable from the path alone, in a log line or a proxy rule, instead of by finding which group it was registered on. `/api/v1/open` holds login and `core/health` only; `/healthz` is unversioned. Refresh is **not** open: renewing a token is an authenticated operation.

Register in `internal/surfaces/routes/router.go`. Authentication is structural (JWT is on the group); authorization comes from the route's `routeGuards` row, mounted by `guard()`:

```go
// 1. add the row (path is the FULL template, as engine.Routes() reports it)
{http.MethodPost, "/api/v1/secure/bastion/sites/store", []permission.Name{permission.SitesStore}},

// 2. mount it
secured.POST("/bastion/sites/store", guard(http.MethodPost, sites+"/store"), siteH.Store)
```

`guard()` **panics** on a route with no row, so a route can never ship without an authority statement. `permission_table_test.go` fails on a row no route mounts. The five `/secure/me/*` routes mount `middleware.AllowAuthenticated()` instead, which states "authenticated and nothing more" without inventing a permission.

## Websockets

`/ws/sessions` and `/ws/requests` sit on their own group with JWT plus `wsActorBridge()`. WS handlers read context keys `user_id`/`perms` (`websocket.ContextKeyUserID`/`ContextKeyPerms`), which **differ** from the JWT middleware's `auth.user_id`/`auth.perms`. Neither package imports the other, so nothing catches a mismatch at compile time; without the bridge both streams fail closed with 401 for everyone.
