---
name: central-list-page
description: The bespoke table+drawer pattern for a central console list screen (Access Control -> Users is the reference implementation) - when to reach for it instead of app-data-table, and the shared components, inputs and gotchas that make it consistent. Read before building or reviewing a hand-built list screen.
---

# The bespoke list-page pattern

**This is the STANDARD shape for a resource's list screen, not a fallback for when a
screen "outgrows" `app-data-table`.** That was the earlier rule. It changed once enough
resources were rebuilt on this pattern end to end that the standing expectation became
every CRUD screen looks and behaves like these: restore/soft-destroy, per-column sort
and filter that actually work, an All/Archive view, and action buttons that show or
hide by permission rather than being present and 404ing. `app-data-table`
(`web/src/app/shared/components/data-table/`) is now the LEGACY shell - a screen still
built on it has not been migrated yet, not a screen correctly left alone.

**Do not build a NEW screen on `app-data-table`, and do not bolt the archive/restore/
destroy lifecycle onto an EXISTING `app-data-table` screen instead of rebuilding its
shell onto this pattern.** That half-migration - lifecycle wired in, shell untouched -
is exactly what shipped on the gateway module's first redesign pass, and it read to the
person who asked for a redesign as no redesign at all, because visually nothing had
changed. `modernize-page`'s STEP 2 carries the same note for the same reason.

The one exception is a resource with genuinely nothing to redesign into: a pure
append-only trail with no write actions at all (an audit log) or live ephemeral state
with no soft-delete concept (a revocable session row) has no Show/Edit/Archive
lifecycle to give a drawer, so `app-data-table` remains acceptable there - there is no
lifecycle to gain by rebuilding the shell around nothing. Everything else - every
ordinary resource, and every existing `app-data-table` screen due a redesign - targets
this pattern.

