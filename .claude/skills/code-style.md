---
name: code-style
description: Go coding conventions for WevetelBastion - the dependency rule, constructors, sentinel errors, fail-closed idioms. Read before writing or reviewing Go code here.
---

# Code Style

## The dependency rule (load-bearing)

Source dependencies point inward only.

- `internal/domain` imports only stdlib, `github.com/google/uuid`, `gorm.io/datatypes`.
- `internal/application` imports domain; declares the interfaces it needs (`port.go`, `repository/*`) and lets infrastructure implement them. Never imports infrastructure.
- `internal/platform` and `internal/api`/`internal/surfaces/smtpd` implement/consume inward.
- **Only `cmd/*` may import `internal/wire`.**

`go list` can prove these. Breaking one is a review-blocking error.

## Constructor pattern

```go
type AccessService struct {
    requests repository.AccessRequestRepository
    sites    repository.SiteRepository
    users    repository.UserRepository
    clock    application.Clock
}

func NewAccessService(requests repository.AccessRequestRepository, sites repository.SiteRepository, users repository.UserRepository, clock application.Clock) *AccessService {
    return &AccessService{requests: requests, sites: sites, users: users, clock: clock}
}
```

Services hold interfaces (repos + ports), not concrete infra types.

## Errors

Four sentinels in `internal/application/errors.go`: `ErrNotFound`, `ErrForbidden`, `ErrInvalidInput`, `ErrConflict`.

```go
user, err := s.users.FindByID(ctx, id)
if err != nil {
    return nil, fmt.Errorf("access: load requester %s: %w", id, err)
}
if !perms.Can(site.RequiredPermission) {
    return nil, fmt.Errorf("access: actor lacks %s: %w", site.RequiredPermission, application.ErrForbidden)
}
```

- Wrap with `fmt.Errorf("ctx: %w", err)`; callers compare with `errors.Is`.
- Transport maps sentinels to HTTP via `response.FromServiceError` (generic message out, real error logged).

## Idioms

| Rule | Detail |
|---|---|
| ctx first | every repo/service method: `(ctx context.Context, ...)` |
| Actor from context | `middleware.MustActorID(c)` (returns `uuid.Nil` having already written 500 and aborted) / `middleware.ActorUser(c)` - **never** from the request body/query |
| Authority | `permission.Name` constants from `internal/domain/permission/catalogue.go`, never a string literal and never a numeric level. The `RoleLevel` ladder was deleted in phase 11; a typed name is what makes a guard's signature checkable |
| Fail closed | when a check can't complete (repo error, missing actor), deny - no existence oracle |
| Repo lookups | single-record returns `(nil, error)` on no match, never `(nil, nil)` |
| Detached cleanup | compensating actions run on `context.WithoutCancel(ctx)` + own timeout |

## Entities

- UUID PK generated in `Base.BeforeCreate` (unset → `uuid.New()`). `AuditLog` has its own hook (no `Base`).
- Explicit `column:` tag where GORM's default mis-splits: `FreePBXIP` → `column:freepbx_ip` (not `free_pbx_ip`).
- Credential fields tagged `json:"-"` (`PasswordHash`, `FreePBXPass`, `KeyData`) so they never serialize.

## Prose: no em dashes

**No em dashes anywhere.** Not in code comments, not in docs, not in UI copy, not in commit messages. Use a comma, a colon, a full stop or parentheses. A repo-wide sweep enforces it, so a stray one is rewritten rather than kept.

Two directories the sweep skips, and you must too:

- **`db/migrations/`, for any migration that has already been applied.** Atlas records each file's checksum in `db/migrations/atlas.sum`; editing an applied file turns a punctuation change into a checksum mismatch that blocks every subsequent `migrate:run`. Rewriting a comment is not worth re-hashing history.
- **`internal/surfaces/docs/assets/`.** Vendored Swagger UI (`swagger-ui-bundle.js`, `swagger-ui.css`); it is third-party output, not our prose, and edits are lost on the next vendor refresh.

## Import order

stdlib → external → internal, separated by blank lines. Packages lowercase even under PascalCase-ish paths.

Last updated: auto-generated from codebase scan
