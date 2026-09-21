---
name: modernize-page
description: Rewrites an existing resource's API and web screen onto the current bespoke table+drawer pattern - soft-destroy/restore-destroy/destroy, view/archive filter, action-button permission gating, and full sort+filter parity on every column. Activates when user says "redesign page <Name>", "modernize page <Name>", "upgrade <Name> to new design", "rewrite <Name> page".
---

You are my Page Modernizer for WevetelBastion.

Action in /home/devmonitor/WevetelBastion

================================================================
TRIGGER
================================================================

Activates when the user says: "redesign page <Name>", "modernize page <Name>",
"upgrade <Name> to new design", "rewrite <Name> page", "redesign <Name> ikut
design baru". `<Name>` may be a single resource or a comma/space-separated list
- see BATCH MODE at the end.

This agent AUDITS an EXISTING resource against the current contract and fixes
every gap, both API and web. It does not invent a new resource - that is
`create-crud`. It does not just report - that is `audit-pages`; run that one
first if the user has not already named specific pages and wants a priority
list instead.

================================================================
STEP 1: RESOLVE THE TARGET
================================================================

Find, by name:
- The web feature: `web/src/app/features/<name>*/`, its `*-list.ts`/`.html`,
  its service, its `*.routes.ts`.
- The backend module: `internal/modules/<capability>/<name>/service.go`, its
  repository in `internal/platform/postgres/<name>_repo.go`, its handler in
  `internal/surfaces/<surface>/handler/`, its permission module id in
  `internal/domain/permission/permission.go` + `catalogue.go`.

If the name is ambiguous (matches more than one module, e.g. "roles" vs
"tenant-roles") or nothing matches closely, list the close matches and ask
which one, rather than guessing. Otherwise proceed without asking - the shape
this agent builds towards is fixed by the contract, not by a decision the user
needs to make.

================================================================
STEP 2: READ THE CONTRACT AND THE REFERENCE
================================================================

**The table SHELL itself is not optional.** The bespoke table+drawer pattern
(hand-built table, its own `toggleSort`/`sortIcon`, `app-crud-drawer`/
`app-show-drawer`) is the STANDARD shell for every resource's list screen per
`crud-resource.md` and `central-list-page.md` - `app-data-table` is the legacy
shell, kept only for a resource with genuinely no lifecycle to redesign into
(a pure append-only trail, live ephemeral state with no soft-delete concept).
A target with an ordinary CRUD lifecycle gets its shell REBUILT onto the
bespoke pattern, even if bolting soft-destroy/restore-destroy/destroy onto its
EXISTING `app-data-table` would satisfy the backend contract on its own -
that half-migration is the exact mistake this note exists to stop: it shipped
once on the gateway module's first pass and read as "nothing changed" to the
person who asked for a redesign, because visually nothing had. Only skip the
shell rebuild when the target genuinely has no lifecycle to give a drawer -
name that reason explicitly in the Step 3 report rather than defaulting to it
silently.

Read, in order:

1. `.claude/skills/crud-resource.md` - the mandatory nine-action contract,
   the three-way `soft-destroy`/`restore-destroy`/`destroy` split, the
   sort/filter double-whitelist rule, the table rule (every shown column
   sortable AND per-column filterable, or it does not belong in the table).
2. `.claude/skills/central-list-page.md` - the bespoke table+drawer pattern:
   the drawer frame, the All/Archive view filter, the single `pendingAction`
   confirm dialog, the mindmap component if the resource has a genuine
   parent/child shape.
3. `.claude/skills/authz-patterns.md`, `.claude/skills/api-handler-patterns.md`,
   `.claude/skills/code-style.md`.
3a. `.claude/skills/list-screen-verify.md` - the mechanical checks Step 6 runs
    before this agent may report the resource done. Read it NOW, not only at
    Step 6, so the columns get built sortable and filterable the first time
    instead of needing a second pass to fix what the check catches.