This pattern was built first on Access Control -> Users
(`web/src/app/features/users/users-list/`) and then applied, more completely, to
Access Control -> Roles (`web/src/app/features/roles/roles-list/`) - **Roles is the
file to copy from**: it is the only one with the full archive/restore/permanent-delete
three-way split (see `crud-resource`'s mandatory nine-action contract) and the mindmap
detail visualisation both wired end to end. Copy from the file, not from memory.

**As of the 2026-08-12 pass, three more screens carry the pattern complete end to
end and are equally valid to copy from**: Tenant Registry
(`web/src/app/features/tenants/tenants-list/`) and Organisation Control's
Departments and Groups (`web/src/app/features/departments/departments-list/`,
`web/src/app/features/groups/groups-list/`). All three carry the view/archive
filter, the sort/filter whitelist parity, and the single `pendingAction`
confirm dialog described below, verified working. When auditing or rebuilding a
screen against this pattern, cross-check it against at least one of these four
(Roles, Tenants, Departments, Groups) rather than against memory of the pattern.

Building this by hand costs more than `app-data-table` in lines of code. The payoff is a
screen that can shape its own detail view instead of being one, and, now that it is the
standard, consistency: an operator moving between screens meets the same table, the
same drawer and the same archive/restore/destroy shape everywhere, not on some screens
and not others.

## The shared components, and what stays page-specific

| Piece | Component | Page-specific? |
|---|---|---|
| Heading + description + action row | `app-page-header` (`shared/components/page-header/`) | No - title/description/projected buttons only |
| Any drawer or bottom sheet | `app-crud-drawer` (form) / `app-show-drawer` (read-only) | No - see below |
| Rows-per-page values | `DT_PAGE_LENGTHS` from `shared/components/data-table/data-table.ts` | No - reuse the constant, the `<select>` markup itself is fine inline |
| Large-desktop-sheet sizing | `DRAWER_LARGE_BOTTOM_SHEET` from `shared/components/crud-drawer/crud-drawer.ts` | No - see below |
| Destructive/lifecycle confirmation | `app-confirm-dialog` (`shared/components/confirm-dialog/`) | No |
| Field error rendering | `app-field-label` + `appFormSubmitted` (`shared/utils/form-helpers` or similar) | No |
| Tree / hierarchy visualisation | `app-mindmap` (`shared/components/mindmap/mindmap.ts`) | No - see below |
| Table columns, filter fields, tab content, sort columns | - | Yes, entirely - this is the screen's own shape |

If you find yourself about to hand-roll a SECOND copy of the drawer frame, the
page-size select's option list, or the "large centred bottom sheet" class string, stop -
one of the rows above already exists. Copying it is the bug this table exists to prevent.

## Every drawer is a bottom sheet, on every viewport, always

This was NOT the first design. It went through a right-side drawer, then a centred
modal-on-desktop / bottom-sheet-on-mobile split, before landing here. Both earlier
shapes are dead ends worth naming so nobody re-walks them:

- **A separate desktop modal component** (a large centred `hlm-dialog`) duplicates the
  whole frame `app-crud-drawer`/`app-show-drawer` already provide, and it means every
  frame change (a new button, a spacing tweak) has to be made twice and kept in step.
  It was deleted for exactly this reason.
- **Switching direction by breakpoint** (`direction="right"` on desktop, `"bottom"` on
  mobile) means every screen decides its OWN breakpoint logic and re-derives its own
  sizing per direction. One direction, sized differently per breakpoint, is simpler and
  is what this pattern does.

So: `direction="bottom"` unconditionally, and the desktop/mobile difference is ONLY in
size, carried by `sizeClass`:

```html
<app-crud-drawer
  [open]="drawerOpen()"
  direction="bottom"
  [sizeClass]="isMobile() ? '' : largeSheetClass"
  [disableClose]="true"
  ...
>
```

```ts
import { CrudDrawerComponent, DRAWER_LARGE_BOTTOM_SHEET } from '.../crud-drawer/crud-drawer';

protected readonly largeSheetClass = DRAWER_LARGE_BOTTOM_SHEET;
```

`''` on mobile keeps Helm's own content-height sheet (short forms stay short). The
desktop value is a fixed height (not a ceiling - see the constant's own doc comment for
why a `max-h-*` alone still leaves tabs opening to different heights) plus a centred 70%
width. Do not hand-derive this string per screen: getting to it cost three separate,
each-invisible-until-clicked bugs (Helm's own rule needing `!important` to lose,
`w-full` over-constraining an `inset-x` centring attempt, `max-h` being a ceiling and
not a size), and `DRAWER_LARGE_BOTTOM_SHEET`'s doc comment is where that reasoning lives
so the next screen does not rediscover it by trial and error.

## `disableClose`

Set `[disableClose]="true"` on every drawer here. The CDK backdrop click bypasses
`closeOnOutsidePointerEvents` entirely (it dismisses on a DIFFERENT reason, `'backdrop'`,
which that flag never gates) - `disableClose` is the one input that actually blocks
outside-click, backdrop-click AND Escape at once. Without it, a tap meant for the list
behind an in-progress form silently discards the form. The X / Cancel / Close button
still works regardless, because it calls `close()` directly rather than going through
the dismiss gate.

## The page-header action row: collapse it behind a toggle, not a growing row

```html
<app-page-header [hasActions]="canDoX() || canDoY()">
  <button hlmBtn variant="outline" type="button" (click)="toggleActionsMenu()">
    <ng-icon name="lucideEllipsisVertical" class="mr-2 text-base" />
    Actions
  </button>
</app-page-header>

@if (actionsMenuOpen()) {
  <div class="mt-3 flex shrink-0 flex-wrap items-center gap-2 rounded-md border p-3">
    <!-- the real, individually-*hasPermission-gated buttons -->
  </div>
}
```

**Every page-level action goes behind this toggle, including a single one.** This was
"a page with one or two actions does not need this" until it shipped inconsistently: SSL
Deployments put a bare `Refresh` button directly in the header instead of behind the
toggle, and Remote Sessions did the same with `Start session` - both one-action screens,
both visibly different from every screen that followed the toggle, and both read as
"why does this page look unfinished" rather than as a deliberate simpler case. One
action still gets the `Actions` button and the collapsed panel underneath it, exactly
like a screen with three - the ONLY thing that varies is how many buttons are inside the
panel when it opens. A screen with zero page-level actions (a pure read-only log with no
create, no refresh, nothing) renders no `Actions` button at all, per `[hasActions]`
already being conditional - see gateway-audit and the central Audit screens.

## The inline filter panel

Search box + a "Filters" toggle button that reveals a panel below the toolbar (never a
drawer - a drawer for filters competes with the record drawer for the same gesture).
Every chip/input inside applies IMMEDIATELY on change; there is no separate Apply step,
because there is no drawer to close over it. Show the applied filters as removable tags
below the panel so a caller can see and undo one without reopening the panel. This part
is inherently page-specific (the fields differ per screen) - copy the STRUCTURE, not the
field list.

## Desktop table / mobile card list, zero shared markup

Two structurally different renderings of the same `rows()` signal - a `<table>` behind
`hidden md:flex`, a card list behind `md:hidden`. Do not try to make one flex layout
serve both; a table's column alignment and a card's stacked layout want different DOM
shapes, and forcing one to become the other is where the CSS gets unreadable. Row
numbering, sortable headers (button + a `sortIcon()` helper returning one of
`lucideArrowUp`/`lucideArrowDown`/`lucideArrowUpDown`) and click-to-open-detail live on
the desktop table; the mobile list gets the same click-to-open plus a "Load more" button
instead of Previous/Next.

## Every column is sortable - no exceptions, and both layers or it silently does nothing

See `.claude/skills/list-screen-verify.md` for the mechanical, command-driven
version of this rule - the one to actually RUN before reporting a screen
done, rather than the prose explanation below.

**MANDATORY for every resource, on this pattern exactly as much as on `app-data-table`
- see `crud-resource`'s equivalent table rule.** If a column is rendered in the desktop
table, it has a `toggleSort('column_key')` header button and a `sortIcon('column_key')`
- a derived COUNT column (member count, permission count, a parent's name pulled through a
join) is not an exception, and neither is "the backend does not support it yet": add the
correlated-subquery sort alias to the repository (`alias=(SELECT ... )`, see
`departmentMemberCountExpr`/`groupDepartmentNameExpr` in `internal/platform/postgres/` for
the shape) rather than leaving the column unsortable. A header with no sort affordance
sitting next to headers that have one reads as broken, the same defect
`crud-resource`'s table rule calls out for a missing filter.

**Verify this mechanically, do not trust that the checklist was followed.** Count `<th `
tags against `toggleSort(` calls inside the table's `<thead>` - they must match except for
one (the `#` row-number column, which correctly carries no sort):

```
th=$(awk '/<thead/,/<\/thead>/' path/to/list.html | grep -c "<th ")
sort=$(awk '/<thead/,/<\/thead>/' path/to/list.html | grep -c "toggleSort(")
```

A prose claim that a column "can't be sorted" is not a reason to skip this count - it is
the exact failure mode this note exists to catch. The gateway module's first pass on this
pattern shipped three screens each missing sort on one or two columns, each with a code
comment ASSERTING no sort was possible (an assembled multi-field display, a derived count
computed after paging), when the fix for both cases already existed in the same codebase
(`upstream_host` sorting on its own stored column beneath the assembled display; the
correlated-subquery alias below for the derived count). A prose checklist did not catch it
because nothing forced the claim against a working counter-example - the `th`/`toggleSort`
count does.

**The whitelist is stated on TWO layers and both must carry every column, or the column is
sortable in neither.** `application.ListQuery.Normalise` checks the caller's `?sort=` against
the SERVICE-layer plain-string list before it ever reaches the repository; the
REPOSITORY-layer list (the one with the `alias=EXPR` entries) is what `PageQuery.OrderBy`
resolves into actual SQL. Adding the correlated subquery to the repository alone is not
enough - the request is refused one layer up, silently normalised back to the default sort,
before the expression is ever consulted. This is exactly the bug that shipped on
Departments/Groups: `permission_count` was added to the repository's sort AND filter
whitelist, wired into the frontend's Confers filter box, and the SERVICE-layer whitelist was
never updated to match - so every request naming it was quietly dropped to the default order
and the confers filter matched nothing, with no error anywhere to notice by. Grep both
`*SortColumns` (or `*FilterColumns`) declarations for the module - one in
`internal/modules/<capability>/<resource>/service.go`, one in
`internal/platform/postgres/<resource>_repo.go` - and confirm the column name appears in
both before calling a column "sortable".

## Lifecycle actions go through ONE confirm dialog

Every destructive or state-changing action (deactivate, archive, restore, ...) sets one
`pendingAction` signal (`{ row, kind }`) rather than each wiring its own dialog. One
`app-confirm-dialog` at the bottom of the template reads title/message/danger/confirm
label off `pendingAction()` via computed signals keyed on `kind`, and one
`confirmPendingAction()` method holds a `Record<Kind, Observable<void>>` map of requests
and fires the right one. This is what stops "the fourth action shipped without its
confirm dialog" from happening again - there is only one dialog to wire, not one per
action.

**The failure branch must notify.** A silent `error: () => { this.reload(); }` with no
`this.notify.error(...)` was a real, shipped bug here: a backend refusal (permission
denied, a business rule like "a super-admin account may never be archived") produced NO
feedback at all, and the row just... didn't change, with no explanation. Extract the
server's message with the shared `errorText(err)` helper pattern (reads
`err.error.message` off an `HttpErrorResponse`, falls back to a generic string) and pass
it to `this.notify.error(...)` in every failure handler, not just the success one.

