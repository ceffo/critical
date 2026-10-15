# ADR 0007: HP ledger

**Status**: accepted, 2026-10-15 (critical-qki.37; collects the rules shipped in critical-qki.7 and
critical-qki.38, until then in `docs/backend.md`); steps 2, 3, 8 and 9 amended by
[ADR 0018](0018-transactions-and-undo.md)

## Context

Hit points change many times a session and the player makes mistakes: a wrong number, a damage
applied twice. The current HP must always be explainable line by line, and a mistake must be
undoable without the arithmetic drifting. The HP total therefore lives in the action log (the one
append-only log of table actions), not in a counter that code updates.

Two user rulings shape it. 2026-10-13: *"a player cannot go above its max hp. period. max hp
grows when leveling up only"*. 2026-10-14: *"no amending values. just the note.. can only pop
elements"*.

## Decision

Tests live in `api/internal/domain/service/hpsvc/` unless named otherwise.

1. **Entries.** An HP change is an `hp_change` entry of the action log with a signed `hp_delta`
   (negative for damage, positive for healing) and a source text, which is required (blank is
   refused, 400). Entries are never edited or deleted: a correction is a separate row.
   Tests: `TestAppendEvent` ("source required", "blank source", "damage", "heal").
2. **Current HP.** `hp_max` plus the SUM of the effective `hp_delta` of the log, computed in SQL
   (`HPTotal`) and never summed in application code. The effective delta of an entry is its
   original delta, replaced by the latest correction that set one, and 0 once voided. Other
   characters' entries do not count; entries of other kinds have delta 0, except an `exposure`
   the Bard takes himself ([ADR 0008](0008-spell-slots-and-long-rest.md) step 16, user ruling
   2026-10-15), whose delta is the damage rolled. (*Amended by [ADR 0018](0018-transactions-and-undo.md) step 4:* an entry of any
   kind may carry a delta, and every entry's effective delta counts.)
   Tests: `TestHPTotal` (actionlogdal: plain sum, "latest amend wins", "void counts zero", "void
   after amend counts zero", "summary-only amend keeps delta"), `TestGetState`,
   `TestExposureOnTheBard_VoidGivesTheHPBack` (composition).
3. **Damage taken is entered as final.** The app never works the damage out: the player types the
   number after the DM's rulings (armour, resistance, saves), and it is logged as typed, never
   capped (HP may go below 0) down to the floor of -10, where it is clamped
   ([ADR 0011](0011-hp-below-zero.md) step 1). The one damage the app rolls itself is a spell's
   exposure the player says the Bard takes ([ADR 0008](0008-spell-slots-and-long-rest.md)
   step 16): it is logged as rolled, under the same floor. (*Amended by [ADR 0018](0018-transactions-and-undo.md) step 4:* any
   damage the app rolls itself, whatever the entry's kind, is logged as rolled under the floor.)
   Tests: `TestAppendEvent` ("damage"), `TestRecord_CapsHeal` (actionlogsvc: "damage does not go
   through the cap"), `TestRecord_FloorsDamage` (actionlogsvc: "damage below 0 but above the
   floor is stored as is").
4. **A heal never goes above max HP.** There are no temporary HP, and max HP grows only when the
   character levels up. When a positive delta is recorded (whatever the service that records it),
   it is capped at the gap to max inside the recording transaction: the entry stores
   `min(heal, -total)`, where `total` is the SUM before the entry. The excess is dropped, never
   kept. The cap is applied when writing; the ledger is never rewritten.
   Tests: `TestCapHeal` (actionlog), `TestRecord_CapsHeal` (actionlogsvc: "heal at max-1", "heal
   exactly to max", "heal bigger than the gap", "payload kept", "total fails", "payload not an
   object"), `TestHPCap_HealAtMaxMinusOne`, `TestHPCap_HealExactlyToMax`,
   `TestHPCap_HealBiggerThanGap` (composition).
5. **A heal at full HP is logged as 0.** The event is kept with delta 0, so the log shows what was
   tried. Every capped heal carries `hpRequested` and `hpApplied` in its payload; the Combat Log
   shows both. Damage that follows is not absorbed by any hidden surplus.
   Tests: `TestRecord_CapsHeal` ("heal at full HP logs 0"), `TestWithHPBreakdown` (actionlog),
   `TestHPCap_HealAtFullHPLogsZero`.
6. **Display guard.** The HP shown is `min(hp_max, hp_max + total)`, so rows logged before the cap
   existed never show HP above max.
   Tests: `TestGetState_NeverAboveMax`.
7. **Amend sets the note only.** `POST /api/log/{id}/amend` sets the note of any entry (trimmed,
   at most 500 characters, an empty note clears it). It changes no HP, summary or payload, and
   the entry stays `active`. Amending a voided entry is refused (409). Value amends written
   before this rule still count, in step 2 (the latest one that set a delta wins).
   Tests: `TestAmend` (actionlogsvc: the cases and "too long"), `TestAmendNoteAndPop_HPTotal`
   (actionlogdal), `TestLogStack_AmendNoteOnly` (composition), `TestEntryStatusAndEffectiveValues`
   ("note only stays active"), `TestLegacyValueAmend_ReadsAfterMigration014`.
8. **Void pops the newest entry only.** Only the newest non-voided entry, of any kind, can be
   voided; an older one is refused (409, naming the entry on top). Popping restores the state
   the entry changed (its snapshots), so popping in order walks the state back one step at a time.
   A popped HP change counts 0. Popping the newest entry puts the total back to what it was
   before that entry, so a pop can never leave HP above max (step 4). (*Amended by
   [ADR 0018](0018-transactions-and-undo.md) steps 2 and 3:* the unit popped is the newest transaction, whole, and the row's
   Void is the global undo.)
   Tests: `TestVoid` (actionlogsvc: "hp change, no snapshots", "already voided", "an older entry
   is refused"), `TestAmendNoteAndPop_HPTotal`, `TestHPCap_PopHealThenDamage`,
   `TestLogStack_PopInOrderRestoresEachStep`, `TestLogStack_VoidOlderEntryIsRefused`.
9. **Undo.** `POST /api/hp/undo` pops the newest entry when it is an HP change; when another kind
   is on top it is refused (409, naming it) and nothing changes; on an empty log it does nothing.
   A heal that was logged as 0 at full HP is undone like any other. (*Amended by
   [ADR 0018](0018-transactions-and-undo.md) step 3:* `POST /api/hp/undo` is retired; the HP panel's undo is the global undo
   and pops whatever transaction is on top.)
   Tests: `TestUndoLast`, `TestVoidLatest` (actionlogsvc: "empty log", "pops the newest", "newest
   is another kind"), `TestHPUndo_NothingToUndoIsNoop` (actionlogdal),
   `TestLogStack_HPUndoOnlyPopsTheTop`, `TestHPCap_UndoHealLoggedAtFullHP`.

## Consequences

- To correct a wrong number the player pops everything above it, then re-enters. That is the
  accepted price of a stack that always walks the state back consistently.
- A heal's excess is lost for good: popping the damage that made room for it is refused while the
  heal sits above it, so the total cannot be pushed past max through the back door.
- Today the only positive deltas come from the manual HP event (cure spells and potions are typed
  there); casts, item uses and rests change no HP. Step 4 holds for any future source because it
  lives in the recording path.
- Level-up (max HP growth) is not an app action yet; `hp_max` is the character sheet's.
- Not covered: temporary HP, regeneration over time. The floor at -10 and the death and
  unconsciousness thresholds are [ADR 0011](0011-hp-below-zero.md).
