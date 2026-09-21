---
name: create-crud
description: Builds a complete CRUD resource end to end - seven permissions, the REST surface, the service and repository, the OpenAPI annotations and the Angular list plus drawers - wired and verified. Activates when user says "create crud <Name>" or "add <Name> resource".
---

You are my CRUD Resource Builder for WevetelBastion.

Action in /home/devmonitor/WevetelBastion

================================================================
TRIGGER
================================================================

Activates when the user says: "create crud <Name>", "add <Name> resource",
"buat crud <Name>".

================================================================
TWO MODES
================================================================

Decide which one applies BEFORE step 1, and say which you picked.

**FULL (default).** The resource does not exist yet. You build it from the table
up: entity, migration, repository, service, API, OpenAPI, web. Steps 2 through 8.

**CRUD-ONLY.** The table and the entity already exist and are correct, and the user
wants the surface over them. Skip step 2 entirely and start at step 3. Triggered by
"crud only", "tanpa migration", "table dah ada", or by you finding an entity and a
migrated table for `<Name>` already present.

How to tell, rather than asking twice: look for `internal/domain/entity/<name>.go`
and a `CREATE TABLE` for its `TableName()` under `db/migrations/`. If both exist,
this is CRUD-ONLY and you say so. If exactly one exists, STOP and report it: an
entity with no migration, or a table with no entity, is a half-finished state and
guessing which way to resolve it is how a schema and a model drift apart.

In CRUD-ONLY mode you may still need a MIGRATION for a column the surface requires
and the table lacks (a `created_at` a listing sorts by, for instance). That is
allowed: add the column, do not rebuild the table.

READ FIRST, in this order, and do not start writing until you have:

1. `.claude/skills/crud-resource.md` - THE CONTRACT. Everything below assumes it.
2. `.claude/skills/authz-patterns.md` - the permission vocabulary.
3. `.claude/skills/api-handler-patterns.md` - handler, binding, envelope.
4. `.claude/skills/code-style.md` - comment style and the house rules.
5. An existing resource end to end as the worked example. `sites` is the simplest
   complete one: `internal/domain/entity/site.go`, `internal/application/site/`,
   `internal/platform/postgres/site_repo.go`, the `sites` rows in
   `internal/surfaces/routes/router.go`, `internal/surfaces/<surface>/handler/sites.go`,
   `internal/surfaces/docs/sites.go`, `web/src/app/features/sites/`.

================================================================
RULES
================================================================

- `<Name>` must be an exported Go identifier: `^[A-Z][A-Za-z0-9]*$`. Reject anything
  else and ask again.
- `wevetel make:full <Name> <module>` scaffolds files and wires NOTHING. Every wiring
  step below is mandatory; a scaffolded resource that is not wired compiles into no
  route and serves nobody.
- The dependency rule is enforced and load-bearing: `internal/domain` imports stdlib,
  uuid and datatypes only; `internal/application` declares the interfaces it needs;
  only `cmd/*` may import `internal/wire`.
- Never invent a permission name. The shape is `module.function[.qualifier]` with a
  CLOSED qualifier vocabulary (own, any, and the three site tiers).
  `permission.Name.Valid()` is the check; the catalogue test enforces it.
- Never widen an existing permission to cover a new route. Add the identifier the
  route needs.
- No em dashes. Use a comma, a colon, a full stop or parentheses.
- Comment WHY, not what. Match the density of the file you are editing. This
  codebase's comments carry the reasoning; a reviewer will ask for it.
- **A new resource is standalone. Do not couple it to an unrelated module and do
  not borrow another module's labels.** When you copy `sites` (or any resource) as
  the worked example, you are copying the SHAPE, not its relationships. Do not drag
  in its foreign keys, its joins, its counts of related rows, or its wording. A
  resource named `Widget` says "Widget", never "Site" or "Register", and joins to
  another table only if the shape you agreed in step 1 genuinely requires it. A
  borrowed relationship (a `site_count`, an attached-rows list) or a borrowed verb
  ("Register" on a resource that is not registering anything) is a defect, not a
  head start.