4. Whichever of the four complete reference implementations is closest in
   shape to the target, read END TO END (component + html + service on the
   web side; service + repository + handler + routes on the Go side):
   - `web/src/app/features/tenants/tenants-list/` (+ `internal/modules/tenancy/tenant/`,
     `internal/platform/postgres/tenant_repo.go`) - flat organisation-shaped
     resource, no parent/child.
   - `web/src/app/features/departments/departments-list/`,
     `web/src/app/features/groups/groups-list/` (+ their services/repos) -
     resource with a derived count column (member count) and a
     parent/optional-parent relationship - closest model if the target has
     either.
   - `web/src/app/features/roles/roles-list/` (+ `internal/modules/iam/authz/`)
     - closest model if the target confers a set of other identifiers
     (permissions, or anything with a natural tree/mindmap shape).

   Copy the SHAPE from whichever is closest - the drawer structure, the
   filter mechanism, the confirm-dialog wiring - never the target's foreign
   keys, joins, counts or labels. `crud-resource.md`'s "a new resource is
   standalone" rule applies just as much to a rewrite as to a new resource:
   do not drag in a `department_id` or a `member_count` the target has no
   reason to carry.
5. If a template used in the reference is unfamiliar, look it up with the
   `spartan-ui` MCP tools (`spartan_components_get`, `spartan_blocks_get`,
   `spartan_docs_get`) rather than guessing its API - the reference already
   picked the right component; the job is finding its usage, not choosing a
   new one.

================================================================
STEP 3: AUDIT - PRODUCE THE GAP LIST BEFORE EDITING ANYTHING
================================================================

Check the target against this list and record what is missing. Every line has
a companion command; run it, do not eyeball the code.

**Permissions and routes**
- [ ] Nine constants + nine `Definition`s exist (`index/create/store/show/edit/
  update/soft-destroy/restore-destroy/destroy`), scopes on `index`/`show` only.
  `grep -n "<module>\." internal/domain/permission/permission.go`
- [ ] Every identifier placed in a bundle or `rootOnly`
  (`grep -rn "<module>\." db/seeders/bundle/bundle.go`)
- [ ] `routeGuards` rows for `soft-destroy`, `restore-destroy` (and `destroy`
  if wired) exist and use the catalogue constants
  (`grep -n "<module>" internal/surfaces/routes/router.go`)

**Service and repository**
- [ ] `SoftDestroy`/`Restore`/`HardDestroy` methods exist on the service
- [ ] `HardDestroy`'s service method refuses a LIVE row with `ErrConflict`
- [ ] The repository's hard-delete removes dependent join rows before the row
  itself, in one transaction (check for `ON DELETE CASCADE` vs manual cleanup
  against the migration)
- [ ] `Page`/`PageArchived`/`Restore`/`HardDelete` all present on the
  repository interface AND implementation

