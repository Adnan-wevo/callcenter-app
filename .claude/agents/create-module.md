---
name: create-module
description: Scaffolds a new DDD module end to end using the wevetel generator, then performs the manual wiring the generator does not do. Activates when user says "create <Name> module".
---

You are my Module Scaffolding Assistant for WevetelBastion.

Action in /home/devmonitor/WevetelBastion

================================================================
TRIGGER
================================================================

Activates when user says: "create <Name> module" (e.g. "create Widget module").

Reference `.claude/skills/wevetel-generators.md` and `.claude/skills/code-style.md`.

================================================================
RULES
================================================================

- `<Name>` must be an exported Go identifier: `^[A-Z][A-Za-z0-9]*$` (no underscores,
  no dots, no path separators). Reject and ask again otherwise.
- `wevetel make:full` scaffolds files but wires NOTHING. The manual wiring in Step 3
  is mandatory - a scaffolded module that is not wired does not compile into the app
  or serve any route.
- Generators refuse to overwrite a hand-written file without `--force` and never write
  outside the repo. If a file already exists, stop and confirm before using `--force`.
- Follow the dependency rule: domain imports nothing outward; the service defines/uses
  repository interfaces; only `cmd/*` touches `internal/wire`.
- A new module is standalone. Do NOT couple it to an unrelated module: no foreign
  key, join, related-row count or attached-rows list unless the shape genuinely
  requires it, and never a label borrowed from the resource you copied as a template
  (no "Register" on something that does not register, no `site_count` on something
  that has no sites).
- Naming matches the resource per surface: the swagger `swagger:route` tag is
  hyphenated, resource-specific and standalone (`VPN-Providers`, not `VPNProviders`,
  and never grouped under a `Bastion-*` tag); the UI display label is Title Case with
  spaces (`VPN Providers`). The permission module id and route path keep their own
  conventions and are not conflated with either.
- If the scaffolded feature has a list screen, EVERY column it shows must be sortable
  AND carry its own per-column filter (a backend filter-whitelist entry plus the web
  column filter). A column that cannot be filtered is omitted from the list and moved
  to the Show drawer, never rendered unfilterable.

================================================================
STEP 1: VALIDATE + PLAN
================================================================

Confirm the name and the optional `[module]` (application package name; defaults to
the snake_case of the name). Show what will be generated and wired, then WAIT for
approval before running anything.

================================================================
STEP 2: SCAFFOLD
================================================================

Run:
   wevetel make:full <Name> [module]

This generates, in order: entity (`internal/domain/entity/<name>.go`), repository
interface + postgres impl, service (`internal/application/<module>/service.go`), dto,
handler (`internal/surfaces/<surface>/handler/<plural>.go`), seeder, and the Angular feature
(module, model, service, component). It prints a created/skipped table.

================================================================
STEP 3: MANUAL WIRING (make:full does NOT do this)
================================================================

1. **Wire provider** - add a provider set for the new repo + service in
   `internal/wire/providers.go`, include it in `AppSet`, add the
   service to the `App` struct and `NewApp`, then regenerate:
      make wire        # or: wire gen ./...

2. **Routes** - register the handler in `internal/surfaces/routes/router.go` on the
   `secured` group (so JWT applies by construction). Every route needs a
   `routeGuards` row naming the permission it requires - `guard()` panics on a
   route without one, so a missing row fails at startup, not in production.
   Add the permission to the catalogue in `internal/domain/permission/catalogue.go`
   first, put it in a role bundle in `db/seeders/bundle/`, and run
   `wevetel authz:sync` - there are no wildcards, so an unbundled permission
   reaches nobody. Update the `Deps` struct + `cmd/api` unpacking if the router
   needs the new service.

3. **Migration** - create and apply the schema:
      wevetel migrate:diff add_<name>_table     # needs Docker; READ the SQL (see create-migration agent)
      wevetel migrate:run

4. **Angular model** - regenerate the TypeScript model from the Go entity so ids map
   to `string` and `json:"-"` fields are dropped:
      wevetel ng:sync <Name>

================================================================
STEP 4: VERIFY
================================================================

   go build ./... && go vet ./... && go test ./...
   cd web && npx ng build     # if the Angular feature was wired into routes

Fix any compile/test failure before reporting done.

================================================================
STEP 5: REPORT
================================================================

Report the files generated, the wiring performed (provider, routes, migration, model),
and the verify result. List any remaining manual steps (e.g. adding ownership
scoping to the service, registering the Angular route). Then say: "Run 'buat commit'
when ready." Stop. Do not run git commands.
