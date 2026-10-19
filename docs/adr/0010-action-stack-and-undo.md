# ADR 0010: Action stack and undo

**Status**: accepted, 2026-10-15; amended 2026-10-15 (step 6, corrections as their own steps);
steps 1 and 2 and the Consequences amended by [ADR 0018](0018-transactions-and-undo.md); step 6 amended by [ADR 0028](0028-ability-checks.md) (ability check adjustment)

## Context

Every state change at the table (HP, buffs, the round and its initiative, ammo, spell slots and
preparation, item charges, the inventory and weapon editors) is an entry of the one append-only
action log, carrying "before" and "after" snapshots of the state it changed
([ADR 0007](0007-hp-ledger.md) steps 7 and 8). Mistakes happen fast during a fight, so the player
needs one global undo, reachable from anywhere (a button and Ctrl/Cmd+Z), that reverts the last
action whatever it was. Several clients may later look at the same table, so an undo must never
pop an action its sender did not see.

User ruling, 2026-10-14: *"no amending values. just the note.. can only pop elements"*, for every
entry kind. Triage, 2026-10-15: undo is the server's action stack, not a client snapshot, and
there is no redo. Triage, 2026-10-15 (option B): the corrections made after a roll (target AC,
save adjustment, GM verdict) become their own undoable steps, still with no redo; before that they
were amends of their roll, undone only by popping the roll.

## Decision

Tests live in `api/internal/domain/service/actionlogsvc/undo_test.go` unless named otherwise.

