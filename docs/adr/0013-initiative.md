# ADR 0013: Initiative

**Status**: accepted, 2026-10-15 (critical-kiv.4); steps 2 and 4 amended by [ADR 0018](0018-transactions-and-undo.md)

_Amended by [ADR 0033](0033-equipment.md) (equipment, accepted 2026-10-18), on implementation: step 3 and Consequences (the initiative adds the declared action's weapon speed, casting time or item modifier, initiative effects and a situational modifier; the declaration is logged, ADR 0033 steps 15 and 16); then amended by [ADR 0035](0035-equipment-screens.md) item 6 (no declaration: d10, the weapon in hand's speed and effects; a spell cast or an item used corrects that round's initiative, logged), built 2026-10-19 (CRI-39; contract, storage and tests in ADR 0035 item 6's as-built note). The value served is the total; the d10 and the modifiers are its breakdown._

## Context

Every combat round starts with an initiative roll at the table. The player wants it rolled for
them on Next Round, shown in the turn header, logged with everything else, and undoable or
rerollable when the table rolls again. Triage (2026-10-15) settled it as a plain 1d10, no
modifiers: "speed will come later, not in scope". The round counter and Next Round are
[ADR 0005](0005-rounds-and-time.md); the log is a stack ([ADR 0007](0007-hp-ledger.md) step 8).

## Decision

Tests live in `api/internal/domain/service/buffsvc/initiative_test.go` unless named otherwise.

1. **Rolled on Next Round.** Every Next Round rolls exactly one d10 through the dice port, on the
   server, in the same transaction as the round advance.
   Test: `TestInitiative_Step1_NextRoundRollsOneD10` (one `RollDie(10)` and no other roll, the
   payload and the stored value carry that die).
2. **Its own log entry, after the advance.** The roll is an `initiative` entry recorded right
   after the `round_advance` entry, stamped with the new round. Its payload is `{round, d10}`; the
   UI words it (no summary). Its snapshot (`initiative`, before `null`) restores "no initiative".
   The advance ends the previous round's initiative and snapshots it. So undo after Next Round
   pops the initiative first (the turn has none), a second undo pops the advance (the previous
   round and its initiative come back). (*Amended by [ADR 0018](0018-transactions-and-undo.md) step 2:* the advance and its
   initiative are one transaction, so one undo after Next Round brings back the previous round
   and its initiative.)
   Tests: `TestInitiative_Step2_OwnEntryAfterTheAdvance` (order of the entries and writes, the
   snapshots, one transaction); `actionlogsvc` `TestUndo_PopsTheWholeTransaction`
   (ADR 0018 step 2: one undo pops the advance and its initiative).
3. **Lowest acts first, display only.** The initiative is the plain die: no modifier from buffs,
   abilities or weapon speed. The app orders nothing; the turn header shows the value with the hint
   "1d10, the lowest acts first".
   (*Amended by [ADR 0033](0033-equipment.md) step 15:* the die gets the declared
   action's modifier (weapon speed, casting time or magic item), initiative effects and a situational
   modifier, each in the entry's breakdown.)
   Test: `TestInitiative_Step3_PlainD10NoModifier` (faces 1, 6 and 10 stored as rolled, with a
   to-hit and AC buff active).
4. **Reroll replaces the newest initiative.** A reroll (`POST /api/round/initiative/reroll`) is
   one server call and one transaction: it pops the round's initiative entry, which must be the
   newest non-voided entry, and records a new roll as in steps 1 and 2. When another entry is
   newer it is refused (409) and nothing is rolled; with no initiative it is refused (409) too.
   (*Amended by [ADR 0018](0018-transactions-and-undo.md) step 5:* the reroll voids nothing; it is its own transaction whose
   snapshot keeps the replaced roll, so undo after a reroll gives that roll back.)
   Tests: `TestInitiative_Step4_RerollReplacesTheNewestInitiative` (no void, exactly one d10, the
   replaced roll as "before"; the refusals), `TestInitiativeRestorer` (undo writes the replaced roll
   back); `router` `TestRoundInitiative_WireShape`.
5. **None out of combat.** At round 0 there is no initiative: End combat, Set round to 0 and
   passing time end it, in their own entry's snapshot, so voiding that entry brings it back. Set
   round to another round keeps it. There is nothing to reroll at round 0. `GET /api/round`
   returns `initiative: null` then.
   Tests: `TestInitiative_Step5_NoneOutOfCombat`.

## Consequences

- The initiative is stored next to the round (`round_state.initiative_d10`,
  `initiative_entry_id`) and restored through the `initiative` snapshot like the round itself; the
  entry id lets the reroll pop exactly that entry. (*Amended by [ADR 0018](0018-transactions-and-undo.md) step 5:* the reroll
  pops nothing; the entry id tells which transaction recorded the current initiative.)
- A reroll followed by an undo leaves the turn without an initiative (the first roll was popped
  by the reroll); Next Round rolls a new one. (*Amended by [ADR 0018](0018-transactions-and-undo.md) step 5:* undo after a
  reroll gives back the roll it replaced.)
- The initiative entry is not a roll kind (`actionlog.RollKinds`): it does not replace the
  result card and does not block adjusting the latest attack or save.
- Not covered: weapon speed, casting time, Dexterity or reaction modifiers, group versus
  individual initiative, the order of the other combatants.
