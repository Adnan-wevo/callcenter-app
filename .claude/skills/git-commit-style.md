---
name: git-commit-style
description: Git commit + branch conventions for WevetelBastion. Strict Conventional Commits v1.0.0, routine work committed onto `develop`, `main` advanced by deliberate promotion. No "Phase N", no em dashes.
---

# Git Commit Style

Follow **Conventional Commits v1.0.0** exactly: https://www.conventionalcommits.org/en/v1.0.0/

## Commit format

```
<type>[(scope)]: <description>

[optional body]

[optional footer(s)]
```

- **type** - one of `feat`, `fix`, `build`, `chore`, `ci`, `docs`, `style`, `refactor`, `perf`, `test`. `feat` = new feature (SemVer MINOR); `fix` = bug fix (SemVer PATCH).
- **scope** (optional) - a noun in parentheses naming a section of the codebase: `session`, `pod`, `site`, `netpol`, `auth`, `tui`, `web`, …
- **description** - a short summary immediately after `: `. Imperative mood. Keep it to the change itself.
- **body** (optional) - after **one blank line**. Free-form paragraphs. **This is where the "why" goes.**
- **footer(s)** (optional) - after a blank line. Each footer is `Token: value` or `Token #value`; the token uses `-` in place of spaces (e.g. `Refs`, `Reviewed-by`).

### Breaking changes

Either put `!` immediately before the colon, or add a `BREAKING CHANGE:` footer (uppercase):

```
feat(api)!: drop the v0 session endpoint

BREAKING CHANGE: POST /sessions now requires request_id in the body.
```

## Do not

- **No `Phase N` in the subject.** Describe the change, not the project milestone. Write `feat(tui): add terminal UI served over Wish SSH`, not `feat: Phase 5 - ...`.
- **No em-dash tail on the subject.** Do not append ` - why...` to the description. The why belongs in the body, one blank line down.
- No trailing period on the description.

## Examples

Subject only:

```
feat(tui): add BubbleTea terminal UI served over Wish SSH
fix(session): refuse a duplicate spawn for a request with a live session
fix(pod): replace Xvfb+x11vnc with TigerVNC Xvnc
feat(site): add configurable portal scheme/path and per-site insecure TLS
docs: add the documentation suite
```

With a body (the "why", not on the subject line):

```
fix(session): refuse a duplicate spawn for a request with a live session

One approved request could spawn multiple pods, leaving orphaned Chrome
sessions. Spawn now scans the user's active sessions and returns
ErrConflict when the request already backs a live one.
```

## Branch workflow

- **`develop`** is the integration branch. Ongoing work commits directly onto it, in a linear history - no topic branch and no merge commit for routine changes. This is the flow actually in use; an earlier version of this file described a `main`-plus-topic-branches model that the project has never followed.
- **`main`** is the stable trunk. It advances only when `develop` is deliberately promoted, not on every change, so it can sit many commits behind. Promote with a single `--no-ff` merge so the integration point is visible:

```bash
git checkout main
git merge --no-ff develop -m "Merge branch 'develop' into main"
git checkout develop
```

- Reach for a topic branch only when the work is genuinely speculative or long-running enough that landing it half-done on `develop` would break the tree. Branch off `develop`, merge back `--no-ff`, delete the branch.
- **Before rewriting published history** (rebase, amend past HEAD, reset), create a recovery ref first - `git branch backup/<what>-<date>` and a tag - and keep it until the result is verified. When the rewrite is meant to preserve content, prove it: `git diff <backup> HEAD` must produce no output.

## Branch naming

For the occasional topic branch: `feat/<area>-<action>`, `fix/<area>-<issue>`, `refactor/<area>-<change>`, `test/<area>-<coverage>`, `docs/<what>`, `chore/<what>`.

## Rules

- Never `git add .` (or `-A`) - stage file by file, or `git add -p <path>` for mixed changes. This is not style: a bare `go build ./cmd/wevetel/` drops a 71 MB executable in the repo root, which `bin/` does not cover, and a blind `git add -A` once swept exactly that into a commit. Build with `-o bin/…`, and read `git status` before every commit.
- Never commit without user approval - propose the plan first.
- Before any commit, verify `git config user.name` and `git config user.email` - expected `SaidNizamWevetel` / `said.nizam@wevetel.com`. If wrong, stop and report.
- A schema-changing commit ships its reviewed Atlas migration and the updated `atlas.sum`.
- **Never** commit a secret - `.env`, `k8s/01_secret.yaml`, `sites/*.yaml`, `*.pem`/`*.key`/`*.ovpn` are gitignored with `.example` counterparts. Use `command grep` (not the shell `grep` wrapper) when scanning.

## Authorship

- **Sole author.** The committer must be `SaidNizamWevetel <said.nizam@wevetel.com>`.
- **Never add a `Co-Authored-By` footer** (or any co-author trailer) - no AI/assistant credit. A body explaining the *why* is fine and encouraged; a co-author footer is not.

Last updated: auto-generated from codebase scan
