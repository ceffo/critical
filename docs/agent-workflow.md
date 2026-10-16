# Agent Workflow Configuration

Project-specific settings for the generic `developer`, `code-reviewer`, `tester`, `scoper` and
`supervisor` agents in `.claude/agents/`. The agents hold no project knowledge of their own: they
read this file (linked from `AGENTS.md`) and the docs it points to. Section headings below are a
contract; keep them stable.

## Project docs to read

| Always | `AGENTS.md`, `docs/all.md` (scope, vocabulary, core architectural rule, REST conventions) |
|---|---|
| Backend work | `docs/backend.md` (infrastructure), `docs/clean_architecture.md` |
| Deployment work | `docs/deploy.md` (fly.io runbook), ADR 0024 |
| Rules / game math | `docs/adr/README.md` (index), the relevant ADRs |
| Frontend work | `docs/frontend.md` |

Domain vocabulary and phase scope live in `docs/all.md`.

Docs index: every doc is reachable from `AGENTS.md` → this file. Decisions and rules are indexed in
`docs/adr/README.md`. Design decisions are read from these docs and the tracker, never from engram.

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
- **e2e budget (hard rule):** the whole suite stays under 5 minutes on CI, absolute (the job times out
  at 6). e2e is for what only a browser proves: wiring, navigation, persistence across reload, one
  happy path per user flow, plus the screenshots. Rules, edge cases and error branches belong in Go
  or unit tests; add the lower-level test first, then drop the e2e one it makes redundant. Tests
  that only call the API belong in Go. A new spec justifies itself in the report: what only a
  browser proves there, and why no Go or unit test can. Layout screenshots stay in `layout.spec`.
- Use `just` recipes rather than bare `go` / `golangci-lint` / `npm` (except the single-test case).

## Worktree setup

- Worktree root: `~/code/critical.worktrees/` (briefs in `briefs/`, reports in `reports/`,
  supervisor ledger in `supervisor/ledger.md`).
- Main checkout: `~/code/critical`; default branch `main`; remote `origin`.
- After creating a worktree: `cd web && npm ci` (`web/node_modules` is gitignored). Go needs nothing.
- After pulling `main` in the main checkout: `just dev` reinstalls web deps (`npm install`) when
  `web/package-lock.json` changed since the last install. Restart `just dev` after a dependency change.
- Shared resources: none locked. Tests use temporary SQLite databases today; per
  [ADR 0025](adr/0025-postgresql-storage.md) they move to mocks and an in-memory e2e mode, with no
  database at all.

## Running the app for acceptance checks

- Full stack with live reload: `just dev` (API via air on :8080, Vite on :5173 proxying `/api`, listening on all interfaces, `*.ts.net` hosts allowed).
  State is in the gitignored `.dev/` (`critical.db` with the real catalogue, `fixture.db` without it); `just dev-reset` deletes both. Use it only
  from the main checkout or a worktree you own; it binds fixed ports, so run one at a time.