- **Naming matches the resource, in the right case for each surface.** The swagger
  `swagger:route` tag is hyphenated, resource-specific and standalone:
  `VPN-Providers`, not `VPNProviders`, and not filed under a shared or legacy tag
  (never group a new resource under the `Bastion-*` tags). The UI display label
  (sidebar, page title, headings) is Title Case with spaces: `VPN Providers`. The
  permission module id and the route path keep their own conventions
  (`vpn-providers.*`, `/secure/vpn/providers/...`); do not conflate the three.

================================================================
STEP 1: AGREE THE SHAPE BEFORE WRITING
================================================================

Ask, and do not guess:

- Which module does it belong to: `acl`, `bastion`, `core`, or a new one?
- Does the resource have an OWNER (a user_id)? That decides whether
  `index` and `show` carry `.own` / `.any`, or no scope at all.
- What custom actions does it genuinely have beyond the seven?
- Which fields are secret or credential material? Those never enter a response DTO.
- Which columns should the table show? **Every column shown in the list must be
  sortable AND carry its own per-column filter.** A column you cannot filter is
  OMITTED from the list and shown in the drawer instead, never rendered
  unfilterable. So this question is really "which columns will the repository
  filter-whitelist", and the list is exactly that set, no wider.

State the resulting plan back in one short block and get a yes before writing.

================================================================
STEP 2: DOMAIN AND MIGRATION            (FULL mode only)
================================================================

Read section 0 and the Migrations section of `.claude/skills/crud-resource.md`
first: they carry the id and timestamp convention and the traps.

1. **Entity** in `internal/domain/entity/<name>.go`, embedding `entity.Base`, which
   is where `id uuid` (assigned by `BeforeCreate`, not by the database),
   `created_at`, `updated_at` and `deleted_at` come from. Give it a `TableName()`.
   Every field gets a gorm tag with an explicit type and a json tag.

   Two decisions that are easy to get wrong and expensive to undo:

   - If the resource is APPEND-ONLY (a log, a trail, an event record), it must NOT
     embed Base. An `updated_at` on such a row is a column for rewriting history and
     a `deleted_at` is a column for erasing it. Declare `ID` and `CreatedAt` and a
     `BeforeCreate` by hand, as `entity.AuditLog` does.
   - Any unique index on a soft-deletable table must be PARTIAL
     (`uniqueIndex:idx_x_name,where:deleted_at IS NULL`), or soft-deleting a row
     does not retire its name, it destroys it: the dead row keeps the key for ever
     and any later insert collides.

2. **Repository interface** in `internal/domain/repository/`, including
   `Page(ctx, q PageQuery) ([]entity.X, int64, error)` and any scoped variant the
   resource needs (`PageByUser`).

3. **Migration** through Atlas, never AutoMigrate. Models are the source of truth:

   ```
   ATLAS_DB_URL="$(./scripts/atlas-url.sh)" atlas migrate diff <name> --env dev
   ```

   Then READ the generated SQL before applying it, and check it against the shape in
   the skill: `id uuid NOT NULL`, three `timestamptz NULL` columns, the
   `idx_<table>_deleted_at` index, `PRIMARY KEY ("id")`.

   - A NOT NULL column on a populated table needs the add-default then drop-default
     pattern (`db/migrations/20260716072041_*`).
   - A backfill belongs in the same migration as the column it fills, so no estate
     is ever briefly in the state where the column exists and lies.
   - Any hand edit means `atlas migrate hash --dir file://db/migrations`.
   - Never edit a migration that has already run. Its checksum is recorded in
     `atlas.sum` AND in the database's revision table, so changing even a comment
     blocks every later apply.

   Apply it, then confirm with `atlas migrate status --env dev`.

================================================================
STEP 3: PERMISSIONS
================================================================

1. Seven constants in `internal/domain/permission/permission.go`, plus the custom
   actions. Scopes on `index` and `show` only.
2. Seven `Definition`s in `catalogue.go`, each with a Title and a Description that
   says what the permission admits and what it does NOT imply. An action with no
   enforcement point gets `(inert - no endpoint)` in its Title and says so first in
   its Description.
3. Update the count assertion in `catalogue_test.go` and, if the module is new, the
   module list in `TestModules`.
4. Place every identifier in `db/seeders/bundle/bundle.go`: a bundle if a built-in
   role should hold it, `rootOnly` otherwise, each with a one-line reason. Inert
   identifiers always go to `rootOnly`: a permission that does nothing must never be
   handed out.
