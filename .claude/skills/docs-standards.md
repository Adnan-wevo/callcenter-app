---
name: docs-standards
description: Documentation standards for WevetelBastion - the docs/ layout, numbering convention, module ordering, and content rules. Read before adding or updating docs.
---

# Docs Standards

## Folder structure

```
docs/
├── api/
├── architecture/
├── database/
├── deployment/
├── development/
├── getting-started/
├── git/
├── claude/
└── modules/
```

`README.md` lives at the repo root, not in `docs/`.

## Numbering convention

All files and folders use a `000-`, `001-`, `002-` … prefix. `000-` is always the first file in a folder (an overview/entry point).

## Module order (by dependency - auth first)

```
001-auth/                 008-kubernetes/            016-departments-groups/  023-rnd-sdlc/
002-remote-workspaces/    010-web/                   017-helpdesk/            024-woztell/
003-remote-accounts/      011-cli-generators/        018-customers/           025-evolution/
004-remote-sites/         012-approvals/              019-audit/
005-users/                013-gateway/                020-ssl-updater/
006-remote-sessions/      014-mail/                   021-dns-proxy/
007-vpn/                  015-sso/                    022-tenancy/

009-tui is RETIRED (the SSH transport was removed on 2026-07-30). The numbers
are not reused and not renumbered, so a link in an old commit still resolves.
```

This must always match CLAUDE.md's own "Module numbering" line, which is the
authoritative source - check there first when this list looks out of date.

A new module gets the next available number by dependency: fewer dependencies → lower number.

## Module files vary

Unlike a fixed template, modules here carry **only the files that map to what they actually have**. Every module has `000-overview.md`; beyond that, e.g.:

- `002-sites/` → `001-models.md`, `002-service.md`, `003-endpoints.md`
- `004-sessions/` → `001-model.md`, `002-service.md`, `003-spawn-flow.md`, `004-endpoints.md`, `005-expiry-job.md`
- `007-vpn/` → `001-secret-manager.md`, `002-cli-vpn-add.md`, `003-site-yaml.md`

Do not create a file for a layer the module does not have.

## Content rules

- Document **only what exists** in code - real type names, method names, route paths, table names, field names, constants.
- No placeholder text, no `TODO`, no "coming soon".
- Every file **ends with** the line:

```
Last updated: auto-generated from codebase scan
```

- Format per file: H1 + one-line summary → `## Overview` (2–4 sentences) → tables for field/endpoint/column lists → fenced code blocks with a language → bold key terms on first use → blockquote (`>`) notes for warnings/caveats.

## Which agent

- First-time full generation → `docs-init` agent.
- Ongoing updates when code changes → `docs-generator` agent.

Last updated: auto-generated from codebase scan
