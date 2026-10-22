# ADR 0041: The ruleset: which book data and which house rules a game is played with

**Status**: accepted 2026-10-21 (the user's triage of 2026-10-21; the details beyond the triage
accepted 2026-10-21 by the supervisor and the user). Not implemented. Amends
[ADR 0001](0001-critical-hits.md) step 4, [ADR 0030](0030-classes-and-races.md) step 21 and
[ADR 0037](0037-proficiencies.md) step 20.6, each in place and dated 2026-10-21. Builds on
[ADR 0036](0036-vocabulary.md) (the terms) and `docs/domain-model.md` ("Ruleset and house rules").
The multi-class and dual-class rules that read the lift of step 2.2 are for a later ADR.

## Context

The table plays the books with house rules. Today they are scattered: the crit table is a global
table (`crit_table_rows`, `/api/crit-table`) edited inside the character's Attack panel; the
Tumbling success buff is a committed Go table in `ruletablesvc` (ADR 0037 step 20.6); the level
limits are catalogue data read in one place (ADR 0030 step 21); and "we lifted the racial
restriction for multi-class and dual-class" has no home at all. A rule that the table decided is a
property of the table's game, not of a character and not of the code.

User, 2026-10-21: "we need an entity to represent a ruleset. currently you stored the critical
damage table in the character which is very wrong. later when we will have an entity to represent
a game or campaign, that has sessions, it will have both a ruleset and a party (collection of
characters)." And: "we lifted racial restriction for multi and dual class in our game."

## Decision

1. **The Ruleset entity.** A ruleset says which book data and which house rules a game is played
   with. It has an `id`, a `name`, the `catalogue version` it reads (recorded now, not enforced:
   there is one catalogue) and its house rules (step 2). Now there is exactly one row, the
   **default**, created by the migration that introduces the table; every character plays under
   it. Test needed: a fresh database has one ruleset named as the default, and the catalogue
   version it names is the one the server loaded.
2. **The house rules are a typed record, not free keys.** One field per rule, each with a Go type
   in the domain and a column or child table in storage; a rule is added by a code change and a
   migration, never by an open key/value bag (so a typo is a compile error and the editor is
   generated from the type). The record holds:
   1. **The crit table**: the rows of [ADR 0001](0001-critical-hits.md) step 4 (a d100 range and a
      multiplier, decimals allowed), as child rows of the ruleset.
   2. **The racial lift**: `multiclass_any_race` and `dualclass_any_race`, two booleans, both true
      in the default (user, 2026-10-21). They say whether the race's class restriction
      (ADR 0030 step 21's dash) is waived for a multi-class or dual-class character. They do not
      change a single-class character's check. What a multi-class or dual-class character is, is
      for a later ADR; this one only gives them their switch.
   3. **The level limits mode**: `level_limits`, an enum with one value today, `warn`: a level
      above the race's limit is a warning on the character (code, class, level, limit), never a
      refusal and never a cap (user, 2026-10-21). The enum leaves room for `enforce` without
      deciding it.
   4. **The success effects**: for a proficiency id, the buff a successful check puts on the
      character (the effects, the rounds). Child rows of the ruleset keyed by proficiency id.
      Tumbling is its one row in the default (ADR 0037 step 20).
   Tests needed, one per rule: the default ruleset's value of each (crit table equals today's
   table, both lifts true, `warn`, the Tumbling row), and that the domain refuses a record with
   an overlapping or gapped crit range, a multiplier below zero, a success effect for an unknown
   proficiency id or with zero rounds.
3. **A character resolves its ruleset through one port.** The domain defines
   `RulesetResolver` (character -> ruleset); the one implementation returns the default. The
   rules services take the ruleset from the resolver, never a constant, never a package-level
   table. No `ruleset_id` is stored on the character today. When games arrive, the resolver reads
   the character's game's ruleset and the default shortcut goes (step 9). Test needed: a rules
   service tested with a resolver mock that returns a non-default ruleset (a different crit table,
   a different success effect) gives a different outcome, so no service reads the default
   directly.
4. **Rules code reads the ruleset.** The crit table (ADR 0001 step 4), the success effects (ADR
   0037 step 20), the level limit mode and the racial lift are read from the resolved ruleset at
   the moment of the rule. A logged outcome keeps what it used (the crit row is stored at the roll,
   ADR 0001 step 4 and its Consequences), so editing the ruleset never rewrites history and a
   correction does not re-read it. Test needed: edit the crit table after an attack, correct the
   attack's target AC, and the multiplier is still the stored row's.