- Backend alone, throwaway data: `just build-api`, then run `bin/critical-srv` with
  `CRITICAL_DSN`, `CRITICAL_SEEDS_DIR`, `CRITICAL_CATALOGUE` (the real `.catalogue/catalogue.json` with
  `seeds/`, or the fixture `api/testdata/catalogue.json` with `api/testdata/seeds`) and `CRITICAL_ADDR`
  pointing at the scratchpad and a free
  port (never the user's data). Exercise endpoints with `curl`.
- UI: `just e2e [spec args]` builds the API and runs the Playwright suite headlessly (Chromium,
  1440x900). Every test gets its own API + Vite on free ports with a fresh temp DB and the
  `api/testdata` fixture (seeds and spell catalogue), so parallel worktrees don't collide. Artifacts, gitignored, in `web/e2e-results/`:
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

Beads, with the `br` CLI (never `bd` here), plus `bv --robot-*` for triage (`bv --robot-triage`,
`bv --robot-plan`; `bv --robot-help` lists the rest). Bare `bv` launches a TUI: never run it.
Commands: `br ready --json`, `br list --status=open`, `br show <id>`, `br search "<words>"`,
`br create --title … -t <type> -p <0-4> [--parent <id>] [-d …]`, `br update <id> --claim`,
`br close <id> --reason "PR #n"`, `br dep add <issue> <depends-on>`, `br sync --flush-only`. On the
main checkout, pull with `just sync-main`, never `br sync --merge`.

- Work item = bead; id looks like `critical-518.3`. Read: `br show <id>`, `br list`, `br search`,
  `br ready`. Everything that writes (`update`, `close`, `create`, `dep add`, `sync`) is the
  supervisor's or scoper's, run from the main checkout; developers, reviewers and testers never
  write to `.beads/`.
- Claim: `br update <id> --claim`.
- Tracker changes do not ride on feature branches (user decision 2026-10-16: parallel PRs kept
  conflicting on `.beads/issues.jsonl` after every merge). The tracker DB is the truth: the supervisor
  claims at dispatch and closes at PR time (`br close`, reason "PR #n") in the main checkout only,
  and commits the export (`br sync --flush-only`, `.beads/issues.jsonl`) in one `chore: tracker
  export` PR per batch of work (end of a session or a night, or when an export is needed on `main`,
  e.g. for `check-no-tracker-ids` to know a new id). If a PR is closed unmerged, reopen its item.
- Priorities are numbers 0-4. Types: task, bug, feature, epic, chore, docs, question.
- Labels in use: `needs-info` (cannot start, questions listed), `risky` (needs review).
- Queue: `bv --robot-triage`, `br ready --json`.
- Hierarchy: epic + children via `br create --parent`; blocking via `br dep add`.
- Work items are written in the vertical-slice convention: one bead = schema + Go engine + REST
  endpoint + React panel for one feature; split only when too big for one PR (backend bead first,
  UI bead blocked by it).
- Rules items: acceptance criteria name the ADR steps to implement (one criterion per step);
  a missing ADR is filed first.
- Defaults when filing autonomously: priority 3 (2 for a correctness bug in rules math).

## Rulebooks

The PHB and DMG live as agent-readable files in a private sibling repo, `~/code/critical-rulebooks`
(shared by every worktree). Run `just rulebooks` to clone it (or fast-forward it); it needs access to
the private repo and is never part of `just validate` or `just dev`.

For a rules question: `grep -i '<term>' ~/code/critical-rulebooks/INDEX.md` first, then open only the
file it names. Never read whole chapters or INDEX.md whole. If the clone is missing, run
`just rulebooks`; if that fails, say so instead of guessing the rule. The book is a reference: the
project's decided rules (ADRs in `docs/adr`, user rulings) win where they differ. Never copy book text
into this public repo.

## Scope

Current phase and what is out of scope: `docs/all.md` ("Phase 1 Scope"). Later-phase work is filed
as backlog (P4) and never dispatched without the user's say-so.

## Hard rules for implementers

From `docs/all.md`, `docs/backend.md` and `docs/adr/`; the ones reviewers check hardest:
- Game logic lives only in Go; the frontend sends intent and renders structured responses.
- Roll endpoints return a complete breakdown struct. Errors are `{"error": "message", "code": "…"}`; a new refusal gets a row (and code) in `internal/render` and an `apiError.<code>` i18n key (en + fr).
- HP is an append-only ledger; current HP = `SUM(delta)`.
- Architecture layering per `docs/clean_architecture.md`; wiring only in the composition root.
- Tests follow "Testing Conventions" in `docs/backend.md`; rule changes get a test that fails on
  the old code.
- Rules math traces to its design (`docs/adr/`). A change cites the ADR step
  it implements (code comment and commit). Each numbered step gets one named test that fails on the
  old code. Dice mocks give every roll a distinct value and assert exact call counts, so a reused
  or missing roll fails the test. If code and design disagree, stop and report; don't pick one.
- **No tracker ids in the code.** No bead/ticket id (`critical-…`, `qki.…`) in source, comments, test
  names, strings, config or seeds. Code says what it does and why; the link to the work item lives in
  the commit message and the PR only. (User rule, 2026-10-14.) Enforced by `just check-no-tracker-ids`
  (part of `just validate`), which reads the ids from `.beads/issues.jsonl` and scans everything but
  `.beads/`, `docs/` and lock files.
- **No throwaway code.** Data processing and verification are done by committed, deterministic, rerunnable
  scripts or tests; never by throwaway code whose result we rely on. (User rule, 2026-10-15.)
- **No special case per spell, item or buff.** Model the mechanic generically and drive it from the
  data (`seeds/spell_effects.json`, item behaviour): no `if id == "<some_spell>"` branches and no
  per-entity constants in services. If a mechanic is missing, add it as a general effect kind.
- **No per-language columns or fields.** A text in a human language is a key into the generic
  `localized_texts` table (one row per language); never a `name_fr`-style column, DTO field, TS property
  or catalogue key. The API serves a language map. [ADR 0023](adr/0023-localized-text.md); enforced by
  `just check-no-language-fields` (part of `just validate`; scans tracked files but docs and the applied
  migrations 027/029/030/031 with their tests; `scripts/test-check-no-language-fields.sh` holds the
  rejected fixtures).
- **No presentation text from the backend.** The API returns structured data; wording, formatting and
  formulas for display are built in the frontend (i18n). No `fmt.Sprintf`/`Fprintf` to compose
  user-facing text in Go (error messages excepted). Enforced by golangci `forbidigo` on
  `api/internal/domain/` (production code): the `fmt.Sprint*`/`Fprint*` family is refused, errors
  use `fmt.Errorf` or `domain.FieldErrorf`. The action log's `summary` is only text the player typed;
  every other entry is a structured payload the UI words (`web/src/lib/logSummary.ts`).
- Rules items end the report with a `RULES TRACE` block: `ADR step -> code location -> test name`.
  A step with no test is not done.

## Review

Needs review (any of): a migration or schema change; ledger/state semantics (HP ledger, buffs, spell
slots, round counter); rules math (attack, crit, damage, THAC0); a change to the API contract or to
domain structs used by several packages; a wide architectural refactor. Not reviewed: UI, copy,
docs, tests, small refactors.

Reviewer checklist, in priority order:
1. **Rules correctness**: quote each ADR step the change touches and verify its test exists
   and would fail if the step were broken (reused dice, wrong order, wrong count). "Met by reading"
   is not acceptable for rules math: a rules criterion without a test is a finding. AD&D 2.5 math per `docs/adr/` (hit if `roll + modifiers ≥ THAC0 -
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
8. **Tests** prove the criteria and would fail on the old code; table-driven rules, mocks for ports;
   no test opens a database or asserts SQL text ([ADR 0025](adr/0025-postgresql-storage.md) step 6).
   **e2e budget**: a new or extended Playwright spec is one happy path per flow, justified in the
   report; rules, edge cases and API-only checks are in Go/unit tests, not e2e (suite < 5 min on CI).
9. **Hard rules for implementers** above, incl. no tracker ids in code, no per-entity special
   cases, and no presentation text in the backend: the API carries structured data only; display
   text is never built by the backend, and a text in a human language is never a per-language column
   or field (one `localized_texts` row per language, [ADR 0023](adr/0023-localized-text.md)). Challenge the item's or brief's own design against these rules. Each violation is at least medium; design is not style.
10. **No cut text** ([ADR 0022](adr/0022-activity-focus-layout.md) step 1): at the layout viewports
   (1280x800 to 2560x1440), important text (names, values, journal text, labels, captions) is never
   cut or ellipsed: no `truncate`, `text-ellipsis` or `line-clamp` on it, no `whitespace-nowrap` in a
   box that hides its overflow; it wraps or gets room. Only the exceptions listed in ADR 0022 step 1.2.
   A UI change's region is covered by `expectNoClippedText` in `layout.spec`, and its screenshots
   show no cut text. A cut name is at least medium.

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

CI: GitHub Actions run on every PR (`gh pr checks <n>`); `just validate` stays the local gate. Jobs: `validate` (`just validate`), `e2e` (`just e2e`; hard limit: the suite must stay under 5 minutes, job timeout 6; Playwright Chromium cached, 2 workers, 1 retry, `web/e2e-results/` uploaded as an artifact), `generated` (bob/mockery output is up to date). PR body ends with
`🤖 Generated with [Claude Code](https://claude.com/claude-code)`. Only the user merges PRs. Agents and the supervisor never merge and never ask whether to merge: a PR is reported as ready (CI green), nothing more.

## Commits

`<type>: <short description>` (`feat`, `fix`, `test`, `docs`, `chore`), optionally with a scope and
the bead id, e.g. `feat(dal): … (critical-518.1)`. Never commit broken tests or a failing build.
Run `br sync --flush-only` before committing tracker changes. Trailer: `Co-Authored-By:` naming the
model actually running.

## Memory

Engram (project `critical`; cross-project gotchas in `personal`) holds **only** situational
know-how: how to do X, tool gotchas, environment quirks. Design decisions (rules, architecture,
data, UI, process choices the user makes) go **only** into project docs: an ADR in `docs/adr/`
(indexed in `docs/adr/README.md`) or the relevant doc, in the PR the decision belongs to. Never read
design decisions from engram; read the docs.
