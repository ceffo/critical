# Agent Workflow Configuration

Project-specific settings for the generic `developer`, `code-reviewer`, `tester`, `scoper` and
`supervisor` agents in `.claude/agents/`. The agents hold no project knowledge of their own: they
read this file (linked from `AGENTS.md`) and the docs it points to. Section headings below are a
contract; keep them stable.

## Project docs to read

| Always | `AGENTS.md`, `docs/all.md` (scope, vocabulary, core architectural rule, REST conventions) |
|---|---|
| Backend work | `docs/backend.md`, `docs/clean_architecture.md` |
| Frontend work | `docs/frontend.md` |

Domain vocabulary and phase scope live in `docs/all.md`.

## Validation

- Full gate, from the repo root: `just validate` (backend lint + tests + build, frontend lint +
  typecheck + production build). Mandatory before any work item is reported done.
- Narrow checks while iterating: `just vet-api`, `just lint-api`, `just test-api`, `just lint-web`,
  `just typecheck-web`, `just build-api`, `just build-web`; `go test -run X ./path` from `api/` for
  a single test.
- UI items: add or extend Playwright specs (`web/e2e/`) for each UI acceptance flow and run
  `just e2e` green (separate from `just validate`, which only lints/typechecks the e2e TS files).
  List the screenshot paths (`web/e2e-results/screens/<spec>/<name>.png`) in the report. First run
  on a machine: `just e2e-install` (Playwright Chromium).
- Use `just` recipes rather than bare `go` / `golangci-lint` / `npm` (except the single-test case).

## Worktree setup

- Worktree root: `~/code/critical.worktrees/` (briefs in `briefs/`, reports in `reports/`,
  supervisor ledger in `supervisor/ledger.md`).
- Main checkout: `~/code/critical`; default branch `main`; remote `origin`.
- After creating a worktree: `cd web && npm ci` (`web/node_modules` is gitignored). Go needs nothing.
- After pulling `main` in the main checkout: `just dev` reinstalls web deps (`npm install`) when
  `web/package-lock.json` changed since the last install. Restart `just dev` after a dependency change.
- Shared resources: none locked. Tests use temporary SQLite databases.

## Running the app for acceptance checks

- Full stack with live reload: `just dev` (API via air on :8080, Vite on :5173 proxying `/api`, listening on all interfaces, `*.ts.net` hosts allowed).
  State is in the gitignored `.dev/`; `just dev-reset` deletes the dev database. Use it only
  from the main checkout or a worktree you own; it binds fixed ports, so run one at a time.
