# ADR 0010: Action stack and undo

**Status**: accepted, 2026-10-15

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
there is no redo.

## Decision

Tests live in `api/internal/domain/service/actionlogsvc/undo_test.go` unless named otherwise.

1. **The stack and its top.** The log is a stack: an entry is never edited, an amend only sets its
   note, and only the newest non-voided entry (the top) can be popped. `GET /api/log` returns
   `topId`, the id of the top (`null` when nothing can be popped). It may be older than every
   listed entry. `POST /api/log/undo` answers with the popped entry, shaped as a `GET /api/log`
   entry (structured payload; the UI words it).
   Tests: `TestTopID`; `router` `TestLogEndpoints_WireShape` (`topId`),
   `TestLogEndpoints_AdjustedSaveShape` (`topId: null`), `TestLogUndo_WireShape`.
2. **Undo pops the top, any kind, and reverts its whole effect.** The pop records a void
   correction and writes back every snapshot's "before" through the restorer registered for its
   state key (or the key's family). No kind is special-cased in the undo: what each kind reverts
   is decided by its restorer and its own ADR (HP: ADR 0007; buffs: 0004; round: 0005;
   initiative: 0013; slots and preparation: 0008; inventory and weapons: 0016). A missing restorer
   fails the undo.
   Tests: `TestUndo` ("matching expected top pops it and restores its snapshots", "restorer fails:
   no void written"); `composition` `TestUndo_PopsEveryKindInOrder` (preparation, cast, buff, bow
   shot, damage, wand charge, round advance, initiative, each popped back to the state before it,
   then nothing to undo).
3. **Expected top.** The request carries `expectedTopId`, the `topId` the client read. When the
   top is another entry (an action landed since, or the expected one was already popped) the undo
   is refused (409) and nothing changes: no restore, no correction. A missing or non-positive
   `expectedTopId` is a 400. With nothing to pop the undo is refused (409).
   Tests: `TestUndo` ("stale expected top is refused and nothing changes", "expected top already
   popped is refused", "empty log"); `composition` `TestUndo_StaleExpectedTopChangesNothing` (two
   clients undoing the same top: the second is refused); `router` `TestSpecialSentinels` ("undo
   stale top", "undo empty log"), `TestRequestParsingErrors` ("undo …").
4. **One transaction per pop.** Reading the top, the expected-top check, every restore and the
   void correction run in one transaction: a failing restore leaves no void and no partial state,
   and nothing can land between the check and the pop.
   Tests: `TestUndo` (one transaction call; "restorer fails: no void written").
5. **Annuler and Ctrl/Cmd+Z.** The journal header has an Annuler button whose tooltip names the
   action on top, worded in the frontend from the entry's payload (`lib/logSummary.ts`); it is
   disabled when `topId` is null. Ctrl+Z (Cmd+Z on macOS) does the same, except while the focus
   is in a text-entry field (text-like input, textarea, select, editable element; a checkbox or
   radio does not count) and while a popover, dialog or menu is open. Both send the `topId` the
   client holds and then refresh the table state; with no `topId` nothing is sent.
   Tests: `web/src/lib/undoShortcut.test.ts`, `web/src/lib/undoButton.test.ts` (empty log, en/fr
   tooltip), `web/src/stores/undo.test.ts` (empty log, stale top from another client, reroll and
   row Void in step), `web/e2e/log.spec.ts` "Annuler and Ctrl/Cmd+Z pop the newest action".

## Consequences

- `POST /api/log/{id}/void` (the journal row's Void) and `POST /api/hp/undo` stay: both pop the
  top too, and both already check what is on top inside their transaction.
- Popping in order walks the state back one step at a time; to fix an old mistake the player pops
  everything above it and replays.
- A refused undo is shown as the server's error; the table refreshes, so the next undo targets the
  new top.
- Not covered: redo (a popped entry stays voided; the action is replayed instead), and
  corrections (attack AC, save adjustment, GM verdict) as their own undoable steps: today they are
  amends of their roll, undone only by popping the roll.
