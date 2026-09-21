---
name: list-screen-verify
description: The mechanical, command-driven checks that MUST run and show real output before any agent reports a list screen (new or redesigned) complete. Read this at the end of create-crud, modernize-page, or any ad hoc list-screen work, before writing the completion report.
---

# List-screen completion is verified, not asserted

A checklist item can be silently skipped by whoever is filling it in ("this
column can't be sorted, moving on") and still read as done. This is not
hypothetical: the gateway module's first pass onto the bespoke pattern
(`central-list-page.md`) shipped three screens each missing sort on one or
two columns - `upstream_host` on gateway-apps, `dns_matches`/`app_count` on
gateway-domains, `client_id`/`app_count` on gateway-identity - each with a
code comment ASSERTING the column structurally could not be sorted, when the
fix for the exact case (a derived count computed after paging) already
existed in the same codebase (`departmentMemberCountExpr` in
`internal/platform/postgres/department_repo.go`, a correlated-subquery sort
alias). The checklist in `crud-resource.md` already said "every column
sortable, no exceptions" - it did not stop the mistake, because nothing
forced the claim against a counter-example before the work was reported
finished.

**These checks exist to replace "I followed the checklist" with a number a
human can read.** Run every one that applies to the screen you touched, and
paste the REAL output in your completion report - not a summary of having
run them.

## 1. Sort completeness: `<th>` count vs `toggleSort(` count

For every list screen using the bespoke pattern (`central-list-page.md`):

```bash
th=$(awk '/<thead/,/<\/thead>/' path/to/<name>-list.html | grep -c "<th ")
sort=$(awk '/<thead/,/<\/thead>/' path/to/<name>-list.html | grep -c "toggleSort(")
echo "th=$th toggleSort=$sort"
```

`th` must equal `sort` plus exactly one - the `#` row-number column, which is
not a data column and correctly has none. Any other relationship means a
rendered column has no sort control. There is no such thing as a column that
structurally cannot be sorted:

- A plain stored column sorts on itself.
- A column assembled from more than one stored field (a combined address, a
  joined display label) sorts on whichever single underlying column its
  filter already targets - see `upstream_host` on gateway-apps for the
  worked example.
- A derived count or a value read from a joined table sorts through a
  correlated-subquery alias in the repository whitelist
  (`alias=(SELECT ...)`) - see `departmentMemberCountExpr` /
  `departmentPermissionCountExpr` (`internal/platform/postgres/department_repo.go`)
  and `gatewayDomainAppCountExpr` / `gatewayIdentityAppCountExpr`
  (`internal/platform/postgres/gateway_domain_repo.go`,
  `gateway_identity_repo.go`) for the shape. A post-page "attach" read that
  fills the response DTO after `Page` already ran does NOT block this - the
  subquery orders the FULL collection before `LIMIT`/`OFFSET`, independently
  of whatever fills the DTO field afterward.
- A boolean or nullable verdict column (a live check, a match/no-match flag)
  sorts like any other column. NULL ordering is a fact worth a code comment,
  never a reason to omit the control.

For a screen still on `app-data-table` (only correct per `crud-resource.md`
when there is genuinely no lifecycle to redesign into), the equivalent check
is that every `DtColumn` in the component's column list carries `sortable:
true` matched to a real backend whitelist entry - there is no partial-sort
`app-data-table` screen either.

If a column genuinely resists this - it has not happened yet in this
codebase - the report must show the FAILED attempt (the whitelist entry
tried and the actual error), not a first-principles assertion. An assertion
with no attempt behind it is exactly the failure mode this check exists to
catch.

## 2. Filter completeness: the same count, for filters

```bash
th=$(awk '/<thead/,/<\/thead>/' path/to/<name>-list.html | grep -c "<th ")
```

Cross-check against the filter panel: every column counted above (less the
`#` column) needs a corresponding control in the inline filter panel
(`crud-resource.md`'s rule - a column with no filter does not belong in the
table, it belongs in the Show drawer). There is no automated count for this
one since filter controls do not share one DOM shape the way sort buttons
do - read the filter panel section of the component by eye against the
column list and name any gap explicitly.

## 3. Sort/filter whitelist parity: both layers, by name

```bash
grep -n "SortColumns\|FilterSpec" internal/modules/<capability>/<module>/service.go
grep -n "SortColumns\|FilterColumns\|alias=" internal/platform/postgres/<module>_repo.go
```

Every column name in the SERVICE list must appear in the REPOSITORY list and
vice versa - diff them by hand, do not eyeball. A column in only one of the
two is silently dropped to the default sort / no filter with no error
anywhere, which is the exact shape of bug that shipped on Departments/Groups'
`permission_count` before this rule existed. A correlated-subquery alias
counts as present under its caller-facing name (the part before `=`), not
its full SQL text.

## 4. When this applies

Every time: `create-crud` (new resource, Step 8), `modernize-page`
(redesigning an existing screen, Step 6), and any ad hoc edit to a list
screen's columns, sort whitelist, or filter whitelist outside those two
agents - the same mechanical checks apply whether or not an agent is doing
the work. Run these before writing "done" anywhere, not after.
