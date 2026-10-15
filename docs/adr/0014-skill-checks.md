# ADR 0014: Skill checks

**Status**: accepted, 2026-10-15

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
   `composition` `TestSkillCheck_UndoPopsTheCheck`; `router` `TestSkills_RollWireShape`.

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
  ([ADR 0006](0006-saving-throws.md) step 6). A check itself is never adjusted.
- The last check of a skill is looked up among the newest 500 checks; an older one is not shown.
- Not covered: proficiency slots and learning, armour and race adjustments to bard skills (the
  sheet's values are final), checks with a situational modifier from the DM.
