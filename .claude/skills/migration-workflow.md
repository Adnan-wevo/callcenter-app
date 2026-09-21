---
name: migration-workflow
description: Atlas model-first migration workflow for WevetelBastion and its gotchas. Read before changing the schema.
---

# Migration Workflow

Schema is driven by the GORM entities and produced by **Atlas** diffing the models - never GORM `AutoMigrate`.

## Steps

1. Edit the entity in `internal/domain/entity/*.go`.
2. If it's a **new** entity, register it in `internal/platform/postgres/schema/main.go` (existing entities are already registered).
3. Generate the diff (needs a Docker dev database - `atlas.hcl` sets `dev = "docker://postgres/16/dev"`):

```bash
make migrate-diff name=add_widget_region     # or: wevetel migrate:diff add_widget_region
```

4. **Read the generated SQL** in `db/migrations/` before applying.
5. Apply and confirm:

```bash
make migrate-run       # wevetel migrate:run
make migrate-status    # wevetel migrate:status
```

## Gotcha: NOT NULL on a populated table

Atlas emits a bare `ADD COLUMN ... NOT NULL`, which only works on an empty table. Against rows it fails with `column ... contains null values`. Rewrite as add-default then drop-default:

```sql
ALTER TABLE "x" ADD COLUMN "c" varchar(512) NOT NULL DEFAULT '';
ALTER TABLE "x" ALTER COLUMN "c" DROP DEFAULT;
```

See `db/migrations/20260716072041_add_session_access_url.sql` for the reference. After any hand edit, **re-hash** `atlas.sum` with Atlas so it stays valid.

## Connection URLs

- `ATLAS_DB_URL` is built by `scripts/atlas-url.sh` from `.env` and **percent-encodes** the password (`@`→`%40`, `#`→`%23`). The Makefile/`wevetel migrate:*` export it automatically.
- The app's own DSN (`config.DSN()`) is libpq **key=value** form, not a `postgres://` URL - do not rewrite it as a URL (the password may contain `@`/`#`).

Last updated: auto-generated from codebase scan
