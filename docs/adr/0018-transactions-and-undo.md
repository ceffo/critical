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
   SQLite migration 025 made each existing entry its own transaction (a past migration; the per-migration test was deleted with the squash into the PostgreSQL baseline, [ADR 0025](0025-postgresql-storage.md) step 2.3);
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
   nothing"); `composition` `TestPostgresStorage_EveryAdapterRunsOnTheCallersTransaction` (every adapter
   writes in the transaction the undo runs in; the rollback itself is the database's,
   [ADR 0025](0025-postgresql-storage.md) step 7); `composition` `TestUndo_EveryStateKeyIsRestoredThroughItsServicesStorage` (every snapshot key has its
   restorer); `domain/service/buffsvc` `TestInitiative_Step2_OwnEntryAfterTheAdvance` (Next Round and
   its initiative are one transaction);
   `web/src/lib/undoButton.test.ts` (the tooltip names the transaction, en/fr).
3. **One undo system, on the journal.** There is one way to pop: `POST /api/log/undo` with
   `expectedTopId`. `POST /api/hp/undo` and `POST /api/log/{id}/void` are retired. The one undo
   control lives in the journal (its header: the Undo button, "Annuler" in French, and Ctrl/Cmd+Z);
   the HP panel has none and the journal rows have no per-row button. Its tooltip names whatever is
   on top, an exposure or a cast as well as an HP change. It sends the top it last read as
   `expectedTopId`; a stale top is refused (409) and nothing changes. Users see "Undo" /
   "Annuler" and "Undone" / "Annulé", never "void"; void stays the storage word (step 6).
   Tests: `domain/service/actionlogsvc` `TestUndo` (whatever kind is on top is popped, with a
   stale top refused); `rest/router` `TestRetiredUndoEndpointsAreGone` (the
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
   `TestRecord_HPDeltaOnAnyKind` (a heal capped and a damage
   floored on an entry that is not an `hp_change`), `domain/actionlog` `TestHPTotal` ("every
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
   (existing, updated: no void, the "before"), `TestInitiativeRestorer` (undo writes the replaced
   roll back).
6. **Redo stays possible.** A popped transaction stays on the log, voided: its entries, payloads
   and snapshots ("before" and "after") are kept unchanged, and a void is a correction row, never
   a delete or a rewrite. No undo path may drop what a later replay of the transaction needs.
   Redo itself (replaying the newest popped transaction) is a later ADR. (*Amended by
   [ADR 0019](0019-redo.md):* redo replays it; a popped entry can be redone and popped again, each
   time with a new correction row.)
   Tests: `adapters/actionlogdal` `TestPop_KeepsEntriesAndSnapshots` (a pop inserts one void
   correction and writes nothing else; the entry reads back with its payload and snapshots
   unchanged, voided).
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
   Tests: each state-changing service records its snapshot in the one transaction (the
   `Test*_Records*`/`Test*_Inserts*` tests of its package) and its restorer writes it back
   (`TestRestorers_*`), and `composition` `TestUndo_EveryStateKeyIsRestoredThroughItsServicesStorage`
   registers every restorer; `domain/service/xpsvc` `TestAdd_RecordsOneTransactionWithTheAwardsSnapshot`,
   `TestUpdate_AppliesOnlyTheSetFieldsInOneTransaction` (before and after),
   `TestDelete_RecordsTheRemovedAwardOrReportsNotFound`, `TestRestorer_WritesTheSnapshotBack`;
   `adapters/xpdal` `TestReinsert_KeepsIDAndDate`.

## Amendment 2026-10-19: the log's codec, re-encoding, state keys on the wire (decided, on implementation)

Decided in the architecture review grilled with the user (reports/architecture-review.html, items W1,
W4 and W7); not implemented. Each point is built by the item named and gets its tests then.

8. **The snapshot codec is the log's.** One snapshotter in `domain/actionlog` encodes a key's before
   and after, decodes them and implements `Restorer` (and `Replayer`) from a service's write and
   delete functions; a service declares what it snapshots and nothing else (W1). The meaning of a
   snapshot (step 2: the before written back on undo, the after on redo) is unchanged.
   _As built (CRI-59)_: `actionlog.Snapshotter[T]`, declared once per state-key family
   (`NewSnapshotter`, `MayBeAbsent` for things that can be absent, `WithReplay` for the initiative,
   whose "after" gets the replayed entry's id) and registered as `Restorer(write, remove)`. Every
   "before" and "after" is an envelope, `{"id":…,"state":…}` or `{"id":…,"absent":true}`: the id
   is the part of the key after `:` (left out for a key without one), the state the service's own
   encoding of `T`, and absent means the thing did not exist or is gone (restoring it calls the
   service's remove). Anything else is refused (`actionlog.ErrNotAnEnvelope`) and nothing is
   written. The ten families (buffs, saved buffs, round, initiative, spell slots, items, the
   weapons of before the equipment model, XP, realized level, spellbook, skill points, skill
   armour, AC override) all go through it; `restoreFunc` is gone.
   Tests: `domain/actionlog` `TestSnapshotter_EncodesBeforeAndAfterInTheEnvelope`,
   `TestSnapshotter_AbsentOnlyWhereTheFamilyAllowsIt`,
   `TestSnapshotter_RestoreWritesTheStateOrRemovesTheThing` (the absent / reinsert protocol, tested
   once here), `TestSnapshotter_TheEnvelopeNamesTheThing`, `TestSnapshotter_AFailingWriteFailsTheRestore`,
   `TestSnapshotter_ReplayStampsTheReplayedEntry`, `TestDecodeEnvelope_RefusesWhatIsNotAnEnvelope`;
   each service's restorer tests, on the envelope; `composition`
   `TestUndo_EveryStateKeyIsRestoredThroughItsServicesStorage` (unchanged cases and expectations,
   its stored states written as envelopes).
   _As built (CRI-63)_: each service declares its codecs in a `Module()` (part of `domain.Service`);
   the root keeps one list of its services, registers every module on the log at the end of `Build`
   and closes them in reverse. There is no package-level registry. A state-key family or a kind two
   modules claim refuses the start, naming both. Tests: `domain/service/actionlogsvc`
   `TestRegister_RefusesWhatTwoModulesClaim`, `TestRegister_RefusesACodecThatCannotRestore`,
   `TestRegister_RefusesAKeyThatIsNotAFamily`, `TestRegister_ARefusedModuleBindsNothing`;
   `composition` `TestBuild_TwoModulesClaimingOneFamilyRefuseTheStart`,
   `TestModules_EveryDeclaredFamilyHasAnUndoCase`, `TestBuild_EveryServiceIsOnTheModuleList`.
9. **Re-encoding is not editing.** A migration-time data step ([ADR 0025](0025-postgresql-storage.md)
   step 2.1.1) may rewrite stored snapshots into the codec's envelope (user, 2026-10-19: "Data step
   rewrites old snapshots"). It changes an encoding, never a before, an after, a delta, a payload or a
   correction: "entries are never edited" ([ADR 0007](0007-hp-ledger.md) step 1) stands for everything
   the player can see. No legacy decoder survives the step (W1).
   _As built (CRI-59)_: the envelopes live in two new columns, `action_snapshots.before_envelope`
   and `after_envelope` (migration 019); the data step `reencodeSnapshots` (`storage/postgres`),
   run right after migration 019 adds them, writes every stored snapshot into them and holds the
   only reader of the old encodings. It reads only the rows without their envelopes, so it can run
   again: the later migration that makes the envelopes required runs it just before. What a restorer used to settle when it read an old
   encoding is settled there, the same way: `null` was no initiative (absent); an item's or a
   weapon's `absent` / `removed` flags are the absence; an item count of before the equipment model
   takes the id the conversion renamed it to (`item_id_renames`, [ADR 0033](0033-equipment.md) step
   26.3, a table written once by that conversion); a name saved as a plain string before names were
   localized becomes its English body marked `namePartial` (restoring it still replaces that body
   only, [ADR 0023](0023-localized-text.md) step 11). A snapshot that predates powers or the
   equipment model keeps saying so (no `powers`, no `piece`), and its restore keeps reading the item
   as it is then ([ADR 0016](0016-inventory.md) Consequences, ADR 0033 step 26.2): that is the
   state's meaning, not its encoding. The old columns (`before_state`, `after_state`) keep the old
   encoding unchanged and are no longer read; a later migration drops them, with the step.
   **Rolling deploy** ([ADR 0024](0024-deployment.md) step 5.2): migration 019 is additive, so the
   machine still running the release before keeps reading the old columns and its undo of an entry
   recorded before the deploy restores it as before. A snapshot the new release records leaves the
   old columns at a JSON string every old decoder refuses, so the old release's undo of such an
   entry fails and changes nothing (never a misread state). The envelopes stay nullable in this
   release, so the old release keeps recording its actions during the minutes both run; its
   snapshots go to the old columns only (no dual write: that would bring the old encoders back).
   The new release reads such a row's envelopes as missing (a `null` before and after in
   `GET /api/log`) and refuses to undo or redo that entry (`snapshot_not_reencoded`, 409; nothing is
   written), until the data step encodes it. The scan of the spells game state references reads a
   spellbook one's spell from its key and skips a slot or buff list one meanwhile. The envelopes
   become required, and the old columns are dropped, by a later migration, in a later release
   (expand, then contract), with the data step run once more just before it.
   `GET /api/log` returns the envelopes as each snapshot's `before` / `after` (typed `unknown`, read
   by no web code).
   Tests: `storage/postgres` `TestReencodeSnapshots_EveryOldEncodingBecomesTheEnvelope` (one
   fixture per old encoding of the nine services and the AC override, an item name saved before
   localisation and an item renamed by the conversion among them; `snapshotfixtures`, whose
   envelopes composition's restorer test restores),
   `TestMigrate_SnapshotsAreReencodedRightAfterTheEnvelopesAreAdded`,
   `TestReencodeSnapshots_ARerunEncodesOnlyWhatIsMissing`, `TestReencodeSnapshots_RefusesWhatItCannotRead`,
   `TestStateSpellRefs_SnapshotsWithoutEnvelopesDoNotFailTheScan`;
   `adapters/actionlogdal` `TestInsertEntry_NewTransactionStartsUnderTheEntrysID`,
   `TestPop_KeepsEntriesAndSnapshots` (the adapter writes and reads the envelope columns) and
   `TestGetEntry_ASnapshotWithoutEnvelopesReadsAsNil`; `domain/service/actionlogsvc` `TestUndo`
   and `TestRedo_ReplaysTheNewestPoppedTransaction` ("a snapshot not re-encoded yet is refused").
   Rehearsal on copies of a real database (`just snapshot-rehearsal`,
   `scripts/snapshot-codec-rehearsal.sh`): the rolling deploy beside the release before, the
   follow-up's re-run of the step, a failing step rolling the run back, and every stack popped and
   replayed with the same state as the release before.
10. **The log says which entries are rolls.** `GET /api/log` marks each entry that is one of the roll
    kinds (`actionlog.RollKinds`); the web keeps no copy of the list (W4). Built: each entry carries
    `roll` (true for a roll kind, false otherwise; undo, redo and amend answers too). Tests:
    `rest/router` `TestLogList_MarksTheRollKinds`.
11. **A mutation answers with its state keys.** Every state-changing request, undo and redo included,
    answers with the state keys of the transaction it wrote; the web refetches by key family and
    nothing else. Moves outside the log ([ADR 0029](0029-users-login-and-access.md) amendment "creating,
    switching and retiring", step 4) keep an explicit refetch (W7).
    _As built (CRI-66)_: the keys travel in the `X-State-Keys` answer header (sorted, comma-separated,
    empty when nothing was logged; never on an error answer), so every body keeps its shape and a
    client of the release before reads every answer as before. They are the snapshots' keys plus two
    the log names itself, `log` (the journal changed) and `hp` (an entry moved current HP), which no
    module may claim. The log's transaction scope notes them and hands them to the request
    (`actionlog.Changes`) only once the outermost transaction commits. The web maps each family to
    the slices it reads again (`web/src/lib/stateKeys.ts`, exhaustive by type); a refused input (400,
    422) changed nothing and reads nothing; no header (the rolling deploy's fallback), an unknown
    family, a stale refusal (409: a stale undo top, a level-up no longer pending; 404: an entity
    removed elsewhere), a server error or a lost answer reads every slice again. The
    action module (`stores/table.ts`) owns the one error, notice and busy flag; the data stores
    expose their data and fetches only. Details: `docs/backend.md`, "State keys on write answers".
    Tests: `domain/service/actionlogsvc` `TestChanges_ARecordedTransactionNamesItsKeys`,
    `TestChanges_UndoAndRedoNameTheKeysOfTheirTransaction`, `TestChanges_AnAmendNamesTheLog`,
    `TestRegister_RefusesTheLogsOwnFamilies`; `rest/router` `TestStateKeys_TheAnswerNamesWhatTheWriteChanged`;
    `composition` `TestStateKeys_EachWriteNamesWhatItChanged`, `TestStateKeys_TheWebMapsEveryFamily`;
    web `stores/table.test.ts` (one case per family: exactly the mapped slices; the fallbacks; the
    refresh queue's ordering).

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
