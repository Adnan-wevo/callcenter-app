# Provenance of this folder

`.claude/`, `.agents/` and `.mcp.json` were copied from the sibling
`wevetel-bastion` repo, because this service is being built to the same
conventions (see `docs/ui-design-spec.md`).

They are NOT all applicable here. Read this before trusting one.

## What was changed on the way in

- **`.mcp.json`** — bastion declares a second MCP server,
  `wevetelbastion`, pointing at `tools/mcp-server/server.mjs`. That path does
  not exist in bastion either, so the entry was already dead there and was
  dropped rather than copied forward. Only `spartan-ui` remains.

## Applies here

- `skills/code-style.md`
- `skills/git-commit-style.md`
- `skills/authz-patterns.md` — this service uses the same fail-closed
  permission model (see `internal/auth`, `web/src/app/core/authz`)
- `skills/central-list-page.md`, `skills/list-screen-verify.md` — the list
  screen shape the report screens follow
- `skills/api-handler-patterns.md` — with one caveat: bastion's handlers are
  a different Go codebase. Treat it as a pattern reference, not a spec.
- `../.agents/skills/spartan/**` — Spartan UI guidance. **Aspirational
  here:** this app does not currently use Spartan. `web/src/styles.css`
  hand-writes the same primitives as plain Tailwind classes so the app
  carries no component-library dependency. The look is deliberately the
  same, so adopting Spartan later is a swap rather than a redesign — and
  that is when this guidance starts applying literally.

## Does NOT apply here

These are bastion-domain specific and describe systems this service has no
part of:

- `skills/helpdesk-setup-page.md`, `agents/modernize-helpdesk-page.md`
- `skills/vpn-site-onboarding.md`, `agents/add-site.md`
- `skills/migration-workflow.md`, `agents/create-migration.md` — bastion uses
  Atlas against its own schema. This service owns no schema: `qstats` is
  externally owned by the PBX (see `docs/extraction-plan.md` §1.2), and
  `migrations/qstats/` here is a local-testing fixture, not a migration
  history.
- `skills/wevetel-generators.md`, `agents/create-module.md`,
  `agents/create-crud.md` — generators bound to bastion's own layout.

Kept rather than deleted so the conventions stay readable side by side; just
do not run them against this repo expecting them to fit.
