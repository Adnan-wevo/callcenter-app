---
name: helpdesk-setup-page
description: The full-page (no drawer) variant of the bespoke list pattern, for Helpdesk central Setup screens whose editor holds more than a bottom sheet can hold - a criteria matrix, an ordered column layout, a routing table, a form builder. Queues (web/src/app/features/helpdesk/helpdesk-queues/) is the reference implementation. Read before building or redesigning a Helpdesk Setup screen.
---

# The Helpdesk Setup full-page pattern

This is `central-list-page.md`'s pattern with ONE piece swapped: CREATE and EDIT move
from a drawer to a routed full page. Nothing else changes, and getting this exactly right
matters more than it looks: it shipped wrong once (an "Actions" table column of icon
buttons, no row click, no Show drawer at all) before landing here, and the fix cost more
than building it correctly the first time would have.

**VIEW stays a drawer bottom sheet, on every screen this pattern touches, with no
exception.** Every central list screen in this codebase - all 29 of them, checked - wires
its desktop `<tr>` and mobile card to `(click)="openShow(row, ...)"`, and NONE of them has
a table column of action icons: `<th>Actions</th>` does not exist anywhere in this
codebase, and a table column of buttons could not carry `toggleSort` in any case, which
would break the mandatory `th`/`toggleSort` 1:1 parity `list-screen-verify.md` checks
mechanically. Archive/Restore/permanent-Delete and every CUSTOM action (Share, Reorder,
Export, Test-connection, Verify-DNS, whatever the resource has) live INSIDE that drawer's
action row (see `vpn-clients-list.html`'s Test-connection button for the reference shape),
gated per permission and per ownership there - never as an icon in the table, full-page
editor or not. `helpdesk-queue-builder.ts` and its route are the ONLY thing this file adds
on top of `central-list-page.md`: a heavier Edit/Create surface, reached from a button
INSIDE the drawer (or the header's New-queue action), not a replacement for the drawer
itself. Everything else - the list shell, the three-way `soft-destroy`/`restore-destroy`/
`destroy` split, the single `pendingAction` confirm dialog, sort/filter whitelist parity,
permission gating - is unchanged and this file does not repeat it. Read
`central-list-page.md` and `crud-resource.md` first; this file only states the one real
difference and why it exists.

## Why a drawer is the wrong shell here

`app-crud-drawer` is sized by `DRAWER_LARGE_BOTTOM_SHEET`: a fixed height, capped, with a
70%-width centred sheet on desktop. That size was chosen for an ordinary form - a dozen
fields, maybe one nested list. A Helpdesk Setup editor routinely holds more than that in
one screen: a queue's builder is an identity form PLUS a criteria matrix (one row per
filter, a kind-dependent control per row) PLUS an ordered column layout (add/remove/move,
per-column truncate mode and conditional format) PLUS three inheritance flags whose effect
depends on a parent picker drawn from the same list. Stacking that inside a capped sheet
either scrolls a sheet inside a sheet or shrinks every control to fit, and both read as
"this screen is too small for what it is asking of the operator." A form builder's field
list, a mail filter's rule chain, and an SLA policy's per-priority matrix are the same
shape: several independent multi-row sections that all want the full viewport at once.

So: **Helpdesk Setup's editor is a routed full page, not a drawer.** The list stays a
list, on the same viewport, reached the same way (click a row); only the thing that opens
changes, from an overlay to a navigation.

## Which Setup screens this applies to

Every screen under `setup/*` in `internal/../helpdesk.routes.ts` gets the full-page
editor **when its own content genuinely does not fit a drawer**: Queues (criteria +
layout + inheritance), Forms (a field list per form), SLA policies and Routing rules (a
per-priority or per-condition matrix), Mail filters (an ordered rule chain), Notification
templates (subject + body + variable reference side by side). A screen whose editor is an
ordinary short form - Categories, Statuses, Topics, Lists, Canned responses - stays on
`app-crud-drawer` exactly as `central-list-page.md` describes: this pattern is not "every
Helpdesk Setup screen becomes a full page," it is "an editor too heavy for a sheet becomes
a page." Judge it the way `central-list-page.md` judges `app-data-table` (its own
"genuinely nothing to redesign into" exception): name the reason explicitly rather than
defaulting to a full page because the resource happens to live under `setup/`.

## The list stays whatever shape the resource actually has

Most Setup screens are an ordinary paginated table (Categories, Mailboxes, and - since
the 2026-08-13 pass - Queues: it briefly shipped as an unpaged, manually-ordered sidebar
with no column filters, on the reasoning that the order WAS the point, and that reasoning
did not survive contact with the standing rule that every central list screen matches the
others. It moved onto `Page`/`PageArchived` with the full sort/search/filter whitelist,
same as any other resource; the manual order survived as `position`, the DEFAULT sort and
a Move up/down action offered only while sorted by it - see `helpdesk-queues.ts`'s own doc
comment for the exact shape). A genuinely ordered POLICY list (Mail filters, Routing
rules) is different in kind: first-match-wins makes the order the RULE itself, so a
sortable column header would let a click change the policy, not just the view of it - see
`support/support-routing/support-routing.ts`. **Do not force a paginated table onto a
screen whose order IS the policy it enforces; do not use "the order is nice to keep" to
excuse a resource, like Queues, whose order is only ever a display convenience.** This
file changes the editor, not the list; keep whichever list shape `central-list-page.md`'s
table rule or the resource's own domain comment genuinely commits it to.

## Route shape

Two or three sibling routes replace the one the drawer used to sit inside, added to the
same `helpdesk.routes.ts` array queues already uses as its example:

