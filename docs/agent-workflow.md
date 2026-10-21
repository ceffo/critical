# Agent Workflow Configuration

Project-specific settings for the generic `developer`, `code-reviewer`, `tester`, `scoper` and
`supervisor` agents in `.claude/agents/`. The agents hold no project knowledge of their own: they
read this file (linked from `AGENTS.md`) and the docs it points to. Section headings below are a
contract; keep them stable.

## Project docs to read

| Always | `AGENTS.md`, `GLOSSARY.md` (the vocabulary), `docs/all.md` (scope, core architectural rule, REST conventions) |
|---|---|
| Backend work | `docs/backend.md` (infrastructure), `docs/clean_architecture.md` |
| Deployment work | `docs/deploy.md` (fly.io runbook), ADR 0024 |
| Rules / game math | `docs/adr/README.md` (index), the relevant ADRs |
| Frontend work | `docs/frontend.md` |

Domain vocabulary lives in `GLOSSARY.md` ([ADR 0036](adr/0036-vocabulary.md)); phase scope in `docs/all.md`.

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
- **Visual QA before committing (hard rule):** a UI change is proven by the developer, never by the
  user. Look at the screenshots you produced (open the png with your file-reading tool), at 1920x1080
  and 2560x1440 as well as the spec's sizes, with real scrollbars shown (headless Chromium with
  `ignoreDefaultArgs: ['--hide-scrollbars']`; never a visible window, see "Hard rules"), and keep an
  automated check (`layout.spec`) that would fail on the defect.
- **e2e budget (hard rule):** the whole suite stays under 5 minutes on CI, absolute (the job times out
  at 6). e2e is for what only a browser proves: wiring, navigation, persistence across reload, one
  happy path per user flow, plus the screenshots. Rules, edge cases and error branches belong in Go
  or unit tests; add the lower-level test first, then drop the e2e one it makes redundant. Tests
  that only call the API belong in Go. A new spec justifies itself in the report: what only a
  browser proves there, and why no Go or unit test can. Layout screenshots stay in `layout.spec`.
- **Local dev gate (hard rule):** a change to the justfile, `docker-compose.yml`, `api/.air.toml`,
  `scripts/dev*`, the storage engine, the migrations or the server's start-up/config runs
  `just dev-smoke` green (start, answer, graceful restart, Ctrl-C and `just stop` leave nothing
  running) and says so in the report. "It starts" is not enough: the user's daily loop (start, stop,
  Ctrl-C, restart, reset, their existing local data) must still work after the merge.
- Use `just` recipes rather than bare `go` / `golangci-lint` / `npm` (except the single-test case).

## Worktree setup

- Worktree root: `~/code/critical.worktrees/` (briefs in `briefs/`, reports in `reports/`,
  supervisor ledger in `supervisor/ledger.md`).
- Main checkout: `~/code/critical`; default branch `main`; remote `origin`.
- After creating a worktree: `cd web && npm ci` (`web/node_modules` is gitignored). Go needs nothing.
- After pulling `main` in the main checkout: `just dev` reinstalls web deps (`npm install`) when
  `web/package-lock.json` changed since the last install. Restart `just dev` after a dependency change.
- Shared resources: none locked. The e2e suite runs on the API's in-memory storage and no Go test
  opens a database ([ADR 0025](adr/0025-postgresql-storage.md)). `just dev` uses the checkout's own
  docker compose PostgreSQL (each worktree is its own compose project, with its own volume).

## Running the app for acceptance checks

- Full stack with live reload: `just dev` (API via air on :8080, Vite on :5173 proxying `/api`, listening on all interfaces, `*.ts.net` hosts allowed; `CRITICAL_API_PORT` / `CRITICAL_WEB_PORT` move them). `just stop` stops it (database included), `just status` shows it; `just dev-smoke` proves the lifecycle on free ports (run it for any change to the justfile, compose, air or storage start-up).
  It starts the docker compose PostgreSQL (`just dev-db`, host port 5432, `CRITICAL_PG_PORT` moves it): database `critical` with the real
  catalogue, `critical_fixture` without it; `just dev-reset` deletes both (the compose volume). Use it only
  from the main checkout or a worktree you own; never stop a stack that is not yours (`just stop` only touches its own checkout).
