# ADR 0015: Experience

**Status**: accepted, 2026-10-15 (critical-kiv.13)

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
   `TestPreview_IsStep1AndValidatesTheBase`, `TestXPLedger_AddEditDeleteOnTheRealStack`
   (composition).
2. **Bonus eligibility from Dexterity and Charisma both at least 16.** Read from the sheet's
   scores (not the effective ones under buffs) and returned as `bonusEligible`; the client uses it
   only as the default of the add form's +10% checkbox. The flag stored on each entry is what
   counts: the server accepts it either way, so a DM ruling can be recorded.
   Tests: `TestBonusEligible_DexAndChaBothAtLeast16`, `TestGet_NotEligibleBelow16`,
   `TestGet_TotalsEntriesOverTheSheetAndReportsEligibility`.
3. **Progress to the next level from PHB Table 25.** The thresholds are rules data in
   `seeds/rule_tables.json` (`bard_experience`, levels 1 to 20, validated at startup: from 0,
   strictly increasing). With `L` the sheet's level and `T(n)` the XP that reaches level `n`:
   `currentFloor = T(L)`, `nextThreshold = T(L + 1)`, `toNext = max(0, T(L + 1) - total)`,
   `progress = clamp((total - T(L)) / (T(L + 1) - T(L)), 0, 1)` and `progressPercent` the same in
   whole percent rounded down. At level 20, or above it (a level override), there is no next level: `nextLevel` and
   `nextThreshold` are null, `toNext` 0 and the progress full; above 20 the floor stays level 20's
   and every listed level is `past`. Reaching the threshold changes
   nothing but `toNext` (0) and the progress (full): no level-up. `levels` lists 8 consecutive
   levels from `L - 2`, kept inside 1 to 20, each with its threshold and a status (`past`,
   `current`, `next`, `ahead`).
   Tests: `TestComputeProgress_FromTable25`, `TestBardXPThreshold` (ruletablesadpt, against the
   PHB page), `TestParse_RejectsBrokenTables` (ruletablesadpt).
4. **The ledger is outside the action stack.** Adding, editing (base, bonus, reason, session) and
   deleting an entry write the `xp_entries` table directly and record no action-log entry, so
   undo (ADR 0010) never pops one. An entry is dated by the server (its local day) when it is
   added. Every write answers with the whole ledger.
   Tests: `TestXPLedger_AddEditDeleteOnTheRealStack` (composition),
   `TestUpdate_AppliesOnlyTheSetFieldsInOneTransaction`, `TestCRUD_RoundTripsNewestFirstPerCharacter`
   (xpdal).

## Consequences

- Correcting the starting XP means editing the sheet (`seeds/character.json` and a dev reset), not
  the ledger.
- A deleted or edited entry leaves no trace: the ledger is the player's notebook, not the table's
  history.
- Migration 018 drops the never-written `xp_events` table from migration 002.
- Not covered: levelling up (HP, THAC0, saves, slots and the level itself), multi-class and dual-class
  XP, XP for other classes, negative awards (level drain is out of scope).
