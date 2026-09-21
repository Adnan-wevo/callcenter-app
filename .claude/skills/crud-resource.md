---
name: crud-resource
description: The nine-action CRUD contract every WevetelBastion resource follows - permissions, routes, service, drawers and gating, including the mandatory three-way soft-destroy/restore-destroy/destroy lifecycle split. Read before adding a resource or judging whether an existing one is complete.
---

# The CRUD resource contract

Every resource in this system is built the same way on purpose. The uniformity is
not tidiness: it is what lets a reader predict what exists for a resource without
reading its routes, lets the role editor lay every resource out identically, and
makes a missing piece visible as a gap in a known shape rather than as an absence
nobody notices.

A resource is complete when all four layers below agree. A resource that has an
endpoint but no permission is unreachable; one that has a permission but no
endpoint is a box an operator can tick that does nothing. Both are defects.

## A new resource is standalone

The uniformity above is a reason to copy an existing resource as a template. It is
NOT a reason to copy that resource's RELATIONSHIPS or its LABELS. When you model a
new resource on `sites`, you copy the shape of the seven actions, the drawer layout,
the whitelist mechanism. You do not copy its foreign keys, its joins, its counts of
related rows (a `site_count`), its attached-rows lists, or its wording ("Register").

- **No cross-module coupling.** A new resource joins another table only if its own
  shape genuinely requires it. A relationship dragged in from the template (a link
  back to sites on a resource that has nothing to do with sites) is a coupling the
  resource does not want and a dependency that outlives the copy-paste. A standalone
  resource persists and reads its own rows and nothing else.
- **No borrowed labels.** The resource names itself, in the right case for each
  surface. The swagger `swagger:route` tag is hyphenated, resource-specific and
  standalone (`VPN-Providers`, not `VPNProviders`, and never filed under a shared or
  legacy tag like the `Bastion-*` family). The UI display label is Title Case with
  spaces (`VPN Providers`). A verb copied from the template ("Register" on a resource
  that registers nothing) is wrong wording, not a shortcut.

## 0. The row itself

Every persisted entity embeds `entity.Base` (`internal/domain/entity/base.go`),
which is the whole of the id and timestamp convention:

```go
type Widget struct {
    entity.Base                     // id uuid PK, created_at, updated_at, deleted_at

    Name string `gorm:"type:varchar(128);uniqueIndex:idx_widgets_name,where:deleted_at IS NULL;not null" json:"name"`
}

func (Widget) TableName() string { return "widgets" }
```

- `ID` is a UUID assigned by `Base.BeforeCreate` when unset, not by the database.
  A caller may pre-set it, which is what lets a test pin an id.
- `CreatedAt` / `UpdatedAt` are GORM's `autoCreateTime` / `autoUpdateTime`.
- `DeletedAt` is `gorm.DeletedAt`, so every read is soft-delete filtered by GORM's
  default scope and a delete is reversible.

**An append-only entity MUST NOT embed Base.** `AuditLog` is the example: it is the
record of what happened, so an `updated_at` on it would be a column for rewriting
history and a `deleted_at` a column for erasing it. Such an entity declares its own
`ID` and `CreatedAt` and its own `BeforeCreate`.

**A unique index on a soft-deletable column must be PARTIAL**:
`uniqueIndex:idx_x_name,where:deleted_at IS NULL`. A plain unique index holds the
name for ever, so soft-deleting a row does not retire the identifier, it destroys
it: the dead row keeps the key and any later insert collides. Scoping the index to
live rows is what makes a soft delete a suspension rather than a permanent loss.

## Migrations

Models are the source of truth. Atlas diffs them; `AutoMigrate` is never used.

```
ATLAS_DB_URL="$(./scripts/atlas-url.sh)" atlas migrate diff <name> --env dev
ATLAS_DB_URL="$(./scripts/atlas-url.sh)" atlas migrate status --env dev
ATLAS_DB_URL="$(./scripts/atlas-url.sh)" atlas migrate apply --env dev
```

The generated table carries the Base columns in this shape, which is what a new
migration should look like:

```sql
CREATE TABLE "public"."widgets" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "name" character varying(128) NOT NULL,
  PRIMARY KEY ("id")
);
CREATE INDEX "idx_widgets_deleted_at" ON "public"."widgets" ("deleted_at");
```