5. `go test ./internal/domain/permission/ ./db/seeders/` must pass, which is what
   proves the catalogue and the bundles agree.
6. `go run ./cmd/wevetel/ authz:export` to regenerate the TypeScript union.

================================================================
STEP 4: APPLICATION SERVICE
================================================================

One service per resource in `internal/application/<name>/`:

- `service.go` with the methods the seven actions need. Every mutating method takes
  the actor id, re-reads the actor, resolves authority live, and checks the
  permission itself. The transport is outside the trust boundary.
- `dto.go` with input DTOs carrying `validate:` tags and response DTOs that omit
  every secret. Response mappers return the zero value for a nil input and build
  slices with `make([]T, 0, n)` so JSON emits `[]` and not `null`.
- Scope reads by ownership INSIDE the query, never after the LIMIT:
  filtering a page after it is fetched returns a thin page beside a total that
  counts rows the caller is not being shown.
- Sort and filter whitelists (`FilterSpec`, the sort column list) live here as the
  first of the two stages.
- Return the four sentinels from `internal/application/errors.go`. Where a rule
  knows which field it rejected, return `application.ValidationError` so the field
  name survives to a 422.

================================================================
STEP 5: REPOSITORY
================================================================

In `internal/platform/postgres/`:

- Implement the interface. `Page*` builds the COUNT and the SELECT from ONE shared
  filter helper, so the total can never describe a different set from the rows.
- Ordering comes only from `q.OrderBy(<whitelist>, <default>)`; filters only from
  `applyFilters` / `q.FilterBy(<whitelist>)`. Never interpolate a caller's string.
- Search terms go through `repository.EscapeLike` with an explicit `ESCAPE '\'`.
- Qualify columns with their table wherever the query joins, or an ambiguous column
  becomes a runtime error that only fires on one code path.
- Translate errors: `notFound()` on a miss, `conflict()` on a unique violation.
  Without them an unknown id is a 500 and a duplicate is a 500.
- Regenerate the mock with the command recorded in the mock file's header.

================================================================
STEP 6: TRANSPORT
================================================================

1. Handler in `internal/surfaces/<surface>/handler/` with methods named `Index`, `Show`, `Store`,
   `Update`, `Destroy` plus the custom actions. There is NO `Edit` handler: `edit` is
   a UI-gate permission with no route, and the edit drawer reads through `Show`. Bind through
   `internal/kernel/validation`. Answer collections with `response.Paginated`.
2. A `routeGuards` row per route in `internal/surfaces/routes/router.go`, then mount it
   through `guard()` in the right `register*` function.
3. Update `permission_table_test.go`'s `wantPerms` and `router_test.go`'s
   `wantRoutes`. Read their doc comments first: they are a deliberate SECOND opinion,
   written by reasoning about what the route should demand. Copying `routeGuards`
   into them destroys the only check that catches a wrong permission on a live route.
4. Annotations in `internal/surfaces/docs/`, then regenerate:
   `swagger generate spec -w . -o internal/surfaces/docs/swagger.json --scan-models`
   and `swagger validate internal/surfaces/docs/swagger.json`.
   The `swagger:route` tag is the resource's own, hyphenated and standalone:
   `VPN-Providers`, not `VPNProviders`, and never grouped under a shared or legacy
   tag such as the `Bastion-*` family. The response models carry only this
   resource's fields: do not leave a borrowed relationship (a `site_count`, an
   attached-rows list) in a model you copied from another resource.
   Traps: go-swagger parses the word `Example:` anywhere in a comment, and a
   `default:` keyword too; `example` is forbidden on a header object.

================================================================
STEP 7: WEB
================================================================

In `web/src/app/features/<name>/`:

Read `.claude/skills/central-list-page.md` first: the bespoke table+drawer pattern is
the STANDARD shape for a new resource's list screen, not `app-data-table`.
`app-data-table` is the legacy shell now, kept only for a resource with genuinely no
lifecycle (a pure append-only trail, live ephemeral state with no soft-delete
concept) - an ordinary CRUD resource with the nine-action contract from Step 3 builds
on the bespoke pattern.

- A service calling through `ApiService` (never `HttpClient` directly) at the v2
  paths, including `archived*Page`/`softDestroy*`/`restore*`/`hardDelete*` for the
  three-way lifecycle.
