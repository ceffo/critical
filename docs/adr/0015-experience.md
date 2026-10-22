# ADR 0015: Experience

**Status**: accepted, 2026-10-15 (critical-kiv.13). Step 4 amended by
[ADR 0018](0018-transactions-and-undo.md) step 7, 2026-10-15. Steps 2 and 3 amended by [ADR 0030](0030-classes-and-races.md) (bonus by class, level from XP), on its implementation; step 3 implemented 2026-10-18 (`L` is the level ADR 0030 step 3 derives from the total, so it never exceeds 20 and reaching a threshold raises it; `T` is the character's class table, e.g. Table 20 for a mage), step 2 not yet (still the bard's rule). Steps 1 to 3 amended 2026-10-21 by [ADR 0042](0042-multi-class-and-dual-class.md) steps 5 to 7 (multi-class and dual-class), on implementation.

## Context

The player records the experience points the DM awards after each fight or session and wants the
total, how far the Bard is from the next level and a ledger he can correct. The PHB gives a bard
whose prime requisites (Dexterity and Charisma) are both 16 or more a 10% bonus on the experience
he earns, and Table 25 (Thief/Bard column) gives the XP that reaches each level. Triage
(2026-10-15), confirmed by the user: levelling up is out of scope ("levelling up is out of scope
for now"), so the level stays the sheet's; the bonus eligibility is decided on the server and is
the default of each entry's checkbox; the ledger sits outside the game action stack, with its own
edit and delete; the sheet's starting XP is not turned into sample entries.

## Decision

Tests live in `api/internal/domain/service/xpsvc/` unless named otherwise.

1. **Total = the sheet's starting XP plus every entry.** The starting XP is `characters.xp_total`
   (the campaign sheet's figure) and stays there: the ledger adds to it, so no opening entry can be
   deleted by mistake. An entry has a base (1 to 10,000,000 XP) and a bonus flag; with the flag it
   adds `base + bonus` where `bonus` is 10% of the base **rounded to the nearest point, halves up**
   (`(base × 10 + 50) / 100` in integers: 15 gives 2, 14 gives 1), without it the base. Rounding is
   per entry, never on the sum. The server returns each entry's `bonusAmount` and `total`, the
   ledger's `total`, and previews an entry before it is added (`GET /api/xp/preview`).
   Tests: `TestEntryAmount_TenPercentBonusRoundedHalfUp`, `TestLedgerTotal_OpeningPlusEveryEntry`,
   `TestPreview_IsStep1AndValidatesTheBase`, `TestGet_TotalsEntriesOverTheSheetAndReportsEligibility`.
   _(Amended 2026-10-21 by [ADR 0042](0042-multi-class-and-dual-class.md) step 5, on implementation: the ledger stays one list per character; each award's amount goes in equal shares to a multi-class character's active class entries (even one at its level limit), wholly to a dual-class's active entry; a class entry's XP is its starting XP plus its shares, derived on read)_
2. **Bonus eligibility from Dexterity and Charisma both at least 16.** Read from the sheet's
   scores (not the effective ones under buffs) and returned as `bonusEligible`; the client uses it
   only as the default of the add form's +10% checkbox. The flag stored on each entry is what
   counts: the server accepts it either way, so a DM ruling can be recorded.
   Tests: `TestBonusEligible_Step6_EveryPrimeRequisiteAt16` (the bard rows; [ADR 0040](0040-classes-are-data.md) step 6), `TestGet_NotEligibleBelow16`,
   `TestGet_TotalsEntriesOverTheSheetAndReportsEligibility`.
   _(Amended 2026-10-21 by [ADR 0040](0040-classes-are-data.md) step 6, on implementation: eligible when every prime
   requisite of the character's class is 16 or more, read from the class definition; Dexterity and
   Charisma are the bard's, not every class's.)_
   _(Amended 2026-10-21 by [ADR 0042](0042-multi-class-and-dual-class.md) step 6, on implementation: with several class entries, eligible when every prime requisite of every active entry's class is 16 or more; a frozen entry's class does not count)_
3. **Progress to the next level from PHB Table 25.** The thresholds are rules data in the
   catalogue artifact's `rule_tables` ([ADR 0021](0021-spell-catalogue.md) step 9;
   `bard_experience`, levels 1 to 20, validated at startup: from 0, strictly increasing). With
   `L` the sheet's level and `T(n)` the XP that reaches level `n`:
   `currentFloor = T(L)`, `nextThreshold = T(L + 1)`, `toNext = max(0, T(L + 1) - total)`,
   `progress = clamp((total - T(L)) / (T(L + 1) - T(L)), 0, 1)` and `progressPercent` the same in
   whole percent rounded down. At level 20, or above it (a level override), there is no next level: `nextLevel` and
   `nextThreshold` are null, `toNext` 0 and the progress full; above 20 the floor stays level 20's
   and every listed level is `past`. Reaching the threshold changes
   nothing but `toNext` (0) and the progress (full): no level-up. `levels` lists 8 consecutive
   levels from `L - 2`, kept inside 1 to 20, each with its threshold and a status (`past`,
   `current`, `next`, `ahead`).
   Tests: `TestComputeProgress_FromTheClassTable`, `TestBardXPThreshold` (ruletablesadpt, against the
   PHB page), `TestParse_Step9_3_RejectsBrokenTables` (ruletablesadpt).
   _(Amended 2026-10-21 by [ADR 0042](0042-multi-class-and-dual-class.md) step 7, on implementation: the progress is per active class entry, on the entry's XP and its class's table; a frozen entry shows none)_
4. **The ledger is on the action stack.** Adding, editing (base, bonus, reason, session) and
   deleting an entry each write the `xp_entries` table and record one action-log entry in the same
   transaction ([ADR 0018](0018-transactions-and-undo.md) steps 1 and 7): `xp_add`, `xp_edit`,
   `xp_remove`, each with the entry's `xp:<id>` snapshot. An edit keeps the entry before and after
   (snapshot and payload); a delete keeps the removed entry as the "before". The journal's Undo
   pops it like any other entry: the ledger, its total and the progress derived from it come back,
   a deleted entry under its own id and date. An edit that changes nothing writes nothing. An entry
   is dated by the server (its local day) when it is added. Every write answers with the whole
   ledger. The payload is the entry as stored with its `bonusAmount` and `total` (and `before` for
   an edit); the web words it.
   Tests: `TestAdd_RecordsOneTransactionWithTheAwardsSnapshot`,
   `TestUpdate_AppliesOnlyTheSetFieldsInOneTransaction`, `TestUpdate_NothingChangedRecordsNothing`,
   `TestDelete_RecordsTheRemovedAwardOrReportsNotFound`, `TestRestorer_WritesTheSnapshotBack`,
   `TestCRUD_ScopesToTheCharacterAndMapsEntries`, `TestReinsert_KeepsIDAndDate` (xpdal, on a mocked
   `Querier`; the newest-first order is the query's `ORDER BY`).

## Consequences

- Correcting the starting XP means editing the sheet (`seeds/character.json` and a dev reset), not
  the ledger.
- A deleted or edited entry stays in the journal (until undone), like every other gameplay change
  (step 4 as amended by ADR 0018; the ledger was first outside the stack).
- Migration 018 drops the never-written `xp_events` table from migration 002.
- Not covered: levelling up (HP, THAC0, saves, slots and the level itself), multi-class and dual-class
  XP _(now [ADR 0042](0042-multi-class-and-dual-class.md) steps 5 to 7)_, XP for other classes, negative awards (level drain is out of scope).