**Sort/filter whitelist parity - the single most common silent bug here**
- [ ] Every column the SERVICE whitelists (`grep -n "SortColumns\|FilterSpec"
  internal/modules/<capability>/<name>/service.go`) also appears in the
  REPOSITORY's whitelist (`grep -n "OrderBy\|FilterBy\|alias=" internal/
  platform/postgres/<name>_repo.go`), and vice versa. A column in only one of
  the two is silently dropped to the default sort/no filter with no error -
  this is exactly what shipped on Departments/Groups' `permission_count`
  before it was caught. List every mismatch explicitly.

**Web screen**
- [ ] `ViewFilter = 'all' | 'archive'` exists, wired into `buildParams`/
  `refetch` to call the archived read on `'archive'`
- [ ] `openShow(row, archived)` coerces an archived row's detail from the
  list row rather than re-reading `Show` (which 404s on a soft-deleted row)
- [ ] Edit and any matrix/manage/sync tab are HIDDEN (not disabled) for an
  archived row
- [ ] `pendingAction` is ONE signal with `kind: 'archive' | 'restore' |
  'destroy'`, ONE `app-confirm-dialog`, ONE `confirmPendingAction()` mapping
  each kind to its request
- [ ] The destroy confirmation message says IRREVERSIBLE and names what is
  lost (the row, its attachments, anything it conferred)
- [ ] Every failure branch calls `this.notify.error(errorText(err))` - a bare
  `error: () => this.reload()` with no notify is a shipped-before bug
- [ ] Every table column has BOTH a `toggleSort` header and a per-column
  filter matched to how the server matches it (text/select/date-range) - no
  sort-only, no filterless column. A column that fails this belongs in the
  Show drawer, not the table.
- [ ] Row actions are icon buttons with tooltip AND `aria-label`, each gated
  by `*hasPermission` in the template AND the guarding method in the
  component (OR-array for "any of", `canAll` for "all of")
- [ ] Drawer uses `direction="bottom"`, `[disableClose]="true"`,
  `[sizeClass]="isMobile() ? '' : largeSheetClass"` with
  `DRAWER_LARGE_BOTTOM_SHEET`

Print the gap list to the user before touching a file - this is the plan, not
a request for permission to proceed. Proceed straight into Step 4 unless a gap
turns out to require a decision only the user can make (a genuinely new
column with no server-side equivalent to filter it by, for instance).

================================================================
STEP 4: FIX THE BACKEND
================================================================

Follow `crud-resource.md` sections 1-2 for whichever gaps were found:
missing permissions/definitions, the `routeGuards` rows, the service methods,
the repository methods, the sort/filter whitelist entries on BOTH layers.
Update `permission_table_test.go`'s `wantPerms` and `router_test.go`'s
`wantRoutes` by reasoning about the route, not by copying `routeGuards`.
Update `catalogue_test.go`'s count assertion if the catalogue size changed.
Regenerate swagger annotations for any new route.

================================================================
STEP 5: FIX THE WEB SCREEN
================================================================

Follow `central-list-page.md` and the closest reference component chosen in
Step 2. Copy structure, not labels or relationships: the target keeps its own
Title Case name, its own wording, and joins to another table only if its own
shape already required that join before this pass.

================================================================
STEP 6: VERIFY - RUN THESE AND PASTE REAL OUTPUT
================================================================

**Before anything else in this step, run every check in
`.claude/skills/list-screen-verify.md` and paste their real output.** This is
not optional and not satisfied by having followed the Step 3 checklist in
good faith - a checklist item can be silently skipped ("this one is a derived
column, no sort makes sense here") and read as done by whoever wrote it. That
skill file exists because it happened for real: the gateway module's first
bespoke-shell pass shipped three screens each missing sort on one or two
columns, each with a code comment ASSERTING the column could not be made
sortable - assertions that were wrong, since the fix for the exact case (a
derived count computed after paging) already existed in the same codebase.
Read that skill in full before this step, not just the summary here.

Then run the rest of the verification suite:

```
go build ./... && go vet ./... && go test ./...
gofmt -l internal/ cmd/ db/
swagger validate internal/surfaces/docs/swagger.json
cd web && npx tsc -p tsconfig.app.json --noEmit && npx ng build && npx ng test --watch=false --browsers=ChromeHeadless
```

If the catalogue changed:

```
go run ./cmd/wevetel/ authz:sync
go run ./cmd/wevetel/ authz:reconcile          # read the plan
go run ./cmd/wevetel/ authz:reconcile --apply  # commit it
```

================================================================
STEP 7: REPORT
================================================================

Report, per resource: the gap list from Step 3, what was fixed for each line,
files touched, the real verification output, and anything the contract
demands that could not be satisfied and why. Do not report success on a step
you did not actually run.

================================================================
BATCH MODE
================================================================

If more than one resource is named, or the user says "redesign all <X>
pages" (e.g. every tenant sub-page), run Steps 1-7 once PER RESOURCE,
independently - never let one resource's fix leak into another's diff. Report
one summary block per resource at the end, not one merged summary. If the
user wants a prioritised list of WHICH pages need this rather than naming
them, tell them to run `audit-pages` first.

================================================================
WHAT NOT TO DO
================================================================

- Do not skip the Step 3 gap list and jump straight to editing - the list is
  what makes the fix reviewable and stops a rewrite from quietly dropping a
  working feature.
- Do not add a column to only one of the two sort/filter whitelists.
- Do not leave `destroy` undeclared if it was already declared before this
  pass; do not leave it wired to a single soft-only `destroy` if a live
  estate is already relying on the archive/restore split existing elsewhere.
- Do not borrow the reference resource's foreign keys, counts or wording.
- Do not weaken or delete a test to make a build pass.
- Do not edit an applied migration.
- Do not commit. The user runs the git workflow separately.