1. **The stack and its top.** The log is a stack: an entry is never edited, an amend only sets its
   note, and only the newest non-voided entry (the top) can be popped. `GET /api/log` returns
   `topId`, the id of the top (`null` when nothing can be popped). It may be older than every
   listed entry. `POST /api/log/undo` answers with the popped entry, shaped as a `GET /api/log`
   entry (structured payload; the UI words it). (*Amended by [ADR 0018](0018-transactions-and-undo.md) steps 1 and 2:* the
   unit popped is the transaction of the top entry, whole; `topId` still names the newest entry
   and the undo answers with the transaction's first entry.)
   Tests: `TestTopID`; `router` `TestLogEndpoints_WireShape` (`topId`),
   `TestLogEndpoints_AdjustedSaveShape` (`topId: null`), `TestLogUndo_WireShape`.
2. **Undo pops the top, any kind, and reverts its whole effect.** The pop records a void
   correction and writes back every snapshot's "before" through the restorer registered for its
   state key (or the key's family). No kind is special-cased in the undo: what each kind reverts
   is decided by its restorer and its own ADR (HP: ADR 0007; buffs: 0004; round: 0005;
   initiative: 0013; slots and preparation: 0008; inventory and weapons: 0016). A missing restorer
   fails the undo. (*Amended by [ADR 0018](0018-transactions-and-undo.md) step 2:* every non-voided entry of the top
   transaction is popped this way, newest first, in the one transaction of step 4.)
   Tests: `TestUndo` ("matching expected top pops it and restores its snapshots", "restorer fails:
   no void written"); `composition` `TestUndo_EveryStateKeyIsRestoredThroughItsServicesStorage` (each state key a service
   snapshots has its restorer registered and writes its "before" through the service's storage
   port: buffs, round, saved buffs, initiative, spell slots, items, weapons, XP).
3. **Expected top.** The request carries `expectedTopId`, the `topId` the client read. When the
   top is another entry (an action landed since, or the expected one was already popped) the undo
   is refused (409) and nothing changes: no restore, no correction. A missing or non-positive
   `expectedTopId` is a 400. With nothing to pop the undo is refused (409).
   Tests: `TestUndo` ("stale expected top is refused and nothing changes", "expected top already
   popped is refused", "empty log"); `router` `TestSpecialSentinels` ("undo
   stale top", "undo empty log"), `TestRequestParsingErrors` ("undo …").
4. **One transaction per pop.** Reading the top, the expected-top check, every restore and the
   void correction run in one transaction: a failing restore leaves no void and no partial state,
   and nothing can land between the check and the pop.
   Tests: `TestUndo` (one transaction call; "restorer fails: no void written").
5. **Annuler and Ctrl/Cmd+Z.** The journal header has an Annuler button whose tooltip names the
   action on top, worded in the frontend from the entry's payload (its kind's view in `web/src/journal/kinds/`); it is
   disabled when `topId` is null. Ctrl+Z (Cmd+Z on macOS) does the same, except while the focus
   is in a text-entry field (text-like input, textarea, select, editable element; a checkbox or
   radio does not count) and while a popover, dialog or menu is open. Both send the `topId` the
   client holds and then refresh the table state; with no `topId` nothing is sent.
   Tests: `web/src/lib/undoShortcut.test.ts`, `web/src/lib/undoButton.test.ts` (empty log, en/fr
   tooltip), `web/src/stores/undo.test.ts` (empty log, stale top from another client, reroll and
   row Void in step), `web/e2e/log.spec.ts` "Annuler and Ctrl/Cmd+Z pop the newest action".
6. **A correction is its own step.** A correction after the roll (target AC, ADR 0002 step 11
   and ADR 0012 step 5; save adjustment, ADR 0006 step 5; GM verdict, ADR 0012 step 4; skill check
   adjustment, ADR 0014 step 7; ability check adjustment, ADR 0028 step 7) records,
   in the transaction that checks it may be made (ADR 0006 step 6), two rows: a `correction`
   entry on the stack (stamped with the current round, no snapshot), whose payload names the
   change (`{entryId, entryKind, field, from, to}`; `field` is `targetAc`, `saveAdjustment`,
   `skillAdjustment`, `abilityAdjustment` or `verdict`; `from` is the value before this correction, null for an unknown AC or no
   verdict), and the amend carrying the corrected entry's new payload, linked to it (`stepId`).
   It is one generic step for every kind of correction: the correcting service only computes
   the payload and names the field. A `correction` entry is not one of the roll kinds.
   - **Popping it** (undo, or the row's Void) is an ordinary pop (steps 2 to 4): it voids the
     correction entry, which puts its amend out of effect (`undone` on the wire). The corrected
     entry then reads the payload, status and HP delta of the newest amend still in effect, or
     its original ones: correcting twice and undoing twice gives the roll as rolled, still on
     the log. A correction changes no other state, so there is nothing else to restore.
   - **Order.** A correction is newer than its roll, so undo pops the newest correction first,
     then the one before, then the roll; the expected top (step 3) is the correction when it is
     the newest entry. The roll is still the latest roll after its corrections are popped, so it
     can be corrected again.
   - **Older amends.** Payload amends written before this step have no `stepId`: they stay in
     effect and are undone only by popping their roll, as before.
   - **Tooltip.** The Annuler tooltip names the correction from its payload, worded in the
     frontend ("Annuler : Correction CA 5 → 7").

   Tests: `actionlogsvc` `TestAmendLatest_RecordsTheCorrectionStep` (step then amend, one
   transaction, nothing written when a write fails), `TestEntry_PoppedStepAmendIsOutOfEffect`;
   `actionlogdal` `TestCorrectionStep_PoppedStepPutsItsAmendOutOfEffect`; SQLite migration 023 kept the
   corrections without a step (a past migration; the per-migration test was deleted with the squash into the PostgreSQL baseline, [ADR 0025](0025-postgresql-storage.md) step 2.3); `combatsvc` `TestAdjustAttackAC_Recompute`,
   `TestAdjustAttackAC_PreChangeEntryResettles`, `TestAdjustSave_Recompute`,
   `TestUnknownAC_Step4_VerdictSettlesWithoutCrit`, `TestUnknownAC_Step6_SameDiceAsKnownAC`; `router`
   `TestLogEndpoints_CorrectionStepShape`; `web/src/lib/undoButton.test.ts` (the correction
   tooltip, en/fr); `web/src/stores/undo.test.ts` ("a pop drops this session's live roll");
   `web/src/journal/card.test.ts` ("a popped correction under the Rolls filter");
   `web/e2e/attack.spec.ts` "the target AC is corrected on the card after the roll" (Annuler
   pops the corrections back to the rolled AC, then the attack is voided).

## Consequences

- `POST /api/log/{id}/void` (the journal row's Void) and `POST /api/hp/undo` stay: both pop the
  top too, and both already check what is on top inside their transaction. (*Amended by
  [ADR 0018](0018-transactions-and-undo.md) step 3:* both are retired; the journal's Undo is the only control and sends the
  global undo with `expectedTopId`.)
- Popping in order walks the state back one step at a time; to fix an old mistake the player pops
  everything above it and replays.
- A refused undo is shown as the server's error; the table refreshes, so the next undo targets the
  new top.
- A correction shows in the journal as its own line ("Correction CA 5 → 7"); the roll's line
  reads its corrected payload. Popping the correction brings the previous reading back.
- Migration 023 adds `action_corrections.step_id`; rows written before keep NULL (step 6, older
  amends).
- Not covered: redo (a popped entry stays voided; the action is replayed instead). (*Amended by
  [ADR 0018](0018-transactions-and-undo.md) step 6:* redo is reserved for a later ADR, which replays a popped transaction.
  *Amended by [ADR 0019](0019-redo.md):* redo replays the newest popped transaction.)
