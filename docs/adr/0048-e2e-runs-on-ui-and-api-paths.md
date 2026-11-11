# ADR 0048: e2e runs on UI and API paths only, not after a branch update, within about 3 minutes

**Status**: proposed 2026-11-11 (the rules were decided by the user in triage the same day; the globs
in step 2 and the consequences are the developer's proposal, for the user to confirm before merge).
Amends the "e2e budget" rule and the "CI and PRs" section of `docs/agent-workflow.md`.

## Context

The e2e job takes about 6.3 minutes on a 4-vCPU runner and is most of the project's CI CPU. It ran on
every pull request run, and every `gh pr update-branch` re-ran it on each of the open PRs, although
the PR's own change had not moved. The 5-minute budget was already exceeded.

## Decision

1. **e2e is for the browser.** On a pull request the `e2e` job runs only when the PR's own change can
   alter what a browser sees. The PR's own change is the diff from the merge base of the PR's base and
   its head (`BASE...HEAD`), never the merge commit CI checks out.
2. **The paths that run e2e** (`e2e_path` in `scripts/ci-skip-slow.sh`):
   - `web/**`, except `*.md`, `*.test.ts`, `*.test.tsx` (unit tests), `web/design/**`,
     `web/ui-review/**`, `web/readme-shots/**` and the two capture configs
     (`playwright.ui-review.config.ts`, `playwright.readme-shots.config.ts`), which `just e2e` does not run;
   - `api/internal/rest/**` (routes, handlers, wire types, middleware), `api/internal/render/**` (the
     error mapping: a round that changes route refusals is here), `api/internal/composition/**`,
     `api/internal/cmd/**` and `api/cmd/**` (the server's start-up), all non-test Go;
   - `api/testdata/**` (the fixture every e2e stack boots on);
   - the e2e stack: `justfile`, `.github/workflows/ci.yml`, `.github/actions/**`,
     `scripts/ci-install-chromium-deps.sh`.

   Everything else skips e2e: Go internals below the handlers (domain, adapters, storage), Go tests,
   scripts, seeds, other workflows, documentation, `api/go.mod` and `api/go.sum`. Their correctness is
   `just validate`'s (Go tests, lint, build), which is a required check on every PR.
3. **A pure branch update does not re-run e2e.** On a `synchronize` action CI compares the patch id of
   the PR's own diff at `github.event.before` (`BASE...BEFORE`) with its diff now (`BASE...HEAD`).
   Equal and non-empty means main was merged into the PR (or the PR was rebased) and nothing else
   moved: e2e is skipped and the PR keeps its last e2e result. A `labeled` or `unlabeled` action, an
   unreadable or zero `before`, a force-pushed `before` that is not fetched, or any difference runs the
   path rule of step 2.
4. **`ci-skip` and documentation-only** skip e2e as before. `dev-smoke` and `image` are decided apart
   (`skip_slow`) and are unchanged: documentation-only or `ci-skip`.
5. **Main always runs everything.** A push to `main` and a dispatched run never skip: Release deploys
   the commit that run gates.
6. **Budget: about 3 minutes and about 100 tests** for the whole suite on CI. The run step's 6-minute
   timeout stays, as the tripwire that fails a regression on that step. Trimming the suite to the
   budget is not this ADR's work (sibling items); the budget rule is the target they are held to.
7. **A skipped e2e counts as a success.** `e2e` stays a required check of `main`
   (`scripts/github-branch-protection.sh`): GitHub treats a job skipped by its `if` as passing, and
   `.claude/scripts/land.sh` accepts a `SKIPPED` conclusion, so a PR that skipped e2e is mergeable.

## Consequences

- A Go change below the handlers that alters what the UI shows reaches `main` without a browser
  check on its PR. The push to `main` runs the whole suite, so the first red is on `main`, before
  Release. This is the user's decision (triage, 2026-11-11); the Go tests are where such a change is
  proven.
- A pure update after a red e2e shows a green (skipped) e2e on the new head, although the PR's change
  is the one that failed. The failing run stays on the previous head. Reviewers and the supervisor
  read the last real e2e result, not only the latest check.
- An update that cancels an e2e still in progress (the concurrency group cancels it) leaves the PR
  with no completed e2e either. The push to `main` covers it.
- A `labeled` action (for example `no-release`) on a PR that touches an e2e path runs e2e again;
  step 3 reads `before` on `synchronize` only.
