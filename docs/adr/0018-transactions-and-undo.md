# ADR 0018: Every mutation is a transaction; undo pops the last one

**Status**: accepted, 2026-10-15. Amends [ADR 0007](0007-hp-ledger.md) steps 2, 3, 8 and 9,
[ADR 0010](0010-action-stack-and-undo.md) steps 1 and 2 and its Consequences,
[ADR 0013](0013-initiative.md) steps 2 and 4, [ADR 0015](0015-experience.md) step 4 (step 7).

## Context

The action stack ([ADR 0010](0010-action-stack-and-undo.md)) pops one log entry at a time, which
was a defence against undoing out of order. It leaves gaps: Next Round writes two entries (the
advance and its initiative), so one gesture needs two undos; the HP panel's undo
(`POST /api/hp/undo`) pops only an HP change, so an exposure the Bard took cannot be undone from
there; the journal row's Void (`POST /api/log/{id}/void`) pops without the expected-top guard. And
[ADR 0007](0007-hp-ledger.md) says only an HP change (and, as an exception, the Bard's exposure)
carries an HP delta, while the recording path already caps and floors any entry's delta.

User, 2026-10-15: *"adr 7 was trying to be defensive against out of order undos. the new rule
we're going towards is: every mutation is transactional, we can reverse and pop last transaction
(undo system) eventually support redo by replaying popped transaction"*. Triage, the same day: the
exposure exception stays and is generalised: any transaction may change HP, through the same
recording path.

## Decision

Tests are to be added with the step they cite, by the items implementing it, unless they already
exist (named "existing"). Paths are under `api/internal/` unless they start with `web/`.

1. **A transaction.** Everything one state-changing request writes to the action log, in its one
   DB transaction, is one transaction: one or more entries, each with its snapshots. Every entry
   carries a transaction id, the id of the transaction's first entry; an entry written before
   this rule is its own transaction. A request that changes no table state writes no transaction.
   A correction step ([ADR 0010](0010-action-stack-and-undo.md) step 6) is a one-entry
   transaction (its amend is not an entry). The undo itself is not a transaction: it writes void
   corrections (step 2). Every `GET /api/log` entry exposes its transaction id (`transactionId`).
   The grouping is generic, never per kind: every service runs its request in one `InTx` of the
   one runner the composition root hands out (`actionlogsvc.Transactions`), and every entry
   recorded inside its outermost `InTx` joins the transaction its first entry started
   (`action_log.transaction_id`).
   Tests: `domain/service/actionlogsvc` `TestRecord_EntriesOfOneRequestShareATransaction`;
   `storage/sqlite` `TestMigrate025_EachExistingEntryIsItsOwnTransaction` (on existing rows);
   `rest/router` `TestLogEndpoints_WireShape` (the transaction id).
2. **Undo pops the newest transaction whole.** `topId` (ADR 0010 step 1) still names the newest
   non-voided entry. The undo pops every non-voided entry of that entry's transaction, newest
   first: for each, every snapshot's "before" is written back through its restorer, then a void
   correction is recorded. Reading the top, the expected-top check (ADR 0010 step 3, unchanged:
   `expectedTopId` must equal `topId`), every restore and every void run in one DB transaction: a
   failing restore leaves no void and no partial state (ADR 0010 step 4). The undo answers with
   the transaction's first entry, shaped as a `GET /api/log` entry; the Annuler tooltip names it
   (the Next Round, not its initiative).
   Tests: `domain/service/actionlogsvc` `TestUndo_PopsTheWholeTransaction` ("newest first",
   "restorer fails mid-transaction: no void, no partial state", "stale expected top changes
   nothing"); `adapters/actionlogdal` `TestUndo_RestorerFailsMidTransaction_RollsBackEverything`
   (the rollback on SQLite); `composition` `TestNextRound_OneUndoRestoresRoundBuffsAndInitiative`,
   `TestUndo_PopsEveryKindInOrder` (existing, updated: Next Round pops in one step),
   `TestUndo_OneUndoWalksEveryMutationFullyBack` (a table test over every table-state mutation,
   ADR 0010 Context, asserting one undo walks the state fully back);
   `web/src/lib/undoButton.test.ts` (the tooltip names the transaction, en/fr).
3. **One undo system, on the journal.** There is one way to pop: `POST /api/log/undo` with
   `expectedTopId`. `POST /api/hp/undo` and `POST /api/log/{id}/void` are retired. The one undo
   control lives in the journal (its header: the Undo button, "Annuler" in French, and Ctrl/Cmd+Z);
   the HP panel has none and the journal rows have no per-row button. Its tooltip names whatever is
   on top, an exposure or a cast as well as an HP change. It sends the top it last read as
   `expectedTopId`; a stale top is refused (409) and nothing changes. Users see "Undo" /
   "Annuler" and "Undone" / "Annulé", never "void"; void stays the storage word (step 6).
   Tests: `composition` `TestOneUndo_JournalPopsWhateverIsOnTop` (an exposure on top: HP comes
   back; a cast on top: the slot comes back); `rest/router` `TestRetiredUndoEndpointsAreGone` (the
   two retired endpoints answer 404 or 405); `web/src/stores/undo.test.ts` ("journal Undo with
   exposure on top", "journal Undo with cast on top", "journal Undo with a stale top");
   `web/e2e/cast.spec.ts` "cast, heal, cast: the journal Undo pops the cast on top, then the heal".
4. **Any transaction may change HP.** Any entry, of any kind, may carry an HP delta; current HP is
   `hp_max` plus the SUM of the effective delta of every entry, whatever its kind (ADR 0007
   step 2). The heal cap (ADR 0007 steps 4 and 5) and the floor ([ADR 0011](0011-hp-below-zero.md)
   step 1) apply in the recording path whatever the kind, and a capped or floored entry carries
   `hpRequested` and `hpApplied`, so the applied delta is what the log shows. Damage the player
   types is final (ADR 0007 step 3); damage the app rolls itself (today: an exposure the Bard
   takes, [ADR 0008](0008-spell-slots-and-long-rest.md) step 16) is logged as rolled. Popping the
   transaction gives the HP back.
   Tests: `domain/service/actionlogsvc` `TestRecord_CapsHeal`, `TestRecord_FloorsDamage`,
   `composition` `TestExposureOnTheBard_VoidGivesTheHPBack`,
   `domain/service/actionlogsvc` `TestRecord_HPDeltaOnAnyKind` (a heal capped and a damage
   floored on an entry that is not an `hp_change`), `adapters/actionlogdal` `TestHPTotal` ("every
   kind counts").
5. **Initiative reroll is its own transaction.** A reroll ([ADR 0013](0013-initiative.md) step 4)
   no longer voids the initiative it replaces: it records a new `initiative` entry, in its own
   transaction, whose `initiative` snapshot has the replaced roll as "before". It is allowed when
   the newest transaction is the one that recorded the round's current initiative (the Next Round
   or a previous reroll), otherwise refused (409, `initiative_not_on_top`) and nothing is rolled;
   with no initiative it is refused (409, `no_initiative`). The initiative entry is the last one
   its transaction records, so that transaction is the newest exactly when the initiative's entry
   is the newest non-voided entry. Undo right after a reroll gives back the roll it replaced; undo again pops the
   Next Round (the previous round and its initiative come back).
   Tests: `domain/service/buffsvc` `TestInitiative_Step4_RerollReplacesTheNewestInitiative`
   (existing, updated: no void, the "before"); `composition`
   `TestInitiative_RerollThenUndoThenUndo`.
6. **Redo stays possible.** A popped transaction stays on the log, voided: its entries, payloads
   and snapshots ("before" and "after") are kept unchanged, and a void is a correction row, never
   a delete or a rewrite. No undo path may drop what a later replay of the transaction needs.
   Redo itself (replaying the newest popped transaction) is a later ADR. (*Amended by
   [ADR 0019](0019-redo.md):* redo replays it; a popped entry can be redone and popped again, each
   time with a new correction row.)
   Tests: `adapters/actionlogdal` `TestPop_KeepsEntriesAndSnapshots` (after a pop the
   transaction's entries, payloads and both snapshots read back unchanged, each voided).
7. **What joins the stack: gameplay state, not metadata.** Every mutation of state that affects
   gameplay is a transaction (step 1) and is popped by the journal's Undo (step 3); player
   metadata and preferences are not. User, 2026-10-15: *"editing player metadata is not in the
   undo pipeline. gaining xp is. anything that mutates the state that affects gameplay"*.
   - Joins: HP, buffs and saved buffs, the round and its initiative, spell slots and
     preparation, casts and exposures, attacks, saves, skill checks, free dice rolls and the
     corrections after a roll, items (use, restock, editor), weapons (editor), long rest, and the
     XP ledger: adding, editing and deleting an award are each one transaction, and an edit keeps
     the award before and after in its snapshot ([ADR 0015](0015-experience.md) step 4).
   - Stays outside (writes no entry, never undone): character details (race, alignment, age,
     languages…), from the ruling. As a default, not yet confirmed by the user: dice favourites,
     the saved attack form (the composer's last choices), note amends on log entries (ADR 0010
     step 2) and the crit table (a rules setting, [ADR 0001](0001-critical-hits.md)).
   A new state-changing request joins the stack unless it is listed as outside here.
   Tests: `composition` `TestUndo_OneUndoWalksEveryMutationFullyBack` (every mutation that
   joins, the XP verbs included, `xp_entries` compared), `TestXPUndo_EachVerbGivesTheLedgerBack`
   (add, edit and delete: the ledger, its total and progress come back);
   `domain/service/xpsvc` `TestAdd_RecordsOneTransactionWithTheAwardsSnapshot`,
   `TestUpdate_AppliesOnlyTheSetFieldsInOneTransaction` (before and after),
   `TestDelete_RecordsTheRemovedAwardOrReportsNotFound`, `TestRestorer_WritesTheSnapshotBack`;
   `adapters/xpdal` `TestReinsert_KeepsIDAndDate`.

## Consequences

- Within a transaction the order of the entries matters only for the restores: newest first, so a
  state two entries both snapshot comes back to the oldest "before".
- To correct an old mistake the player still pops every transaction above it and replays; the
  unit is now the gesture, not the row.
- ADR 0010 step 6 is unchanged: a correction is its own one-entry transaction, popped before its
  roll.
- Undoing an XP delete writes the award back under its id and date, so an undo of an older edit
  of the same award still finds it.
- Not covered: redo (step 6 only keeps it possible; *amended:* [ADR 0019](0019-redo.md)), multi-transaction undo, undo of a request
  that wrote nothing.
