# ADR 0039: The in-app changelog: deterministic, from the commit titles, English only

**Status**: accepted 2026-10-20 (the user's rulings in triage, CRI-126). Amends
[ADR 0024](0024-deployment.md) step 1.4 (the build also carries the changelog) and states an exception to
[ADR 0023](0023-localized-text.md).

## Context

User, 2026-10-20: _"is it possible to automatically extract the changelog and have it grouped per release
version and then per change type (fix/feat) in order to show, in the app, the changelog list when the user
clicks on the version number. i want this process to be deterministic, no llm involved. we should only list
fix and feat information, strip the ticket number and author, even the context inside the parenthesis. so,
for example, "feat(web): added twelve new themes (CRI-1234) by ceffo" becomes a bullet point "Added twelve new
themes" in the Features of that version"_.

[ADR 0024](0024-deployment.md) gives every staging release a version computed from the conventional-commit
titles of the squash commits on main (the PR titles) since the last version tag. Nothing records what a
version contains besides GitHub's generated release notes. This ADR fixes the rules; the generator and the
endpoint (CRI-128) and the panel (CRI-129) follow it.

## Decision

1. **The source and the method are deterministic.**
   1. The source is the conventional-commit title of each squash commit on main, per version tag, read from
      the same range (`last tag..commit`) the release uses for its bump. No LLM, no network call and no
      hand-written text takes part.
   2. The same range gives the same changelog: the generator is a committed, rerunnable script with a
      table-driven test (the "No throwaway code" rule).
2. **Only `feat` and `fix` are listed.**
   1. A `feat` is a Feature and a `fix` a Fix. Every other type (`docs`, `test`, `chore`, `refactor`,
      `perf`, a non-conventional title) is never listed.
   2. `feat!` stays a Feature. Breaking-change markers are not shown.
3. **A title is cleaned by a fixed rule**, in this order:
   1. Drop the `type(scope)!:` prefix, the scope included.
   2. Drop every trailing parenthesised group: the ticket `(CRI-n)`, the pull request `(#n)`, and any other
      `(...)`. The author is not in a title and nothing of an author is kept.
   3. Trim the spaces, then capitalise the first letter. Nothing else is rewritten.
   4. Example: `feat(web): added twelve new themes (CRI-1234) by ceffo` gives the Feature
      `Added twelve new themes`.
4. **The shape: versions, newest first, then Features and Fixes.**
   1. Each version has `features[]` and `fixes[]`, lists of strings, in the order of the commits.
   2. A version with no `feat` and no `fix` is still listed, as a version with both lists empty; the panel
      shows a "no user-facing changes" line for it.
5. **English only, an exception to ADR 0023.** The changelog is a build artifact generated from commit
   titles, not domain text, so it has no `localized_texts` rows and no language map: its strings are
   English, served as they are. The panel's own chrome (title, headings, the empty-version line, the loading
   and error states) is translated (en + fr) like any other screen. A French changelog is out of scope.
6. **The build carries its changelog.**
   1. The changelog is made at Release (staging), next to the version it describes, and baked into the image
      with that version ([ADR 0024](0024-deployment.md) step 1.4), so a version promoted later carries its
      own.
   2. A build lists the versions up to its own, never newer ones.
   3. The server reads the baked file and does no git work at run time.
7. **The panel (CRI-129) follows these rules.**
   1. It opens from the version badge, covers about two thirds of the canvas and lists the newest version
      first.
   2. It loads by infinite scroll: the endpoint is paginated by version; the page size is the implementing
      item's choice.
   3. Layout details (the rows, the headings) follow [ADR 0022](0022-activity-focus-layout.md) and
      [ADR 0027](0027-space-follows-frequency-of-use.md); nothing in the panel is cut.

## Consequences

- A title that reads badly gives a bad line: the changelog is only as good as the PR titles, which
  `pr-title.yml` already checks for format. Rewording one means rewording the PR title before merge.
- A `fix` or `feat` whose only user-facing effect is invisible is still listed; the type decides, not a
  judgment of relevance.
- English-only means the panel mixes the player's language (chrome) with English (lines). Accepted by the
  user. A later language would need generation from a source other than commit titles, a new decision.
- The image grows by one small file per build.
- Not covered: the new-version toast that tells a user a newer version is deployed (a separate item).

## Tests

Steps 1 to 4 are proved by the generator's script test, a table of titles and a fixture repository, in the
style of `scripts/test-release-version.sh`, written by the generator's item (CRI-128); it is named here by
file once it exists and is not a Go `Test*` name, so the ADR-test check has nothing to resolve yet. Step 6
is proved by the CI `image` job (`scripts/image-smoke.sh` checks the baked changelog beside the version) and
a Go test of the endpoint's handler (CRI-128); step 5's chrome strings by the web's i18n parity test; step 7
by the panel's Playwright spec and unit tests (CRI-129).
