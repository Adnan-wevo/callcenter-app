---
name: modernize-helpdesk-page
description: Rewrites an existing Helpdesk central Setup resource's API and web screen onto the full-page (no drawer) variant of the current bespoke pattern - soft-destroy/restore-destroy/destroy, view/archive filter, action-button permission gating, full sort+filter parity where the list is a table, and a ROUTED FULL PAGE for the editor instead of a bottom-sheet drawer. Activates when user says "redesign helpdesk page <Name>", "modernize helpdesk setup <Name>", "upgrade helpdesk <Name> to full page".
---

You are my Page Modernizer for WevetelBastion's Helpdesk central Setup screens.

Action in /home/devmonitor/WevetelBastion

================================================================
TRIGGER
================================================================

Activates when the user says: "redesign helpdesk page <Name>", "modernize helpdesk
setup <Name>", "upgrade helpdesk <Name> to full page", "redesign <Name> tanpa drawer".
`<Name>` names one Setup screen under `web/src/app/features/helpdesk/*` (Queues, Teams,
Categories, SLA policies, Business hours, Topics, Forms, Lists, Statuses, Numbering,
Canned responses, Knowledge base, Mailboxes, Mail filters, Ban list, Customer portal,
Notification wording, Alerts, Settings).

This is `modernize-page`'s sibling, scoped to ONE difference: the editor is a routed
full page instead of `app-crud-drawer`. Everything else the two agents fix is identical.
Use `modernize-page` instead of this one for any screen outside
`web/src/app/features/helpdesk/` - the drawer is still correct there.

================================================================
STEP 1: RESOLVE THE TARGET
================================================================

Same as `modernize-page` Step 1: find the web feature under
`web/src/app/features/helpdesk/<name>*/`, its route in
`web/src/app/features/helpdesk/helpdesk.routes.ts`, its backend module under
`internal/modules/helpdesk/<name>/`, its repository, its permission module id in
`internal/domain/permission/permission.go` + `catalogue.go`.

If the name is ambiguous or matches nothing closely, list the close matches and ask.

================================================================
STEP 2: READ THE CONTRACT AND THE REFERENCE
================================================================

Read, in order:

1. `.claude/skills/crud-resource.md` - the mandatory nine-action contract and the
   three-way `soft-destroy`/`restore-destroy`/`destroy` split. Unchanged by this agent.
2. `.claude/skills/central-list-page.md` - the list shell, the All/Archive view filter,
   the single `pendingAction` confirm dialog, sort/filter whitelist parity. Unchanged.
3. `.claude/skills/helpdesk-setup-page.md` - the ONE thing this agent does differently
   from `modernize-page`: the editor is a routed full page (`setup/<name>/new`,
   `setup/<name>/:id`), not a drawer. Read its "Which Setup screens this applies to"
   section BEFORE committing to a full-page rewrite - a screen whose editor is an
   ordinary short form (Categories, Statuses, Topics, Lists, Canned responses) stays on
   `app-crud-drawer` and this agent should say so rather than force a page onto it.
4. `.claude/skills/authz-patterns.md`, `.claude/skills/api-handler-patterns.md`,
   `.claude/skills/code-style.md`.
5. `.claude/skills/list-screen-verify.md` - read now, not only at Step 6, for whichever
   part of the target stays a table.
6. The reference implementation for the full-page half:
   `web/src/app/features/helpdesk/helpdesk-queues/` (`helpdesk-queues.ts`/`.html` for
   the list, `helpdesk-queue-builder.ts`/`.html` for the full-page editor) - the first
   screen built on this pattern. For the parts that are unchanged from the ordinary
   drawer pattern (permissions, three-way destroy, the list shell itself), also read
   whichever of `modernize-page`'s four references is closest in shape
   (`web/src/app/features/roles/roles-list/` if the target confers something and wants
   a mindmap; `web/src/app/features/departments/departments-list/` if it has a
   parent/child shape; `web/src/app/features/tenants/tenants-list/` otherwise).

   Copy the SHAPE, never the target's foreign keys, joins, counts or labels - same rule
   as `modernize-page`.

================================================================
STEP 3: AUDIT - PRODUCE THE GAP LIST BEFORE EDITING ANYTHING
================================================================

Run the IDENTICAL checklist `modernize-page.md` Step 3 states (permissions and routes,
service and repository, sort/filter whitelist parity), plus these full-page-specific
lines:

- [ ] Is the editor genuinely too heavy for a drawer? Name the reason (a criteria/rule
  matrix, an ordered sub-list with add/remove/move, several independent multi-row
  sections) or stop here and hand the target to `modernize-page` instead.
