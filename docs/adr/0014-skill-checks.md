# ADR 0014: Skill checks

**Status**: accepted, 2026-10-15; amended 2026-10-16 (DM adjustment, steps 5 to 8)

## Context

At the table the Bard checks his non-weapon proficiencies (playing an instrument, tumbling,
appraising…) and his bard skills (climbing walls, detecting noise, picking pockets, reading
languages). The player wants them rolled on the server like every other roll, shown in the result
card and the journal, and undoable. Triage (2026-10-15) took the list from the campaign sheet,
checked against PHB Table 37 (ability and modifier of each proficiency); where the sheet was
unclear the book's value was taken. The bard skill percentages are the sheet's, every adjustment
included. The PHB fails a proficiency check on a natural 20; triage kept that rule for proficiency
checks only (attacks and saves keep [ADR 0002](0002-attack-resolution.md) step 3 and
[ADR 0006](0006-saving-throws.md) step 3: no special naturals).

At the table the DM also rules that a check is harder or easier ("this tumbling has an additional
-2, so now it's DEX -2"). Triage (2026-10-16) added that adjustment with the logic of the save
adjustment ([ADR 0006](0006-saving-throws.md) steps 5 to 7: latest roll only, optional reason,
amend correction, undoable, verdict as rolled kept), with one difference: the roll is never
modified, **the target is moved**. The player may set it before the roll or on the result card
after it.

## Decision

Tests live in `api/internal/domain/service/skillsvc/skillsvc_test.go` unless named otherwise.

1. **Proficiency check.** One d20 through the dice port. The target is the sheet's score of the
   proficiency's ability plus its modifier, computed on the server; the check succeeds when the
   roll is at or under the target.
   Test: `TestSkillCheck_Step1_ProficiencyD20AtOrUnderTarget` (at the target, one over, a
   positive and a negative modifier; exactly one d20, no d100).
2. **A natural 20 always fails** a proficiency check, whatever the target.
   Test: `TestSkillCheck_Step2_NaturalTwentyAlwaysFails` (target 22: 20 fails, 19 succeeds).
3. **Bard skill check.** One d100 through the dice port; the check succeeds when the roll is at or
   under the skill's percentage. No natural is special.
   Test: `TestSkillCheck_Step3_BardSkillD100AtOrUnderPercent` (at the percentage, one over, 20,
   100; exactly one d100, no d20).
4. **Logged and poppable.** Each check is one `skill_check` entry recorded in the check's
   transaction, with no snapshots (it changes no state) and no summary. Its payload is
   `{id, kind, die, roll, target, success}`, plus `ability`, `abilityScore` and `modifier` for a
   proficiency; the UI words it. Undo pops it like any top entry
   ([ADR 0010](0010-action-stack-and-undo.md)); popping restores nothing. A skill's last check
   (`GET /api/skills`) is its newest non-voided entry.
   Tests: `TestSkillCheck_Step4_OneLoggedEntryPerCheck`, `TestList_TargetsAndLastCheckPerSkill`;
   `router` `TestSkills_RollWireShape`.
5. **Adjustment before the roll.** `POST /api/roll/skill/{id}` takes an optional
   `{adjustment, reason}` (no body: none). The adjustment moves the target, **+ easier, - harder**
   (the sign of every adjustment in the app): a proficiency is judged against ability score +
   modifier + adjustment, a bard skill against percentage + adjustment, in percentage points. Still
   exactly one die; a natural 20 still fails a proficiency check (step 2), whatever the adjusted
   target. Plain arithmetic, no clamp: an adjusted target at or under 0 always fails, one of 100 or
   more always succeeds on a d100. The payload gains `adjustment`, `adjustmentReason` (when set),
   `adjustedTarget` and `rolledSuccess` (the verdict as rolled, with this adjustment); `target`
   stays the unadjusted target, `success` is the verdict in effect. An entry logged before
   adjustments reads as adjustment 0. The adjustment applies to that one check only.
   Tests: `TestSkillCheck_Step5_PreRollAdjustmentMovesTheTarget` (harder, easier, at the adjusted
   target, natural 20 above 20, bard skill, at or under 0, over 100; one die),
   `TestList_TargetsAndLastCheckPerSkill` (entries without adjustment); `router`
   `TestSkills_RollWireShape` (with and without a body).
6. **Adjustment after the roll, on the latest check.** `POST /api/roll/skill/{entryId}/adjust`
   `{adjustment, reason?}` sets the adjustment of the check: it replaces the one in effect (the
   pre-roll one included), 0 removes it and drops the reason. The verdict is recomputed from the
   stored roll and the new adjusted target: **the roll never changes**, `rolledSuccess` is kept.
   Only the latest roll can be adjusted (`actionlog.RollKinds`, voided entries included, as in
   [ADR 0006](0006-saving-throws.md) step 6), and it must be a skill check: another kind is
   refused (400), a voided check or one that is not the latest roll (409). `GET /api/skills` shows
   a skill's last check with the adjustment (and its reason), adjusted target and verdict in effect.
   Tests: `TestSkillCheck_Step6_PostRollAdjustmentRecomputesTheVerdict` (fail to success, success
   to fail, a new adjustment replaces the previous one, 0 removes the pre-roll adjustment and
   drops the reason, natural 20, an entry logged before adjustments),
   `TestSkillCheck_Step6_LatestCheckOnly`; `actionlogsvc` `TestAmendLatest` ("not a skill check",
   "skill check, a later save exists", "latest skill check amended"); `router` `TestSkills_AdjustWireShape`,
   `TestSpecialSentinels` ("skill adjust not latest", "skill adjust voided", "skill adjust not a
   skill check").
7. **Audited and undoable.** Each adjustment after the roll is an `amend` correction on the entry
   carrying the recomputed payload (with `previousSuccess` and `previousAdjustment`); the entry is
   never edited. It is its own step on the action stack ([ADR 0010](0010-action-stack-and-undo.md)
   step 6, field `skillAdjustment`, from and to the adjustment): undo pops the newest adjustment
   and the check reads the one before it, down to the check as rolled.
   Tests: `TestSkillCheck_Step7_AdjustmentIsAnAmendCorrection`; `actionlogsvc`
   `TestEntry_PoppedStepAmendIsOutOfEffect` (a popped step leaves the previous adjustment in effect).
8. **Ranges.** A proficiency's adjustment is -20 to +20, a bard skill's -100 to +100 (percentage
   points); the reason is optional, trimmed, at most 80 characters. Anything else is refused (400)
   before a die is rolled or the log is amended.
   Tests: `TestSkillCheck_Step8_AdjustmentRanges` (each bound and one beyond, per kind, before and
   after the roll; reason of 80 and 81); `router` `TestSpecialSentinels` ("skill adjust out of
   range").

## Consequences

- The catalog is data: `seeds/skills.json` is reconciled at every start with the
  `proficiencies` (id, ability key, modifier) and `bard_skills` (id, percentage) tables by id: a
  new id is added, an existing one takes the seed's values and order, a row the seed no longer
  lists is kept. Ids are stable keys the UI names in en and fr. A new proficiency or a raised
  percentage is a seed change, never code. Editing them in the app is not covered.
- The ability score is the sheet's. Spells that raise an ability (Strength) do not change a
  proficiency's target.
- A skill check is one of the roll kinds (`actionlog.RollKinds`): it replaces the result card,
  so a check rolled after a save or an attack ends the correction of that roll
  ([ADR 0006](0006-saving-throws.md) step 6). Its own adjustment moves the target, never the
  roll (steps 5 to 8); a check rolled after it ends that adjustment.
- The verdict a check shows is always the arithmetic of its stored roll and adjusted target; there
  is no manual "mark as success". A DM ruling is an adjustment with a reason.
- The last check of a skill is looked up among the newest 500 checks; an older one is not shown.
- Not covered: proficiency slots and learning, armour and race adjustments to bard skills (the
  sheet's values are final), an adjustment kept from one check to the next ("sticky").
