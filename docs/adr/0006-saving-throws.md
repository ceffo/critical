# ADR 0006: Saving throws

**Status**: accepted, 2026-10-15 (critical-qki.37; collects the rules shipped in critical-qki.9 and
critical-qki.14, until then in `docs/backend.md`); amended 2026-10-16 (the adjustment moves the
target: steps 2, 4 and 5, step 8 added); step 6 amended by 0028 (ability checks are a roll kind). Step 1 amended by [ADR 0030](0030-classes-and-races.md) steps 6 and 22.1 (targets derived, racial bonus), implemented 2026-10-18. Step 9 (Wisdom's magical defence adjustment on saves against magic that attacks the mind) added
2026-10-21, decided in triage (user), as built 2026-10-21 (the column is catalogue format 10, [ADR 0021](0021-spell-catalogue.md) step 17).

_UI word: the French UI says jet de sauvegarde ([ADR 0036](0036-vocabulary.md) step 3)._

## Context

A saving throw is a d20 against a target from the character sheet, and the DM often rules it easier
or harder in a way the app cannot know in advance ("+4 against fire, Gaia protects you"). The player learns it after
the roll, so the result card must be correctable without rolling again, and the correction must
stay auditable. The Bard's sheet holds one target per category; the buffs may add to the roll.
Triage (2026-10-16) ruled that the DM's adjustment is not a bonus on the roll: **it moves the
target**, like the skill check adjustment ([ADR 0014](0014-skill-checks.md) step 5). The sign stays
the app's (+ easier, - harder), so a -2 raises the target by 2; the verdict of a given set of
numbers is unchanged.

## Decision

Tests live in `api/internal/domain/service/combatsvc/` unless named otherwise.

1. **The roll.** One d20, plus the save bonus of each active buff (a buff's `save_bonus`, which
   may be negative), against the target of the category on the character sheet: Paralyze/Poison,
   Rod/Staff/Wand, Petrify/Polymorph, Breath Weapon, Spells. Each buff is its own modifier, named
   after the buff. Any other category is refused (400) before anything is rolled.
   Tests: `TestResolveSavingThrow` (one case per category), `TestResolveSavingThrow_OneModifierPerBuff`,
   `TestResolveSavingThrow_Errors` ("unknown category").
2. **Verdict.** The roll total is `d20 + buff bonuses`; the adjusted target is `target -
   adjustment` (step 5). The save passes when the roll total is at or above the adjusted target:
   equal passes. The margin is the roll total minus the adjusted target (negative on a failure).
   Without an adjustment the adjusted target is the target.
   Tests: `TestResolveSavingThrow` ("para/poison pass" at exactly the target, "para/poison fail" one
   below, "buff bonus turns a fail into a pass", "negative buff bonus"), `TestAdjustSave_Recompute`,
   `TestAdjustSave_Step5_MovesTheTarget` ("harder, exactly at the adjusted target", "easier, one
   below the adjusted target").
3. **Natural 1 and 20.** Plain arithmetic, as for attacks ([ADR 0002](0002-attack-resolution.md)
   step 3): no automatic pass, no automatic failure.
   Tests: `TestResolveSavingThrow_NaturalOneAndTwentyArePlain`, `TestAdjustSave_Recompute`
   ("natural 1 is plain arithmetic").
4. **Logged, changes no state.** Every save is one `save` entry in the action log with no
   snapshots; its payload holds the category, the d20, the buff modifiers, the target (the
   sheet's, never adjusted), the adjustment and its reason, `rollTotal` (d20 + buffs, never the
   adjustment), `adjustedTarget`, the verdict, the margin and `rolledPassed` (the verdict as
   rolled, kept through any adjustment). `total` stays in the payload as `rollTotal + adjustment`
   (fields are added, never renamed); the UI reads `rollTotal` and `adjustedTarget`.
   Tests: `TestResolveSavingThrow_LogPayload`, `TestResolveSavingThrow` (asserts the kind and no snapshots on every case).
5. **Adjustment.** After the roll the player may set a signed adjustment from -20 to +20 with an
   optional reason (at most 80 characters, trimmed). The adjustment **moves the target, + easier,
   - harder**: the adjusted target is `target - adjustment` (a -2 raises a target of 14 to 16).
   It replaces any earlier adjustment; 0 removes it and drops the reason. The adjusted target,
   verdict and margin are recomputed from the stored d20, buff modifiers and target: **the d20,
   the buff modifiers and the roll total never change**, and the original verdict stays in
   `rolledPassed`. For the same numbers the verdict and margin are the ones of the adjustment as
   a roll bonus (`roll total >= target - adjustment`), natural 1 and 20 still plain arithmetic.
   The save result (`POST /api/roll/save` and `/adjust`) carries `rollTotal` and
   `adjustedTarget`; the adjustment is not one of its modifiers. An out-of-range value or a too
   long reason is refused (400).
   Tests: `TestAdjustSave_Step5_MovesTheTarget` (harder, easier, at and one below the adjusted
   target, buff and adjustment, natural 1, natural 20; the payload keeps the d20, the target and
   the adjustment's sign), `TestAdjustSave_Recompute` ("fail to pass", "pass to fail, buff kept",
   "back to 0 restores the rolled verdict, reason dropped", "a new adjustment replaces the
   previous one"), `TestAdjustSave_Rejections` ("above +20", "below -20", "reason too long");
   `router` `TestAdjustSave_WireShape`.
6. **Latest roll only.** Only the newest entry among the roll kinds (`actionlog.RollKinds`:
   attack, save, cast, item use, skill check ([ADR 0014](0014-skill-checks.md)), spell exposure
   ([ADR 0008](0008-spell-slots-and-long-rest.md) step 16), ability check
   ([ADR 0028](0028-ability-checks.md) step 5): the ones that
   replace the result card) can be adjusted, and that
   entry must be a save. Voided entries count when looking for the newest roll. Another kind is
   refused (400), a voided save or one that is not the latest roll is refused (409).
   Tests: `TestAmendLatest` (actionlogsvc: "not a save", "voided", "a later save exists", "a later
   attack exists", "a later cast exists", "a later item use exists", "a later skill check exists", "save, a later ability check exists"), `TestAdjustSave_Rejections`
   ("not the latest", "voided", "not a save"), router `TestSpecialSentinels` ("adjust voided", "adjust not latest",
   "adjust not a save").
7. **Audited.** The adjustment is an `amend` correction on the entry carrying the new
   payload (with `previousPassed` and `previousAdjustment`); the entry is never edited, the log
   reads the latest payload in effect (`effectivePayload`). Each adjustment is its own undoable
   step ([ADR 0010](0010-action-stack-and-undo.md) step 6).
   Tests: `TestAdjustSave_Recompute` (one amend per adjustment, `rolledPassed` kept),
   `TestAmendLatest` ("latest roll amended, older attack ignored", "amended again"),
   `TestEntryEffectivePayload`.
8. **Entries logged before the target moved.** A save (or its adjustment amend) logged before
   step 5 moved the target has no `rollTotal` and no `adjustedTarget`. The log is not migrated:
   the API (`GET /api/log`, and the entries undo, redo and amend return) serves both, computed on
   read in Go from the stored d20, modifiers, target and adjustment (`combatsvc.SavePayloadView`,
   registered per kind in the composition root and applied by `actionlogsvc` to every entry it
   returns, so no reader can skip it). Every stored field, the verdict and the margin included,
   is served as stored.
   Tests: `TestSavePayloadView_Step8_OldEntryServedInTheNewShape`; `router`
   `TestLog_ASaveIsServedWithRollTotalAndAdjustedTarget` and
   `TestLog_ASaveIsServedWithItsFieldsByEveryEndpoint` (the wire, on a mocked log service); `actionlogsvc`
   `TestListReads_ServeEntriesThroughThePayloadViews`; `combatsvc`
   `TestAdjustSave_Step8_SaveLoggedBeforeTheTargetMovedStoresBothFieldsInTheAmend`.
9. **Wisdom's magical defence adjustment** _(decided 2026-10-21, user triage; as built
   2026-10-21)_. PHB chapter 1, "Wisdom", the Magical Defense Adjustment paragraph and the
   column of the same name in Table 5 (p. 17): the adjustment applies to saving throws against
   magical spells that attack the mind (the paragraph's examples: beguiling, charm, fear,
   hypnosis, illusions, possession, suggestion), automatically.
   1. **The value.** Read from Table 5 for the sheet's Wisdom (the score the other Wisdom rules
      read, [ADR 0030](0030-classes-and-races.md) steps 13 and 14), from -6 at Wisdom 1 to +4 at 18
      and up. It is rules data, not a constant in the code: the catalogue's `wisdom` rule table
      gains the column (ADR 0030 step 23; the artifact change follows [ADR
      0021](0021-spell-catalogue.md)'s format rules), with the fixture's rows.
   2. **Every character.** It is an ability rule, not a class feature: a character of any class
      and race with that Wisdom gets it.
   3. **Which saves: a flag on the save request.** `POST /api/roll/save` takes an optional
      `mindAffecting` boolean (default false), set by the player when the magic attacks the mind
      (the DM's call, the PHB's list as the guide). It is not read from spell effect data: the
      saves rolled here are the character's own against magic aimed at it, which has no entry in
      the app (`seeds/spell_effects.json` describes the character's own spells); and no code
      names a spell (hard rule: no per-spell branch). The flag is valid with every category (a
      fear wand is a Rod/Staff/Wand save); it changes nothing else in the roll.
   4. **A named modifier on the roll.** With the flag, a non-zero adjustment is one modifier of
      kind `wisdom` whose detail is the Wisdom score and whose value is the signed adjustment,
      added to the roll like the race's bonus and the buffs' (step 1, ADR 0030 step 22.1); a
      negative value is a penalty. It never moves the target: only the DM's adjustment does
      (step 5). Without the flag, or with an adjustment of 0, there is no `wisdom` modifier. The
      breakdown and the result card show it as one more modifier line, worded by the web (no
      presentation text from the backend).
   5. **Logged with the roll.** The `save` payload gains `mindAffecting` (a field added, step 4;
      an entry without it reads as false). The `wisdom` modifier is stored among the modifiers,
      so step 5's recompute keeps it and step 8's view needs nothing new. The flag is fixed at
      the roll: a save rolled without it is corrected by the DM's adjustment (step 5) or undone
      and rolled again.
   6. **As built.** The value is derived for every character as `charactersvc.Derived.MagicDefence`
      (`ruletablesvc.Tables.MagicDefenceFor`, the catalogue's `magic_defence` column, format 10); the
      request flag is `combatsvc.SaveOptions.MindAffecting`; the modifier's kind is `wisdom`, its detail the
      Wisdom score. The web's "Against the mind" checkbox (fr "Contre l'esprit") sits on the saving throws'
      heading line and clears after each roll; the card words the modifier "Wisdom 17 (magical defence)" /
      "Sagesse 17 (défense contre la magie)".
   Tests: `combatsvc`
   `TestResolveSavingThrow_Step9_WisdomMagicDefence` (flagged Wisdom 17 gives +3 and Wisdom 3 gives
   -3 as a `wisdom` modifier, verdict at and one below the target; Wisdom 12 flagged and Wisdom
   17 unflagged give none; a flagged Rod/Staff/Wand save; a fighter and a mage get it like a
   cleric), `TestAdjustSave_Step9_TheWisdomModifierIsKept`; the rules adapter's
   `TestWisdomFor_Step9_MagicDefence` (the column read for Wisdom 1, 3, 7, 8, 14, 15, 17, 18 and 25);
   `charactersvc` `TestDerive_Step9_MagicDefenceForEveryClass` (step 9.2, on the fixture's tables); `router`
   `TestRollSave_Step9_MindAffectingFlag` (the wire: the flag in, the modifier and `mindAffecting` out).

10. **Wisdom's spell immunity** _(decided 2026-10-22, user triage; as built 2026-10-22)_. PHB chapter 1, "Wisdom", the Spell
   Immunity paragraph and the column of the same name in Table 5 (p. 17): a character with Wisdom 19 or more is immune to the
   spells listed on Table 5's rows, and the immunities are cumulative (a score keeps every lower row's).
   1. **The spells.** Read from Table 5 for the sheet's Wisdom (the score step 9.1 reads), as catalogue spell ids: the catalogue's
      `wisdom` rule table carries each row's own spells (`spell_immunities`, [ADR 0021](0021-spell-catalogue.md) step 19), and the
      reader adds up every row from the lowest to the score; none below 19. Rules data, never a list of spell ids in Go. Every
      character gets it, of any class and race (as step 9.2). Two Table 5 names are no catalogue spell and are left out: Rod of
      rulership (a magic item) and Cause fear (the catalogue has it only as the reverse of Remove fear; mapping it to `remove_fear`
      would make the character immune to a helpful spell, supervisor decision 2026-10-22).
   2. **A save names its spell.** `POST /api/roll/save` takes an optional `spellId` (a catalogue id). Against a spell the character
      is immune to, no d20 is rolled and no modifier read: the save passes, and the breakdown carries `immunity: {source: "wisdom",
      score}` and `spellId`; it is logged as a `save` entry like any save (step 4). Nothing to adjust: an adjustment of that entry is
      refused (409 `save_immune`). A spell the character is not immune to, or no spell named, rolls as before (steps 1 to 9; the
      `spellId` is logged with the roll). The spell is the player's choice; whether it is mind-affecting stays step 9.3's flag.
      The immune save's numbers: `adjustedTarget` is the `target` (nothing adjusts it); `d20Roll`, `rollTotal`, `total` and `margin`
      are 0 (there is no roll). The save does not check `spellId` against the catalogue: the picker sends catalogue ids, and an
      unknown id is logged as sent and matches no immunity (it rolls).
   3. **A spell cast on the character.** A spell received (cast on the character by someone else, `POST /api/spells/receive`) that the
      character is immune to applies nothing and rolls nothing (no buff, no damage, no duration roll): one cast entry with `textOnly: "immune"`
      and the same `immunity`, no snapshot.
   4. **As built.** `ruletablesvc.Tables.SpellImmunitiesFor`, derived for every character as `charactersvc.Derived.SpellImmunities`;
      `combatsvc.ImmunityTo` (the one check, read by saves and by `castsvc` Receive), `combatsvc.Immunity`,
      `combatsvc.SaveOptions.SpellID`, `combatsvc.ErrSaveImmune`. The web's "Spell" picker (fr "Sort") shows under the saves' label
      line while "Against the mind" is ticked (every Table 5 spell attacks the mind), a tag once set, cleared with the box after each
      roll; the card says "Immune · Passed" and "Wisdom 19: immune to Command, nothing to roll" (fr "Immunité", "Sagesse 19 :
      immunité contre Command, aucun jet"), the journal "Save vs Spells against Command: immune (Wisdom 19), PASSED", the last-roll
      column a check with no die; a spell received words "Your Wisdom (19) makes you immune to this spell: nothing applied."
   Tests: the rules adapter's `TestSpellImmunitiesFor_Step10_Cumulative` (none at 1, 12 and 18; the four at 19, not hold person;
   hold person, both spells, at 20; 26 at 25; 0 and 26 refused); `charactersvc` `TestDerive_Step10_SpellImmunitiesForEveryClass`;
   `combatsvc` `TestResolveSavingThrow_Step10_WisdomSpellImmunity` (WIS 18 rolls against charm person; WIS 19 immune to charm
   person, no die; WIS 19 rolls against hold person; WIS 20 immune to hold person), `TestAdjustSave_Step10_AnImmuneSaveIsRefused`;
   `castsvc` `TestReceive_Step10_ImmuneSpellAppliesNothing`; `router` `TestRollSave_Step10_SpellIDAndImmunity` (the wire:
   `spellId` in, `immunity` out, `save_immune`).

## Consequences

- The verdict a save shows is always the arithmetic of its stored numbers; there is no manual
  "mark as passed". A DM ruling that changes the outcome is an adjustment with a reason.
- The UI shows the adjustment on the target side ("d20 + buffs vs adjusted target", the sheet's
  target and the signed adjustment beside it) and never recomputes the verdict.
- A save changes no state, so popping it from the log ([ADR 0007](0007-hp-ledger.md) step 9)
  restores nothing; it only removes the line and frees the adjustment of the roll before it.
- The save bonus of a buff is a modifier of the roll, not of the target: a buff added after the
  roll does not change a logged save. The DM's adjustment is the opposite: it moves the target,
  never the roll.
- Not covered: the target values themselves (they come from the character sheet seed), saves by
  creatures other than the Bard, and the effect of a failed save (the DM's call). Table 5's spell
  immunity is step 10; immunities from other sources (race, items) are not covered.