Rules that bite:

- **Read the generated SQL before applying it.** Atlas writes what the diff implies,
  not what you meant.
- A `NOT NULL` column added to a populated table needs the add-default then
  drop-default pattern (see `db/migrations/20260716072041_*`).
- Any hand edit means `atlas migrate hash --dir file://db/migrations` afterwards.
- **Never edit a migration that has already run.** Its checksum is recorded in
  `atlas.sum` and in the database's revision table, so changing even a comment turns
  it into a checksum mismatch that blocks every later apply. That includes the
  em-dash sweep, which skips `db/migrations/` for this reason.
- A backfill belongs in the migration that introduces the column it fills, so an
  estate that runs the migration is never briefly in the state where the column
  exists and lies (see `20260720084503_permissions_is_system.sql`).

## 1. Nine permissions, always

`internal/domain/permission/permission.go` and `catalogue.go`, one constant plus
one `Definition` each:

| Action | Route | Gates in the UI | Notes |
|---|---|---|---|
| `index` | `GET .../index` | the list screen | scoped: `.own` / `.any` where the resource has an owner |
| `create` | none | the "new" button and the create drawer | no route: it gates the affordance, `store` is what accepts the write |
| `store` | `POST .../store` | the drawer's Save | |
| `show` | `GET .../show/:id` | the view icon and the read-only drawer | scoped like `index` |
| `edit` | none | the edit icon and the edit drawer | no route: it gates the affordance, `update` is what accepts the write. The drawer loads the record (and any option data) via `show` |
| `update` | `PUT .../update/:id` | the drawer's Save | |
| `soft-destroy` | `DELETE .../soft-destroy/:id` | the archive icon and its confirm dialog | the ORDINARY delete - see below |
| `restore-destroy` | `POST .../restore/:id` | the Restore button, shown only in the Archive view | reverses `soft-destroy` |
| `destroy` | `DELETE .../destroy/:id` | the "Delete permanently" button, shown only in the Archive view | PERMANENT - see below |

Plus whatever custom actions the resource genuinely has (`sync`, `assign`,
`revoke`, `give`, `spawn`, `terminate`, `approve`, `deny`, `cancel`, `export`).
A custom action is ONE permission shared by its route and its control.

### The three-way destroy split is MANDATORY for every resource, not an option

This was `destroy` alone until the Access Control screens' 2026-08 redesign,
where it was discovered that `roles.destroy` (and, earlier, `users.destroy`)
had always performed a SOFT delete under a name that promised something
permanent - the identifier's own name was lying about what it did. Every
resource built or touched from that point on carries three separately-gated
verbs instead of one, mirroring `internal/modules/iam/authz/service.go`'s
`SoftDestroyRole`/`RestoreRole`/`HardDestroyRole` (Roles is the reference
implementation - it is the first resource with all three working end to end;
Users has the first two but `users.destroy` is still reserved with no route,
which is the ONE acceptable reason a resource may ship with only two of the
three: declare `destroy` now, wire it later, never skip declaring it):

- **`soft-destroy`** is the everyday delete an operator reaches for. It
  soft-deletes the row (`gorm.DeletedAt`) - the resource disappears from the
  ordinary listing into an Archive view, and the resolution/authority
  consequences (if any - a role stops conferring, a user stops
  authenticating) take effect immediately. The row and every attachment it
  carries are left untouched, only hidden.
- **`restore-destroy`** reverses that: clears the soft-delete marker, the row
  reappears in the ordinary listing, and whatever it carried resolves again
  unchanged - it restores no authority of its own, because nothing was ever
  removed by the soft delete.