```ts
{
  path: 'setup/queues',
  canActivate: [requirePermission('helpdesk-queues.index')],
  loadComponent: () => import('./helpdesk-queues/helpdesk-queues').then((m) => m.HelpdeskQueuesComponent),
},
{
  path: 'setup/queues/new',
  canActivate: [requirePermission('helpdesk-queues.create')],
  loadComponent: () =>
    import('./helpdesk-queues/helpdesk-queue-builder').then((m) => m.HelpdeskQueueBuilderComponent),
},
{
  path: 'setup/queues/:id',
  canActivate: [requirePermission('helpdesk-queues.show')],
  loadComponent: () =>
    import('./helpdesk-queues/helpdesk-queue-builder').then((m) => m.HelpdeskQueueBuilderComponent),
},
```

`new` and `:id` load the SAME component, exactly as the drawer used one form for both -
the difference between them is whether a row was passed in, not a different screen.
`new` is guarded on `create`, `:id` on `show`; the builder itself still checks `update`
before it lets a save through, same as the drawer's `submit()` always did.

## The builder component: what moves out of the drawer and what does not

- **`app-page-header`** still opens it, with a back control (`routerLink="../"` or
  `location.back()`) where the drawer had a Close button, and the title is "New <thing>"
  or the record's own name/reference rather than a fixed "Edit" - the record IS the page
  now, not a panel over a list the operator can still see behind it.
- **Save / Cancel become a header action row**, `sticky top-0` so a long page still has
  them in view, rather than a footer pinned to a sheet that no longer exists. Cancel
  navigates back; it does not close anything.
- **Content is `hlmCard` sections, not drawer tabs.** A drawer tabs unrelated concerns to
  fit one viewport; a full page has no such pressure, so each concern (identity, the
  criteria matrix, the layout, the inheritance flags) gets its own card, stacked, and the
  operator scrolls instead of clicking between tabs that hide what is not showing.
- **Every multi-row sub-form** (the criteria `FormArray`, the columns `FormArray`) is
  unchanged from the drawer version - same add/remove/move methods, same per-row control
  choice keyed on the column's kind. None of that logic is about the shell; it moves file
  unmodified.
- **Loading the record for edit** reads the route's `:id` and calls the same `Show`
  method the drawer would (`this.svc.get(id)`), because a full-page reload (a bookmarked
  URL, a browser refresh) has no list row sitting in memory to fall back on. This is a
  smaller gap than it looks: the Edit route is reached from an "Edit" button INSIDE the
  Show drawer's action row (see below), never from a table row directly, and the drawer
  never offers that button for an archived record in the first place - Restore is what it
  offers instead. A bare `/setup/<name>/<id>` deep link onto an archived record is
  therefore reached only by a stale bookmark or a hand-typed URL, and `Show` 404s on it by
  the estate's standing rule (see `crud-resource.md`); render that as the plain "not
  found, try restoring it first" state Queues' builder does, rather than building a
  read-only mode for a path the UI itself never offers.
- **Unsaved-changes discard is now a navigation, not a close.** Wrap the deactivate path
  in an Angular `CanDeactivateFn` (or the router's `canDeactivate` guard array) that
  checks `form.dirty` and raises the SAME `app-confirm-dialog` component the list already
  imports for its lifecycle actions, rather than a second bespoke dialog: "Leave without
  saving? Changes on this page have not been saved." A drawer's `resetForm` on close made
  this moot by discarding silently into a control that was about to vanish anyway; a page
  navigation needs the operator told, because leaving reads as intentional.

## Every action lives in the Show drawer, exactly as central-list-page.md describes

The view/archive toggle, the Show drawer, one `pendingAction` signal and one
`app-confirm-dialog` for Archive/Restore/permanent-Delete - all of it stays exactly as
`central-list-page.md` describes. Share, Reorder, Export and every other custom action
live in the drawer's action row beside the lifecycle buttons (see
`vpn-clients-list.html`'s Test-connection button), gated per permission and per ownership
there, NEVER as an icon in the table - a table column of buttons has no `toggleSort` of
its own, which breaks the mandatory `th`/`toggleSort` parity every screen in this codebase
is checked against. The ONE thing that changed is where Create and Edit LAND: the header's
New button and the drawer's Edit button both navigate to the full page instead of opening
`app-crud-drawer`. The drawer that opens on a row click is still, and always, the read-only
Show - see `helpdesk-queues.ts`'s `openShow`/`showItems` for the reference shape.

## Applying this to a screen

1. Read `crud-resource.md`'s nine-action contract and three-way destroy split, and
   `central-list-page.md`'s list/table/confirm-dialog conventions - unchanged here.
2. Judge whether the editor is genuinely too heavy for a drawer (see "Which Setup
   screens this applies to" above); state the reason either way.
3. Split the existing single component into a list component (keep its file, strip the
   drawer and the form out of it) and a new `<resource>-builder.ts`/`.html` component
   carrying the form, its sub-forms, and the load/save/discard logic the drawer used to
   own.
4. Add the `new` and `:id` sibling routes beside the list route in `helpdesk.routes.ts`,
   guarded on `create` and `show` respectively.
5. Wire the `CanDeactivateFn` dirty-check, the archived-read-only rendering, and the
   list's row click to navigate (`routerLink`) instead of opening a drawer.
6. Run the same verification `modernize-page.md` Step 6 runs (`ng build`, `ng test`,
   `list-screen-verify.md`'s checks against whatever stayed a table) - a full page is
   still a route, and `tsc`/`ng build` catch the same class of mistake a drawer would.