- One list screen on the bespoke pattern: its own desktop `<table>` / mobile card
  list, `toggleSort`/`sortIcon` per column, an inline filter panel with removable
  chips. **Every column carries its own per-column filter AND is sortable**, matched
  to the repository whitelists (both layers - see `crud-resource.md`'s whitelist
  parity rule); the filter kind must match how the server matches the column (a text
  box for containment, a select for a closed set, a date-range for a timestamp).
  There is no such thing as a sort-only or a filterless column here: a column that
  cannot be filtered does not belong in the list, it belongs in the Show drawer. Copy
  the shell from whichever of the four reference implementations (Roles, Tenant
  Registry, Departments, Groups - see `central-list-page.md`) is closest in shape to
  the new resource.
- The list labels and headings name THIS resource in Title Case (`VPN Providers`,
  `Add provider`, `New provider`), never a label copied from the reference resource
  ("Register site", "Register provider"). Do not port the reference's related-row
  column (a `member_count`) or its attached-rows section unless the new resource
  genuinely has the same shape: material or relationship presence belongs in the
  drawer, and an unrelated relationship does not belong at all.
- A Show drawer (`app-show-drawer`), read-only, `direction="bottom"`,
  `[disableClose]="true"`, sized with `DRAWER_LARGE_BOTTOM_SHEET` on desktop.
- An Edit drawer (`app-crud-drawer`, same sizing/disableClose) fed by
  `GET .../show/:id` (which carries any option data the form needs), resetting on
  close. There is no `edit/:id` route.
- An All/Archive view filter and one `pendingAction` signal (`kind: 'archive' |
  'restore' | 'destroy'`) driving one `app-confirm-dialog` - see
  `central-list-page.md`'s "three-way destroy split" section for the exact shape,
  Roles for the file to copy.
- Row actions as icon buttons with tooltip and `aria-label`, gated per action.
- A `*.routes.ts` for the feature, then the entry in `app.routes.ts` and the nav item
  in `web/src/app/layout/wevetel-sidebar.ts`.
- 422s bind to fields through `applyServerErrors`; the global dialog never sees them.

================================================================
STEP 8: VERIFY, THEN REPORT
================================================================

**First, run `.claude/skills/list-screen-verify.md`'s mechanical sort/filter
checks and paste their real output.** Do this before the build commands, not
after - a screen that fails the sort/filter count check needs a code fix,
which changes what the build/test run below is even verifying. "I gave every
column a filter and made it sortable" is a claim; the `th`/`toggleSort` count
is the proof, and skipping straight to `go build` is how a column with no
sort control shipped anyway on a prior redesign.

Then run all of these and paste the real output, do not summarise a run you did not do:

```
go build ./... && go vet ./... && go test ./...
gofmt -l internal/ cmd/ db/
swagger validate internal/surfaces/docs/swagger.json
cd web && npx tsc -p tsconfig.app.json --noEmit && npx ng build && npx ng test --watch=false --browsers=ChromeHeadless
```

Then apply the catalogue to a running estate:

```
go run ./cmd/wevetel/ authz:sync
go run ./cmd/wevetel/ authz:reconcile          # read the plan
go run ./cmd/wevetel/ authz:reconcile --apply  # commit it
```

Report: which MODE you used and why, the seven permissions and where each is
placed, the routes with their guards, what you put in the table versus the drawer and why, and anything in the
contract you could not satisfy. If a step failed, say so with the output rather
than reporting success.

================================================================
WHAT NOT TO DO
================================================================

- Do not reuse an existing permission because it is "close enough".
- Do not put a control in the UI that the server will refuse; gate it.
- Do not put a column in a table that cannot be sorted and per-column filtered;
  drawer it. One shown column, one backend filter-whitelist entry, one web column
  filter, always.
- Do not couple a new resource to an unrelated module, and do not carry over its
  labels or wording. No `site_count` on a Widget, no "Register" on a resource that
  is not registering.
- Do not name the swagger tag `VPNProviders` or file it under a `Bastion-*` tag; it
  is `VPN-Providers`, hyphenated and standalone. The UI label is `VPN Providers`.
- Do not weaken or delete a test to make a build pass. If a test is wrong, say why.
- Do not edit an applied migration.
- Do not commit. The user runs the git workflow separately.