- **`destroy`** PERMANENTLY removes an ALREADY-ARCHIVED row: the row itself
  plus any join-table rows that reference it (`role_permissions`,
  `user_roles`, or this resource's own equivalents), in one transaction. It
  MUST refuse (`application.ErrConflict`, 409) a row that is currently LIVE -
  see `RoleRepository.HardDelete`'s doc comment for why: archiving first is
  what performs the revocation, and a permanent delete must not be able to
  skip past a row still in use. Join tables without `ON DELETE CASCADE`
  (check the migration - most in this schema are `ON DELETE NO ACTION`
  deliberately) need their dependent rows deleted explicitly, oldest-first,
  inside the same transaction as the row itself.

The repository interface gains `PageArchived(ctx, q PageQuery) (…)` and
`Restore(ctx, id) error` alongside the ordinary `Page`/`Delete`, plus
`HardDelete(ctx, id) error` for the permanent case. See
`internal/domain/repository/role_repo.go` for the exact shape all three take.

Three rules constrain the set:

- **Scopes on reads only.** `.own`/`.any` go on `index` and `show`,
  and on the custom actions where the scope is a real authority difference. Writes
  are unqualified: whether a particular row may be written is a service rule about
  ownership, and a scope there would put the same decision in two places and let
  them disagree.
- **Split assign from revoke.** Handing out authority and withdrawing it are
  different powers, and one identifier for both means delegating one always
  delegates the other.
- **The honesty rule.** If an action has no enforcement point on this resource,
  the identifier is still declared (the convention stays uniform) but its Title
  ends with `(inert - no endpoint)` and its Description opens by saying granting it
  confers nothing today. It then goes in `rootOnly`, never in a bundle. An operator
  ticking a box that does nothing while believing otherwise has been handed a
  security misconception, not a feature.

**Placement is mandatory.** `db/seeders/bundles_test.go::TestCatalogueIsFullyPlaced`
fails if an identifier is in no bundle and not in the reviewed `rootOnly` list, so
"added a permission and nobody can reach it" is a test failure rather than a
mystery. After adding entries: `wevetel authz:sync`, then
`wevetel authz:reconcile --apply` to reach roles that already exist. The seeder
deliberately does not reconcile pre-existing roles, so operator retuning survives.

## 2. The API

Routes are `/api/v1/{open|secure}/{module}/{function}/{action}`. Every guarded
route needs a row in `routeGuards` in `internal/surfaces/routes/router.go`; `guard()`
panics on a route with no row, so a route with no authority statement cannot ship.

- Handlers bind through `internal/kernel/validation`: `Bind[T]`, `BindQuery[T]`,
  `ListQuery(c)`, `Meta(q, total)`. They write the 422 and abort themselves.
- Collections answer `response.Paginated(c, rows, validation.Meta(q, total))`,
  never a bare slice, and never a nil slice.
- Errors go through `response.FromServiceError`. Never pass `err.Error()` to
  `response.Error`.
- Nothing decides authority in a handler. The service does.

Every collection supports `page`, `per_page`, `search`, `sort`, `order` and
`filter[<column>]=<value>` (with `a..b` for a date range). **Sort and filter keys
are column names that reach SQL**, so both are whitelisted twice: once in the
service (`application.FilterSpec`, the sort list), once in the repository
(`PageQuery.OrderBy`, `PageQuery.FilterBy`). The second check is the one next to
the danger. Search terms go through `repository.EscapeLike` with an explicit
`ESCAPE` clause.

## 3. The web screen

One route per resource, one list screen, three drawers. There are no `/new` or
`/:id` form routes.

**The screen is built on the bespoke table+drawer pattern in the
`central-list-page` skill. That skill's own opening section states the current
rule in full - read it before building or redesigning any screen - and this
section assumes it: `app-data-table` (`web/src/app/shared/components/data-table/`)
is the LEGACY shell, kept only for a resource with genuinely no lifecycle to
redesign into (a pure append-only trail, live ephemeral state with no
soft-delete concept). A new resource, and any existing resource being
redesigned, is built on the hand-rolled table+drawer shape - its own desktop
table / mobile card list, `toggleSort`/`sortIcon` per column, an inline filter
panel, `app-crud-drawer`/`app-show-drawer`. Wiring the archive/restore/destroy
lifecycle onto an existing resource while leaving its `app-data-table` shell in
place is a half-migration, not a redesign - it looks unchanged to the person
who asked for one.**

- **Show** is read-only: header, a `<dl>` of label and value, one Close button. No
  form, no Save.
- **Edit** is the form drawer, fed by `GET .../show/:id` (which now carries any option data the form needs), not by the list row.
- **Create** is the same form drawer with no record behind it.
- Every drawer discards its state on close (`resetForm`), so a reopened drawer
  never shows a stale error about input the user has since changed.

**Archive / Restore / Delete permanently follows the three-way split above,
built into the bespoke pattern's own shell.** An All / Archive-only view control
(a chip, a select, a tab - whatever the screen's filter UI already uses for
similar toggles) switches the list between the live `Page` and the archived
`PageArchived` read. The archive icon replaces the old delete icon on a live
row; a row in the Archive view gets Restore and, if `destroy` is wired,
"Delete permanently" instead. All three still go through ONE confirm dialog
keyed by an action kind (`{row, kind: 'archive' | 'restore' | 'destroy'}`),
not one dialog per action - see `RolesListComponent.pendingAction` for the
pattern.

**The table rule: every shown column carries its OWN per-column filter and is
sortable, or it is not shown.** There is no sort-only column and no filterless
column. Each shown column pairs a backend filter-whitelist entry with a web column
filter, one to one: if you cannot give a column a filter, it does not go in the
list, it goes in the Show drawer. Everything else lives in the drawer too. Prose
never goes in a table cell, and record-metadata dates (created, updated, last
login) belong in the drawer unless you filter them there with a date-range control.
A header that looks sortable and does nothing, or a column with no filter beside its
neighbours that have one, is the same class of defect as a search box that silently
matches nothing: the user concludes the data is wrong. The failure to avoid is a
generated list where one column is filterable and the rest are decoration.

A filter control must follow how the server matches the column: a text box for a
containment match, a select for a closed set, a date-range control for a
timestamp. A text box against an exact-match column returns nothing for a value
that exists.

Row actions are icon buttons with a tooltip AND an `aria-label` (a tooltip is not
an accessible name).

## 4. Gating

`*hasPermission` in templates, `PermissionStore.can/canAny/canAll` in components.
Gate the control AND the method behind it. Note that `*hasPermission` OR-combines
an array: for an AND, use a computed `canAll`.

Gating is a courtesy, not the enforcement. The server refuses regardless, and no
comment should claim otherwise. But a control the caller cannot use is not
rendered: offering a button that always answers 403 is its own bug.

## Completion checklist

**Before ticking the sort/filter lines below, run `list-screen-verify.md`'s
mechanical checks and paste their real output.** A ticked box here is a claim
someone followed the checklist; it is not proof any column actually sorts.
That gap shipped for real once (three gateway screens, each with a code
comment asserting a column could not be sorted, each wrong) - the mechanical
check is what a checklist line cannot be.

- [ ] Nine constants and nine Definitions (index/create/store/show/edit/update/soft-destroy/restore-destroy/destroy), scopes on reads only, inert entries marked
- [ ] `soft-destroy`/`restore-destroy` actually wired (route, service method, repository method); `destroy` at minimum DECLARED even if not yet wired - never silently dropped to a single `destroy`
- [ ] `destroy`'s service method refuses a LIVE row with `ErrConflict`, and its repository method removes dependent join rows before the row itself, inside one transaction
- [ ] Every identifier in a bundle or in `rootOnly` (`TestCatalogueIsFullyPlaced`)
- [ ] `routeGuards` row per route, permissions taken from the catalogue constants
- [ ] `wantPerms` and `wantRoutes` updated in `internal/surfaces/routes/*_test.go`, reasoned from the route rather than copied from `routeGuards`
- [ ] Service methods authorise, scope by ownership, and return the four sentinels
- [ ] Sort and filter whitelists in BOTH the service and the repository, and every column name appears in BOTH lists - a column added to only the repository's whitelist is refused one layer up and silently falls back to the default sort, which is the exact bug that shipped on Departments/Groups' `permission_count`
- [ ] Swagger annotations in `internal/surfaces/docs/`, each response set listing what the route can actually return
- [ ] List screen built on the bespoke table+drawer pattern (`central-list-page.md`), not `app-data-table`, unless the resource is genuinely lifecycle-less (append-only trail, live ephemeral state)
- [ ] Show drawer, Edit drawer, Archive view, confirm dialog (one, keyed by action kind), all gated
- [ ] Every table column sortable and per-column filterable (one backend whitelist entry + one web column filter each); everything else in the drawer
- [ ] No cross-module coupling and no borrowed labels: standalone rows, hyphenated resource-specific swagger tag, Title Case UI label
- [ ] `go build ./... && go vet ./... && go test ./...`, `gofmt -l` clean, `swagger validate`, `tsc`, `ng build`, `ng test`
