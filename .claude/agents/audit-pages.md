---
name: audit-pages
description: Read-only survey of every list screen against the current contract (three-way soft-destroy/restore-destroy/destroy, sort/filter whitelist parity, view/archive filter, action-button gating). Produces a prioritised gap table, most-broken first, with no edits made. Activates when user says "audit pages", "cek page mana perlu redesign", "list pages that need modernize", "scan old design pages".
---

You are my Page Auditor for WevetelBastion.

Action in /home/devmonitor/WevetelBastion

READ-ONLY. This agent never edits a file. It exists so the user does not have
to open every screen by hand to find out which ones are behind the current
contract before deciding what to hand to `modernize-page`.

================================================================
TRIGGER
================================================================

Activates when the user says: "audit pages", "cek page mana perlu redesign",
"list pages that need modernize", "scan old design pages", "which pages are
outdated".

================================================================
STEP 1: READ THE CONTRACT
================================================================

Read `.claude/skills/crud-resource.md` and `.claude/skills/central-list-page.md`
first - the checklist below is derived from both and assumes their wording.

================================================================
STEP 2: ENUMERATE SCREENS
================================================================

```
find web/src/app/features -iname "*-list.ts" -not -name "*.spec.ts" | sort
```

For each, resolve its permission module id (the `<module>` prefix of the
permissions it checks - grep the component/service for `.hasPermission(` or
`requirePermission(` calls) and its backend module under
`internal/modules/<capability>/<module>/`.

================================================================
STEP 3: SCORE EACH SCREEN
================================================================

Run these greps per resource - fast, no full file reads needed for a first
pass; only open a file if a grep result is ambiguous.

**Backend three-way destroy**
```
grep -n "<module>\.\(soft-destroy\|restore-destroy\|destroy\)" internal/domain/permission/permission.go
grep -n "PageArchived\|HardDelete\|SoftDestroy\|Restore" internal/modules/<capability>/<module>/service.go internal/platform/postgres/<module>_repo.go
```
Score: **COMPLETE** (all three identifiers + methods present), **PARTIAL**
(soft-destroy/restore present, destroy declared but unwired or missing),
**OLD** (a single `destroy` identifier only, no soft/restore split at all).

**Sort/filter whitelist parity**
```
grep -n "SortColumns\|FilterSpec" internal/modules/<capability>/<module>/service.go
grep -n "OrderBy\|FilterBy\|alias=" internal/platform/postgres/<module>_repo.go
```
Diff the column names in both lists by hand. Flag any column present in only
one side - this is a SILENT bug (falls back to default sort/no filter, no
error), so it does not show up by using the screen, only by reading both
whitelists side by side.

**Frontend pattern**
```
grep -n "ViewFilter\|pendingAction\|toggleSort\|sortIcon\|DRAWER_LARGE_BOTTOM_SHEET\|disableClose" web/src/app/features/<feature>/<name>-list/<name>-list.ts
grep -n "app-data-table" web/src/app/features/<feature>/<name>-list/<name>-list.html
```
Score: **BESPOKE-COMPLETE** (all markers present, hand-rolled shell),
**NEEDS-MIGRATION** (still on `app-data-table` - as of the current standard in
`central-list-page.md` this is the LEGACY shell, not a fine default; every
resource with an ordinary CRUD lifecycle is due a shell rebuild onto the
bespoke pattern, not just a lifecycle bolt-on), **BESPOKE-PARTIAL** (hand-rolled
table+drawer missing one or more markers above). Before scoring a
`DATA-TABLE` resource as needing migration, check whether it is one of the
genuinely lifecycle-less exceptions (a pure append-only trail, live ephemeral
state with no soft-delete concept) - those are correctly on `app-data-table`
and should be scored **DATA-TABLE-OK** with a one-line reason, not flagged.

**Table completeness**
For a bespoke screen, count columns rendered in the desktop `<table>` versus
columns with a `toggleSort` + filter control. Flag any column present in the
table but missing either.

================================================================
STEP 4: REPORT
================================================================

One table, most-broken first (OLD before PARTIAL before COMPLETE, then
BESPOKE-PARTIAL before NEEDS-MIGRATION before BESPOKE-COMPLETE within each
backend tier; DATA-TABLE-OK sits with BESPOKE-COMPLETE since neither needs
work):

| Resource | Backend | Frontend | Sort/filter parity | Notes |
|---|---|---|---|---|

Below the table, one line per resource naming exactly what is missing (so the
user can hand any row straight to `modernize-page` as its target list without
re-deriving the gap). End with a suggested batch: which resources are closest
in shape to each other (worth running through `modernize-page` in one batch
because they share a reference implementation) versus which need individual
attention.

Do not edit anything. Do not run `go build`/`ng build` - this is a static
grep-based survey, not a verification pass.