## The All / Archive view filter, and the three-way destroy split

`crud-resource` makes this MANDATORY for every resource, not just the ones using this
bespoke pattern - read that skill's "three-way destroy split" section for the backend
half (`soft-destroy`/`restore-destroy`/`destroy`, the permanent one refusing a LIVE
row). This is the frontend half, and Roles (`roles-list.ts`/`.html`) is the complete
reference.

- **A `view: 'all' | 'archive'` filter is one more entry in `FilterState`**, rendered
  as a chip pair in the same inline filter panel every other filter uses - not a
  separate tab, not a separate route. `buildParams()`/`refetch()` branch on it to call
  either the ordinary paged read or the archived one (`authz.archivedRolesPage(...)` /
  `authz.rolesPage(...)`), and `openShow(row, archived)` takes a second argument so the
  detail drawer knows which population the row it was opened from belongs to.
- **An archived row cannot be re-read through the ordinary Show endpoint** - it
  soft-deleted, so the default GORM scope answers 404 for it the same way every other
  read does. `openShow` for an archived row skips the re-read and renders straight from
  the list row instead, coerced into whatever shape the detail view's type expects with
  the fields only Show would have supplied left empty (see `showingArchived` and the
  `attached_ids: [], attached_names: [], catalogue: []` coercion in `roles-list.ts`).
  The View tab must say plainly that this data is unavailable for an archived row
  rather than silently rendering an empty state that reads as "confers nothing."