5. **Level limits warn.** Amends [ADR 0030](0030-classes-and-races.md) step 21 (2026-10-21): "shown,
   not enforced" becomes "the ruleset's `level_limits` is `warn`: shown as a warning, never a
   refusal". The creation refusal of a class the race does not admit stays for a single-class
   character; step 2.2 lifts it for multi-class and dual-class. The limits and the dashes themselves
   are book data, rows keyed by race and class beside the race definitions of
   [ADR 0040](0040-classes-are-data.md) step 2 (cross-reference added 2026-10-21); the ruleset only
   says how they apply. Test needed: with `warn`, a
   level-up past the race's limit succeeds and the character's warnings list the limit.
6. **Success effects leave the code.** Amends [ADR 0037](0037-proficiencies.md) step 20.6
   (2026-10-21): the "committed rules data (`ruletablesvc`)" is the ruleset's success effects. The
   behaviour of steps 20.1 to 20.8 is unchanged. Test needed: with the Tumbling row removed from
   the ruleset a successful Tumbling check adds no buff; with it, the buff of step 20.1; no code
   names Tumbling.
7. **Who edits, and the audit.** Only an admin edits a ruleset
   ([ADR 0029](0029-users-login-and-access.md) step 1; a player reads). An edit is not a
   character action: it is not on any action stack, has no undo and appears in no character's
   journal, because it changes the table's rules, not a character's state. It is audited instead:
   an append-only `ruleset_changes` record (who, when, which rule, the value before and after),
   written in the same transaction as the edit, shown on the Ruleset screen. Tests needed: a
   player's edit is refused with the central 403; an admin's edit writes one change record with the
   before and after; a failed edit writes none.
8. **API and screen.** `GET /api/ruleset` (any logged-in user) returns the default ruleset, house
   rules included, as structured data ([ADR 0023](0023-localized-text.md): no per-language
   fields; names of proficiencies and effects are keys the UI words). Edits are admin-only
   requests under `/api/ruleset/`, one per rule (crit table, the two lifts, the success effects;
   the level limits mode is read-only while it has one value). `/api/crit-table` is removed outright, with
   no alias, and its callers move in the same change. The crit table editor leaves the Attack panel for a
   **Ruleset** screen in the cog menu, visible to admins to edit and to players to read (read-only
   for players); the Attack panel keeps only the roll. New refusals get a row in
   `internal/render` and an `apiError.<code>` key in both locales. Test needed: an e2e-free API
   test per refusal; one Playwright flow (an admin edits a crit row on the Ruleset screen and the
   next attack uses it), the rest in Go.
9. **Games later.** A game (future) has one ruleset and a party; a character reads the ruleset of
   its game through the resolver (step 3). Until then every character plays under the default, and
   this is the only "everyone uses the default" shortcut: it lives in the resolver and nowhere
   else. A game that wants other rules points at another ruleset row; rulesets are not edited per
   game.
10. **Migration of what exists.** One migration creates the ruleset tables, the default row, copies
    the existing `crit_table_rows` into the default's crit rows (preserving an edited table, and
    its `CritPlaceholder` flag when it is still the default table) and seeds the Tumbling success
    effect. It is a new file, safe on existing data. Test needed (in Go, no database, per
    [ADR 0025](0025-postgresql-storage.md) step 6): the seed loader is idempotent and the default
    ruleset's values are those of steps 2 and 6.

## Consequences

- [ADR 0001](0001-critical-hits.md) step 4: "the house crit-table row (`crit_table_rows`, edited in
  the app)" reads "the row of the ruleset's crit table (ADR 0041 step 2.1)". Amended in place,
  2026-10-21.
- [ADR 0030](0030-classes-and-races.md) step 21: amended in place, 2026-10-21 (step 5 above).
- [ADR 0040](0040-classes-are-data.md) (classes and races as catalogue data) is the book data a
  ruleset reads: a house rule is a setting here, never an edit of a class or race definition
  (ADR 0040 step 10).
- [ADR 0037](0037-proficiencies.md) step 20.6: amended in place, 2026-10-21 (step 6 above).
- The glossary has **Ruleset**, **house rule** and **house rule setting**; `docs/domain-model.md`
  draws the ruleset with its settings.
- Implementation is risky (a migration, a changed endpoint, rules math reading a new source) and
  is reviewed; this ADR is docs only.
- Out of scope: the Game, Campaign, Session and Party entities; the multi-class and dual-class
  rules themselves; a second ruleset in the UI (there is one row, so no selector).
