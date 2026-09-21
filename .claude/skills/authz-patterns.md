---
name: authz-patterns
description: The permission-based authorization model of WevetelBastion - the catalogue, the routeGuards table, live DB resolution, and service-side scoping. Read before touching auth-gated routes or approval logic.
---

# Authorization Patterns

Authorization is **permission-based** (Spatie-shaped) and **resolved live from the database on every request**. The JWT carries only `user_id`, `username` and the registered claims, so there is nothing in it to trust.

**The RoleLevel ladder is gone** (phase 11, migration `db/migrations/20260720084501_drop_level_rbac.sql`). There is no `roles.level`, no `roles.permissions`, no `users.role_id`, no `sites.min_role_level`, no `RoleLevel*` constants and no route "floors". Roles are unordered peers; nothing outranks anything.

## The vocabulary

One hand-authored catalogue: `internal/domain/permission/catalogue.go`, 89 identifiers shaped `module.function[.qualifier]`. `All()`/`Lookup()` read it; the slice itself is unexported.

The v3 convention, stated in that file's `catalogue` doc comment:

- **Every resource carries the same seven actions** - index, create, store, show, edit, update, destroy - plus its genuine custom ones (approve, deny, cancel, spawn, terminate, stream, export, sync, catalogue, unlock, reveal-credentials, vpn-write). Uniformity is the point: a reader can tell what exists for a resource without reading the routes.
- **Scopes are on reads only.** `.own`/`.any` appear on `index` and `show`, and on the custom actions where the scope is a real authority difference (spawn, terminate, stream, approve, deny, cancel). Writes are unqualified, because whether a particular row may be written is a service rule about ownership. A scope on `store`/`update`/`destroy` would put the same decision in two places and let them disagree.
- **`create` and `edit` have no route.** Both gate a UI affordance (the "new" button and the row's edit control), so offering a form is delegable separately from the write it leads to. `users.create` creates nobody: the write is `users.store`, checked independently. `users.edit` opens nothing on its own either: the edit drawer loads the record via `show` and the write is `users.update`, checked when the form is submitted.
- **The honesty rule.** Actions meaningless on a resource (audit is append-only, a session is a pod) are still declared, but their `Title` ends `(inert - no endpoint)` and the `Description` says granting it confers nothing. An operator ticking a box that does nothing while believing otherwise is a security problem.
- **Assign is split from revoke** (`roles.assign-permission` / `roles.revoke-permission`, `users.assign-role` / `users.revoke-role`), so handing authority out and taking it back are separately delegable.
- **No wildcards.** `*` cannot match the identifier regex, so there is nothing to expand (`internal/domain/permission/doc.go`, rule 4). A `sites.*` grant would silently confer every permission later added to that module and would form a second uncatalogued super-admin path.

Qualifiers are a **closed** vocabulary and are **not ordered**: `sessions.index.any` does not imply `sessions.index.own`, and `sites.access.restricted` does not imply `.sensitive` or `.standard` (`internal/domain/permission/sitetier.go`). Ask for both/all via `CanAny`, or grant all three.

## Resolution

`authz.AuthzService` (`internal/application/authz/service.go`) builds a `permission.Set` = (roles via `user_roles` → `role_permissions`) ∪ (unexpired `user_permissions`) in one indexed UNION query, `internal/platform/postgres/authz_query.go`. That file is the only place authority becomes SQL; its four filters (soft-deleted role, soft-deleted permission on **both** legs, the expiry predicate, UNION not UNION ALL) are what make revocation actually revoke.

**There is deliberately no cache** (`internal/application/authz/doc.go`). A cached Set is authority that outlives its own revocation.

- **The zero Set denies everything**, so a guard whose resolution failed fails closed by construction rather than by remembering to check an error (`internal/domain/permission/set.go`).
- **Super-admin is `roles.is_super`**, short-circuiting `Set.Can()`. A super-admin holds **zero** permission rows, so `Set.List()` is empty for them: never infer authority from the list's length, call `IsSuper()`.
- `ActorPerms(c)` returning false means **"cannot answer"**, not "denied" (`internal/surfaces/middleware/authn.go`). An absent Set is a route wired without JWT, a fault; an empty Set is a real decision.

## Route gating

Every guarded route's permission lives in the `routeGuards` table in `internal/surfaces/routes/router.go`, one row per route, and is **mounted from** rather than described. `guard(method, path)` looks the row up and mounts `middleware.RequireAnyPermission(row.perms...)`; it **panics** on a route with no row, so a route with no declared authority cannot ship. `internal/surfaces/routes/permission_table_test.go` reads the same rows and fails on a row no route mounts.

```go
// routeGuards
{http.MethodGet, "/api/v1/secure/bastion/sites/index", []permission.Name{permission.SitesIndex}},

// registerBastion
secured.GET("/bastion/sites/index", guard(http.MethodGet, sites+"/index"), siteH.Index)
```

The path in a row is the **full** route template including the group prefix, exactly as `engine.Routes()` reports it, because that is the key the table test joins on.

Guards live in `internal/surfaces/middleware/authz.go`: `RequirePermission`, `RequireAnyPermission`, `RequireSitePermission`, `AllowAuthenticated`. `guard()` always mounts `RequireAnyPermission` (holding any one named permission admits). `AllowAuthenticated` is mounted directly on the five `/secure/me/*` routes, which need a live account and nothing more. `RequireSitePermission` is defined but currently mounted on **no** route: the site-tier check lives in `AccessService.RequestAccess`.

The permission named on a route is what **admits**, not everything the handling touches. Services scope results independently.

## Service-side scoping

One rule is **not a capability**, and no grant overrides it:

- **The self-approval ban.** An actor may never review their own request, super-admin included (`permission.MayApprove`, `internal/domain/permission/review.go`).

**Approver separation of duty was removed on 2026-07-19.** Peer approval is permitted by design: two holders of `access-requests.approve.any` may approve each other. This is a deliberate weakening, recorded in `review.go` so a future reader does not "restore" it as a bug fix. Control over who may approve whom now rests entirely on who is granted `access-requests.approve.any`. **The department concept was removed on 2026-07-22:** the `.department` scopes are gone and their capabilities collapse onto `.own` and `.any`.

`MayApprove`/`MayDeny` are the single statement of the rule; the listing-side SQL predicate (`applyReviewQueueScope` in `authz_query.go`, projected via `permission.QueueScope`) must agree exactly, or a request that never surfaces in a queue becomes reviewable by knowing its id. The two have diverged before, which is why the agreement is pinned by tests rather than a comment.

## Non-negotiables

- Take the actor from `middleware.MustActorID(c)` / `middleware.ActorUser(c)`, never the body, query or path.
- **Grant-what-you-hold** is enforced by *every* authz mutator, not just `AssignRole`. Otherwise a holder of `roles.assign-permission` could add a permission they lack to a role they are in and acquire it. The check must be per-permission `Can(p)` so the super short-circuit fires: a set-subset formulation would leave a super-admin (zero rows) unable to grant anything.
- **Adding a catalogue entry reaches nobody automatically.** With no wildcards, a new identifier must be placed in a role bundle under `db/seeders/bundle/` and synced. `TestCatalogueIsFullyPlaced` (`db/seeders/bundles_test.go`) turns "added a permission and nobody can reach it" into a test failure. `is_system` roles are deliberately not reconciled on seed re-run, so operator retuning survives.
- **Boot invariant.** `cmd/api` refuses to start if `permissions` is empty or no active user holds a live `is_super` role (`postgres.CheckAuthzInvariant`). Fix with `go run ./db/seeders` or `wevetel authz:sync`.
- Fail closed. A repo error or missing actor denies, and not-found and not-permitted look the same where it matters.
- The operator CLI (`cmd/wevetel`) calls the application services with no gin middleware in front of it. Evidence gathered from `routeGuards` is evidence about the HTTP surface only. The SSH/TUI transport that used to be the other such surface was removed on 2026-07-30.

To answer who-can-do-what, read `routeGuards` + `internal/domain/permission/` + the relevant service's authorization helper (`access.authoriseReview`, `Authorizer.CheckRoleGrant`), never one file alone.
