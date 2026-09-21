---
name: wevetel-generators
description: How to scaffold code with the wevetel CLI generators and the manual wiring they leave for you. Read before running make:*/ng:*/module:make.
---

# wevetel Generators

`wevetel` (built from `cmd/wevetel`) scaffolds DDD layers and Angular features from embedded templates. Build with `make build-cli` (→ `bin/wevetel`).

## Command groups

| Group | Commands |
|---|---|
| Go layers | `make:entity`, `make:repository`, `make:service`, `make:dto`, `make:seeder` |
| Web layer | `make:handler`, `make:routes`, `make:middleware`, `make:job`, `make:event`, `make:listener`, `make:command`, `make:mail`, `make:provider` |
| Full scaffold | `make:full` |
| Angular | `ng:module`, `ng:component`, `ng:service`, `ng:model`, `ng:guard`, `ng:sync` |
| Migrations | `migrate:diff`, `migrate:run`, `migrate:rollback`, `migrate:status` |
| Module tooling | `module:list`, `module:status`, `module:make` |
| Operator | `vpn:add` |

## Rules

- `<Name>` must be an **exported Go identifier** - uppercase first letter, letters/digits only (underscores/dots rejected).
- Optional `[module]` defaults to the snake_case form of the name.
- `--force` overwrites an existing file; generators **refuse to write outside the repo**.

## Examples

```bash
wevetel make:full Widget billing     # entity+repo+service+dto+handler+seeder + Angular feature
wevetel make:entity Widget
wevetel ng:sync Site                 # regenerate the TS model from the Go entity
```

## After generating - wire it yourself

`make:full` / `module:make` scaffold but do **NOT** wire. `module:make` prints these steps; do them:

1. Add a Wire provider set in `internal/wire` and include it in the graph.
2. Register routes in `internal/surfaces/routes/router.go`: add a `routeGuards` row naming the
   permission that admits, then mount on the `/api/v1/secure` group through `guard()`.
   `guard()` panics on a route with no row, so a scaffolded route that is only mounted
   will crash the API at construction rather than ship ungated. New permissions must also
   be added to `internal/domain/permission/catalogue.go` and placed in a bundle under
   `db/seeders/bundle/` (there are no wildcards, so an unplaced permission reaches nobody).
3. `wevetel migrate:diff add_<name>_table` then `wevetel migrate:run`.
4. `wevetel ng:sync <Entity>` to generate the TypeScript model.
5. `go build ./... && go test ./...`.

## `ng:sync` type mapping

Parses the Go entity with `go/parser` (not a template):

| Go | TS |
|---|---|
| `string`, `uuid.UUID`, `time.Time` | `string` |
| int/uint/float | `number` |
| `bool` | `boolean` |
| `datatypes.JSON` | `any` |
| `*T` | `T \| null` |
| `[]T` | `T[]` |

Skips any `json:"-"` field (so credentials never reach the model) and expands embedded `Base` into `id`/`created_at`/`updated_at`/`deleted_at`.

Last updated: auto-generated from codebase scan