- Backend alone, throwaway data: `CRITICAL_PG_PORT=<free port> just dev-db` in your worktree (its own
  compose project and volume), `just build-api`, then run `bin/critical-srv` with `CRITICAL_DSN`
  (`postgres://critical:critical@localhost:<port>/critical_fixture?sslmode=disable`, or a database you
  create with `docker compose exec postgres createdb -U critical <name>`), `CRITICAL_SEEDS_DIR`,
  `CRITICAL_CATALOGUE` (the fixture `api/testdata/catalogue.json` with `api/testdata/seeds`, or the real
  `.catalogue/catalogue.json` with `seeds/`), `CRITICAL_ENV=dev` (the dev login; without it and
  without OAuth settings the server refuses to start) and `CRITICAL_ADDR` on a free port (never the user's data).
  `just dev-reset` in the worktree removes its database afterwards. Exercise endpoints with `curl`.
- UI: `just e2e [spec args]` builds the API and runs the Playwright suite headlessly (Chromium,
  1440x900). Every test gets its own API (in-memory storage, no database) + Vite on free ports with the
  `api/testdata` fixture (seeds and spell catalogue), so parallel worktrees don't collide. Artifacts, gitignored, in `web/e2e-results/`:
  `screens/<spec>/<name>.png` (screenshots), `artifacts/` (trace.zip on failure), `report/` (HTML;
  `cd web && npx playwright show-report e2e-results/report`).
- Type check and build are covered by `just validate` and don't prove a panel looks right.

## Generated code

Generated files are committed and never hand-edited.

| Trigger | Command |
|---|---|
| Migration added under `api/internal/storage/postgres/migrations/`; `.sql` changed under `api/internal/adapters/*/queries/` or `api/internal/storage/postgres/queries/`; sqlc version bumped, `api/sqlc.yaml` changed | `just generate` |
| New or changed service port, or a sqlc `Querier` changed (mockery) | `just mocks` |

Generated paths: sqlc output (`api/internal/adapters/*/queries/` and `api/internal/storage/postgres/queries/`: `{db,models,querier}.go`, `*.sql.go`), mockery output.
The generators' constraints are in `docs/backend.md` ("DAL code generation"). Migrations are new
files only, never an edited applied one. When two branches both regenerate, regenerate after the
merge instead of hand-merging.

## Tracker

Linear: team `CRI`, project "critical-hit". The workspace shape (priorities, states, labels) is written once
in [`scripts/linear-mapping.json`](../scripts/linear-mapping.json) and described in [linear.md](linear.md);
this file links it, never copies it. The decision is [ADR 0031](adr/0031-linear-tracker.md).

- Work item = Linear issue; id `CRI-<n>` (e.g. `CRI-42`). Old bead ids (`critical-518.3`) are looked up in
  `docs/beads-id-map.json` (`{"critical-x":"CRI-n"}`); the old text is in the annotated git tag
  `beads-archive` (`git show beads-archive:.beads/issues.jsonl`).
- Tools. Agents use the Linear MCP tools listed in their `tools:` (`mcp__linear-server__…`): read
  (`get_issue`, `list_issues` with `query` for search, `list_comments`) for developer, code-reviewer and
  tester; read plus write (`save_issue`, `save_comment`, `list_issue_labels`, `list_issue_statuses`) for the
  supervisor and the scoper. Where a shell is the better tool, `scripts/linear.sh '<GraphQL>'` (needs
  `$LINEAR_API_KEY` or the key file written by `just linear-setup`; never in the repo).
- Queue: `scripts/linear-ready.sh [--json]` (state Todo, no open blocker, no `needs-info` / `deferred`
  label, not an `epic`; Urgent first, then oldest). Pending questions:
  `.claude/scripts/pending-questions.sh`.
- States: Todo (open) → In Progress (claimed by the supervisor at dispatch:
  `.claude/scripts/linear-state.sh CRI-n "In Progress"`) → In Review (PR opened, by
  `.claude/scripts/ship.sh`) → Done (by Linear's GitHub integration when the PR merges; nobody closes by
  hand). A PR closed unmerged goes back to Todo (`linear-state.sh CRI-n Todo "PR #n closed unmerged"`).
  Deferred work is Backlog with the label `deferred`. The PR title ends with `(CRI-n)` and its body has a
  `Closes CRI-n` line, which is what the integration reads.
- Everything that writes (`save_issue`, `save_comment`, `linear-state.sh`) is the supervisor's or the
  scoper's; developers, reviewers and testers read only. Tracker state lives in Linear, not in git: there
  is no export, no tracker PR and nothing tracker-related to merge or commit.
- Priority: Urgent / High / Medium / Low / None (old P0 / P1 / P2 / P3 / P4). Types are labels: Bug,
  Feature, `chore`, `docs`, `question`; a task has no type label.
- Labels in use: `needs-info` (cannot start: waiting on the user; the issue description holds a
  "## Questions" section, each question with the default applied meanwhile), `risky` (needs review),
  `epic`, `deferred`, plus the area labels (`backend`, `frontend`, `infra`, `data-prep`, `tooling`,
  `mvp-saturday`). Every open `needs-info` or `question` issue is a pending question the supervisor
  restates to the user in every reply (`.claude/scripts/pending-questions.sh`).
- Hierarchy: an epic is a parent issue labelled `epic` (sub-issues via `parentId`); blocking is a
  "blocked by" relation (`save_issue` `blockedBy`).
- Work items are written in the vertical-slice convention: one issue = schema + Go engine + REST
  endpoint + React panel for one feature; split only when too big for one PR (backend issue first,
  UI issue blocked by it).
- Rules items: acceptance criteria name the ADR steps to implement (one criterion per step);
  a missing ADR is filed first.
- Defaults when filing autonomously: priority Low (Medium for a correctness bug in rules math).

## Rulebooks

The PHB and DMG live as agent-readable files in a private sibling repo, `~/code/critical-rulebooks`
(shared by every worktree). Run `just rulebooks` to clone it (or fast-forward it); it needs access to
the private repo and is never part of `just validate` or `just dev`.

For a rules question, ask the `rules-lookup` agent (Agent tool, `subagent_type: rules-lookup`; Haiku,
one question per call): it reads the books and returns the rule with its file, section and a short
quote, so the book text stays out of your context. Check its quote before a rule drives game logic.
If you can't start agents (you run as a subagent yourself), look it up directly: `grep -i '<term>'
~/code/critical-rulebooks/INDEX.md` first, then open only the file it names. Never read whole chapters
or INDEX.md whole. If the clone is missing, run
`just rulebooks`; if that fails, say so instead of guessing the rule. The book is a reference: the
project's decided rules (ADRs in `docs/adr`, user rulings) win where they differ. Never copy book text
into this public repo.

## Scope

Current phase and what is out of scope: `docs/all.md` ("Phase 1 Scope"). Later-phase work is filed
as backlog (P4) and never dispatched without the user's say-so.

## Hard rules for implementers

From `docs/all.md`, `docs/backend.md` and `docs/adr/`; the ones reviewers check hardest:
- **Never open a visible window on the user's desktop** (browser, viewer, anything): the user works
  on this machine. Every UI check runs headless, in memory: `just e2e` / Playwright headless
  Chromium; to see real scrollbars, launch it with `ignoreDefaultArgs: ['--hide-scrollbars']`, or
  use `xvfb-run` for a virtual display. Never `headless: false`, `--headed`, `xdg-open` or a plain
  `chromium` (user, 2026-10-18: "any ui check has to be done in memory, not spawning a friggin
  window").
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
- **No tracker ids in the code.** No bead/ticket id (`CRI-<n>`, `critical-…`, `qki.…`) in source, comments, test
  names, strings, config or seeds. Code says what it does and why; the link to the work item lives in
  the commit message and the PR only. (User rule, 2026-10-14.) Enforced by `just check-no-tracker-ids`
  (part of `just validate`; `scripts/test-check-no-tracker-ids.sh` plants ids in throwaway trees). Linear
  ids are matched by pattern; old bead ids are read from `docs/beads-id-map.json` (the cutover's id map).
  It scans everything but `docs/` and lock files.
- **No throwaway code.** Data processing and verification are done by committed, deterministic, rerunnable
  scripts or tests; never by throwaway code whose result we rely on. (User rule, 2026-10-15.)
- **No special case per spell, item or buff.** Model the mechanic generically and drive it from the
  data (`seeds/spell_effects.json`, item behaviour): no `if id == "<some_spell>"` branches and no
  per-entity constants in services. If a mechanic is missing, add it as a general effect kind.
- **No per-language columns or fields.** A text in a human language is a key into the generic
  `localized_texts` table (one row per language); never a `name_fr`-style column, DTO field, TS property
  or catalogue key. The API serves a language map. [ADR 0023](adr/0023-localized-text.md); enforced by
  `just check-no-language-fields` (part of `just validate`; scans tracked files but docs;
  `scripts/test-check-no-language-fields.sh` holds the rejected fixtures).
- **No presentation text from the backend.** The API returns structured data; wording, formatting and
  formulas for display are built in the frontend (i18n). No `fmt.Sprintf`/`Fprintf` to compose
  user-facing text in Go (error messages excepted). Enforced by golangci `forbidigo` on
  `api/internal/domain/` (production code): the `fmt.Sprint*`/`Fprint*` family is refused, errors
  use `fmt.Errorf` or `domain.FieldErrorf`. The action log's `summary` is only text the player typed;
  every other entry is a structured payload the UI words (its kind's view, `web/src/journal/kinds/`).
  One exception, decided: the invite mails are worded by the mail adapter's embedded templates, not by
  the domain ([ADR 0029](adr/0029-users-login-and-access.md) step 2.5).
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
   committed; sqlc constraints respected.
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

11. **Local dev loop**: a change listed under the local dev gate (Validation) shows `just dev-smoke`
   green in the developer's report, and anything it changes for the user locally (commands, ports,
   required tools, where their data lives, a one-off step such as an import) is written in the
   report. A broken or silently changed daily loop is at least high.

12. **Layout pass rules** ([ADR 0027](adr/0027-space-follows-frequency-of-use.md)): space follows
   frequency of use (step 1: every-turn controls get the room, rare ones one click away, a set rare
   control shows as a tag); the layout is judged **in French first** (step 2: screenshots and the
   content targets in French, the no-cut audit in French and English); **one action word per button
   caption** (step 3: the detail in the tooltip / `aria-label`, never a sentence or a parenthesis).

Severity examples: critical = wrong game math in a main flow, ledger/state corruption, broken
migration on existing data; high = game logic in the frontend; medium = architecture-layer
violation.

Visual review uses the e2e screenshots (`web/e2e-results/screens/`, produced by `just e2e`) instead
of running the app by hand. Reference: `docs/frontend.md` (dashboard layout, display conventions, shadcn/ui).

## Model selection

Sonnet by default, for every role: developers, reviewers, scopers, testers, tooling and fleet
work, UI (new panels included), refactors. Opus only for core rules-engine math: a change to how the
game's rules compute (combat, saves, spell and level arithmetic, effects stacking), for its developer
and its reviewer, and for a scoper whose items will amend a rules ADR. When unsure, Sonnet (user,
2026-10-20: Opus was assigned too liberally and the weekly limit was projected to run out).

## Parallelism

Two items may run in parallel only if they add no migrations (or only to disjoint tables), touch no
shared packages/files/components, don't affect each other's generated output, and share no API
contract or domain struct.

## CI and PRs

CI: GitHub Actions run on every PR (`gh pr checks <n>`); `just validate` stays the local gate. Jobs: `validate` (`just validate`), `e2e` (`just e2e`; hard limit: the suite must stay under 5 minutes, job timeout 6; Playwright Chromium cached, 2 workers, 1 retry, `web/e2e-results/` uploaded as an artifact), `generated` (sqlc/mockery output is up to date), plus `dev-smoke` and `image`.
**Main runs:** a push to `main` runs CI, and Release follows. A Dependabot PR merged by auto-merge starts no push run (GitHub: events caused by `GITHUB_TOKEN` start no workflow), so `dependabot-auto-merge.yml` waits for the merge and runs `gh workflow run ci.yml --ref main`; that dispatched run has no base commit, so `changes` runs every slow job.
**Slow-job skip:** a first job, `changes` (`scripts/ci-skip-slow.sh`, tested by
`scripts/test-ci-skip-slow.sh` in `just validate`), skips `e2e`, `dev-smoke` and `image` when every
changed file is documentation (`*.md`, `docs/**`; on a PR against its base, on a push
against the previous head), or when the PR carries the label `ci-skip` (PR runs only: the push that
merges it runs everything). `validate` and `generated` always run (required checks). A skipped job
counts as a success, so the CI run is green and the Release workflow still starts and decides by its
own rules; `ci-skip` does not imply `no-release`. Adding or removing a label re-runs CI. Don't use
`ci-skip` on a change to the justfile, compose, air, scripts or storage: the local dev gate still
applies to it. PR body has a **What changes for you locally** line (dev commands, ports, tools, local data, one-off steps; or "nothing") and ends with
`🤖 Generated with [Claude Code](https://claude.com/claude-code)`. Only the user merges PRs. Agents and the supervisor never merge and never ask whether to merge: a PR is reported as ready (CI green), nothing more.

## Commits

`<type>(<scope>)?!?: <short description>` (`feat`, `fix`, `test`, `docs`, `chore`, `refactor`, `perf`),
optionally with a scope, and ending with the issue id, e.g. `feat(dal): … (CRI-42)`. The PR title follows the
same format (checked in CI by `.github/workflows/pr-title.yml`): it becomes the squash commit on main,
and the release version bump is read from it. Never commit broken tests or a failing build.
Trailer: `Co-Authored-By:` naming the
model actually running.

## Memory

Engram (project `critical`; cross-project gotchas in `personal`) holds **only** situational
know-how: how to do X, tool gotchas, environment quirks. Design decisions (rules, architecture,
data, UI, process choices the user makes) go **only** into project docs: an ADR in `docs/adr/`
(indexed in `docs/adr/README.md`) or the relevant doc, in the PR the decision belongs to. Never read
design decisions from engram; read the docs.