- [ ] `setup/<name>/new` and `setup/<name>/:id` routes exist, guarded on `create` and
  `show` respectively, both loading one shared builder component
- [ ] The list's row click and mobile card STILL open the read-only Show drawer, exactly
  as `central-list-page.md` requires on every other central screen - this is the
  single most common way this pattern has shipped wrong. There is NO table "Actions"
  column (`grep -c ">Actions<"` on the list's `.html` must be 0) and no bare icon button
  sitting in a table cell with no `toggleSort` beside it. Create navigates to `.../new`
  from the header's action row; Edit navigates to `.../:id` from a button INSIDE the Show
  drawer, gated on `is_mine`/ownership where the resource has an owner and hidden
  entirely for a row that cannot be edited - never from the table.
- [ ] Every custom action the resource has (Share, Reorder, Export, Test-connection,
  whatever) and the three-way Archive/Restore/permanent-Delete all live in the Show
  drawer's action row, matching `vpn-clients-list.html`'s reference shape - not as icons
  in the table.
- [ ] The builder loads a record via the same `Show` call the drawer uses
  (`this.svc.get(id)`) - a full-page deep link has no list row in memory to fall back on.
  A bare deep link onto an ARCHIVED record (the one path the UI itself never offers,
  since the drawer shows no Edit button for one) renders a plain "not found, restore it
  first" state rather than a 404 or a silently-editable form.
- [ ] A `CanDeactivateFn` guards navigating away from a dirty, unsaved builder, raising
  the shared `app-confirm-dialog` rather than discarding silently or not at all

Print the gap list before touching a file, exactly as `modernize-page` does.

================================================================
STEP 4: FIX THE BACKEND
================================================================

Identical to `modernize-page.md` Step 4. The full-page editor changes nothing about the
API: same nine permissions, same routes, same service/repository methods, same
sort/filter whitelist entries on both layers, same test updates.

================================================================
STEP 5: FIX THE WEB SCREEN
================================================================

Follow `.claude/skills/helpdesk-setup-page.md` end to end:

1. Split the existing single component into a list component (its own file, `app-crud-
   drawer`/form-drawer stripped out, `app-show-drawer` KEPT and untouched) and a new
   `<name>-builder.ts`/`.html` carrying the form, its sub-forms and the load/save/discard
   logic that used to live in the form drawer.
2. Add the `new`/`:id` routes to `helpdesk.routes.ts` beside the existing list route.
3. Wire the dirty-navigation guard, and point the header's New button and the Show
   drawer's Edit button at the new routes.
4. Leave everything else exactly where `central-list-page.md` puts them: the row
   click/mobile card still open the Show drawer; Share, Reorder, Export, Archive,
   Restore, permanent Delete and any other custom action live in that drawer's action
   row (never as a table icon); the All/Archive toggle and the single confirm dialog are
   unchanged. Only Create and Edit's DESTINATION moved, from `app-crud-drawer` to a route.

================================================================
STEP 6: VERIFY - RUN THESE AND PASTE REAL OUTPUT
================================================================

Identical to `modernize-page.md` Step 6: `list-screen-verify.md`'s mechanical checks
first, then

```
go build ./... && go vet ./... && go test ./...
gofmt -l internal/ cmd/ db/
swagger validate internal/surfaces/docs/swagger.json
cd web && npx tsc -p tsconfig.app.json --noEmit && npx ng build && npx ng test --watch=false --browsers=ChromeHeadless
```

and, if the catalogue changed, `authz:sync` then `authz:reconcile`/`--apply`.

Additionally click through the new routes once the dev server (or a build) is up: open
the list, open Create, open an existing record's Edit, confirm Cancel and the back
control return to the list, confirm navigating away from a dirty form prompts, confirm
an archived record renders read-only.

================================================================
STEP 7: REPORT
================================================================

Same shape as `modernize-page.md` Step 7, plus the "genuinely too heavy for a drawer"
judgment from Step 3 stated explicitly.

================================================================
WHAT NOT TO DO
================================================================

- Do not apply the full-page editor to a Setup screen whose form is ordinary and short -
  hand it to `modernize-page` instead and say so.
- Do not force the list into a paginated table if the resource's own domain comment
  already commits it to an unpaginated, manually-ordered list with no column filters
  (Queues, Mail filters, Routing rules) - this agent changes the editor, not the list.
- Do not skip the Step 3 gap list.
- Do not leave an archived record's full page silently accepting edits the server will
  refuse - render it read-only.
- Do not commit. The user runs the git workflow separately.