- Backend alone, throwaway data: `just build-api`, then run `bin/critical-srv` with
  `CRITICAL_DSN`, `CRITICAL_SEEDS_DIR` and `CRITICAL_ADDR` pointing at the scratchpad and a free
  port (never the user's data). Exercise endpoints with `curl`.
- UI: `just e2e [spec args]` builds the API and runs the Playwright suite headlessly (Chromium,
  1440x900). Every test gets its own API + Vite on free ports with a fresh temp DB and the real
  `seeds/`, so parallel worktrees don't collide. Artifacts, gitignored, in `web/e2e-results/`:
  `screens/<spec>/<name>.png` (screenshots), `artifacts/` (trace.zip on failure), `report/` (HTML;
  `cd web && npx playwright show-report e2e-results/report`).
- Type check and build are covered by `just validate` and don't prove a panel looks right.

## Generated code

Generated files are committed and never hand-edited.

| Trigger | Command |
|---|---|
| Migration added or changed under `api/internal/storage/sqlite/migrations/`; `.sql` changed under `api/internal/adapters/*/queries/`; bob version bumped | `just generate` |
| New or changed service port (mockery) | `just mocks` |

Generated paths: `api/internal/adapters/*/queries/*.bob.go`, `api/internal/storage/sqlite/bob/**`,
mockery output. bob constraints are in `AGENTS.md` ("DAL Code Generation"). Migrations are new
files only, never an edited applied one. When two branches both regenerate, regenerate after the
merge instead of hand-merging.

## Tracker

Beads, with the `br` CLI (never `bd` here), plus `bv --robot-*` for triage; commands and rules are
in `AGENTS.md` ("Beads Workflow Integration"). Bare `bv` launches a TUI: never run it.

- Work item = bead; id looks like `critical-518.3`. Read: `br show <id>`, `br list`, `br search`,
  `br ready`. Everything that writes (`update`, `close`, `create`, `dep add`, `sync`) is the
  supervisor's or scoper's, run from the main checkout; developers, reviewers and testers never
  write to `.beads/`.
- Claim: `br update <id> --claim`. Close after the PR is merged: `br close <id> --reason="…"`.
  Export after tracker changes: `br sync --flush-only`.
- Priorities are numbers 0-4. Types: task, bug, feature, epic, chore, docs, question.
- Labels in use: `needs-info` (cannot start, questions listed), `risky` (needs review).
- Queue: `bv --robot-triage`, `br ready --json`.
- Hierarchy: epic + children via `br create --parent`; blocking via `br dep add`.
- Work items are written in the vertical-slice convention: one bead = schema + Go engine + REST
  endpoint + React panel for one feature; split only when too big for one PR (backend bead first,
  UI bead blocked by it).
- Defaults when filing autonomously: priority 3 (2 for a correctness bug in rules math).

## Scope

Current phase and what is out of scope: `docs/all.md` ("Phase 1 Scope"). Later-phase work is filed
as backlog (P4) and never dispatched without the user's say-so.

## Hard rules for implementers

From `docs/all.md` and `docs/backend.md`; the ones reviewers check hardest:
- Game logic lives only in Go; the frontend sends intent and renders structured responses.
- Roll endpoints return a complete breakdown struct. Errors are `{"error": "message"}`.
- HP is an append-only ledger; current HP = `SUM(delta)`.
- Architecture layering per `docs/clean_architecture.md`; wiring only in the composition root.
- Tests follow "Testing Conventions" in `docs/backend.md`; rule changes get a test that fails on
  the old code.

## Review

Needs review (any of): a migration or schema change; ledger/state semantics (HP ledger, buffs, spell
slots, round counter); rules math (attack, crit, damage, THAC0); a change to the API contract or to
domain structs used by several packages; a wide architectural refactor. Not reviewed: UI, copy,
docs, tests, small refactors.

Reviewer checklist, in priority order:
1. **Rules correctness**: AD&D 2.5 math per `docs/backend.md` (hit if `roll + modifiers ≥ THAC0 -
   target_AC`, damage formula, crit table, nat 1 / nat 20), off-by-one in rounds/charges,
   negative/overflow values, dice injected through the dice port.
2. **State integrity**: append-only ledger; buff expiry on Next Round (charge vs duration); spell
   slots consumed once, reset on long rest; check-then-write without a transaction; partial writes.
3. **No game logic in the frontend.**
4. **API contract**: `/api/` resource/action naming, JSON shapes, central error mapping, full
   breakdown on rolls; `web/src/types` matches the Go structs.
5. **Architecture** per `docs/clean_architecture.md`.
6. **Migrations and DAL**: new files only; safe on existing data; generated code regenerated and
   committed; bob constraints respected.
7. **Seeds**: `seeds/*.json` and the loader stay valid and idempotent.
8. **Tests** prove the criteria and would fail on the old code; table-driven rules, mocks for ports.

Severity examples: critical = wrong game math in a main flow, ledger/state corruption, broken
migration on existing data; high = game logic in the frontend; medium = architecture-layer
violation.

Visual review uses the e2e screenshots (`web/e2e-results/screens/`, produced by `just e2e`) instead
of running the app by hand. Reference: `docs/frontend.md` (dashboard layout, display conventions, shadcn/ui).

## Model selection

Opus: core rules-engine math, ledger/state semantics across several packages, wide refactors,
UI-heavy beads (new panel, redesign). Sonnet: everything else. Reviewer: Sonnet, Opus for core
rules math.

## Parallelism

Two items may run in parallel only if they add no migrations (or only to disjoint tables), touch no
shared packages/files/components, don't affect each other's generated output, and share no API
contract or domain struct.

## CI and PRs

CI: GitHub Actions run on every PR (`gh pr checks <n>`); `just validate` stays the local gate. PR body ends with
`🤖 Generated with [Claude Code](https://claude.com/claude-code)`. Only the user merges PRs. Agents and the supervisor never merge and never ask whether to merge: a PR is reported as ready (CI green), nothing more.

## Commits

`<type>: <short description>` (`feat`, `fix`, `test`, `docs`, `chore`), optionally with a scope and
the bead id, e.g. `feat(dal): … (critical-518.1)`. Never commit broken tests or a failing build.
Run `br sync --flush-only` before committing tracker changes. Trailer: `Co-Authored-By:` naming the
model actually running.

## Memory

Engram project: `critical`; cross-project gotchas go to `personal`.