- **`pendingAction` grows a third `kind`.** What was `'archive' | 'restore'` becomes
  `'archive' | 'restore' | 'destroy'` - still ONE signal, ONE `app-confirm-dialog`, one
  `confirmPendingAction()` with a `Record<Kind, Observable<void>>` map. The destroy
  confirmation message must say IRREVERSIBLE and name exactly what is lost (the row,
  its attachments, its former holders) - a generic "are you sure?" undersells a
  permanent delete next to an archive that undoes in one click.
- **Edit and any matrix/manage tab are hidden entirely for an archived row**, not just
  disabled - there is nothing valid to edit or sync until it is restored, and a visible
  but broken control invites exactly the confusion `crud-resource`'s gating section
  warns about.

## The mindmap component: a tree, not a wall of identifiers

`app-mindmap` (`shared/components/mindmap/mindmap.ts`) replaces a flat, alphabetised
list of related identifiers - "what does this role confer" is the case it was built
for - with a small interactive tree. Reach for it wherever a plain list would bury a
genuine PARENT/CHILD shape (a role and its permissions grouped by module; anything
else with the same "many children, several natural groupings" shape) in a wall of
text a reader has to scan line by line to find structure in.

- **It is a generic `MindmapNode[]` tree, not fixed to three levels.** A node with
  `children` is expandable; one without is a leaf. Build the tree with as many levels
  as the DATA actually has - see `RolesListComponent.confersTree`, which groups
  permission identifiers by module and then folds sibling modules sharing a hyphenated
  family (`helpdesk-teams`, `helpdesk-drafts`, `helpdesk-queues`, …) under one
  `helpdesk-*` group node first, so a role touching a dozen `helpdesk-*` modules reads
  as one family instead of a dozen branches fanned equally off the root. A module with
  no siblings sharing its prefix skips the group level entirely - never insert an
  empty wrapper node.
- **It opens COLLAPSED and grows one click at a time**, left to right (root on the
  left; each level's children appear to its right). Everything-open-at-once is the
  same "too much to read at a glance" problem the tree replaced a list to solve in the
  first place - see the component's own doc comment for the fuller reasoning, including
  why left-to-right over a radial (rings-around-a-centre) layout: a leaf belongs to
  exactly ONE branch, and hanging it directly off that branch says so, where a ring
  says every leaf is equally related to the centre.
- **Labels are truncated to fit their box (`fitLabel`), never wrapped or left to
  overflow** - SVG text does not clip to a `<rect>` on its own. The FULL text always
  goes in the SVG `<title>` (hover/focus tooltip); never truncate the tooltip too.

## Verify sizing changes against the compiled CSS, not just the build exit code

`ng build` succeeding proves the TEMPLATE is valid, not that a Tailwind arbitrary-value
class actually compiled the way you expect, or that it beats what it is trying to
override. When adding or changing a `!important` override, grep the ACTUAL emitted rule
out of `dist/web/browser/styles-*.css` before calling it done:

```bash
grep -o "[^;{]*<value>[^;}]*" dist/web/browser/styles-*.css
```

Two things this catches that a passing build does not: a class that silently compiled
to nothing (a malformed arbitrary-value syntax Tailwind dropped rather than errored on),
and a class that compiled but WITHOUT the `!important` you meant to add - which reads
identically at build time and only shows up as "I fixed it and nothing changed" once a
human clicks around after deploy.
