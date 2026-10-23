# ADR 0044: Psionic combat: defence modes, attack modes, tangents and contact

**Status**: accepted 2026-10-22 (user: "default": every default of Q1 to Q16 taken). Layer 2 of the psionicist, which the user's triage of 2026-10-21
put in a "later item in the same epic" ([ADR 0043](0043-psionics.md), Context). The readings where
the book is silent, unclear or leaves the choice to the DM (Q1 to Q16) are proposed defaults put to
the user ("Open questions"); each step cites the question it waits on. Not implemented. Replaces
ADR 0043's "Layer 2, later" steps 20 to 22 and amends ADR 0043 steps 6, 7, 10 and 11,
[ADR 0004](0004-buffs.md) step 17 and [ADR 0021](0021-spell-catalogue.md) (a format 18), each on
implementation.

## Context

Layer 1 (ADR 0043, built on main and in the use-power item) gives the psionicist a PSP ledger,
known powers, the power check (step 10), the initial cost (step 11) and maintained powers as buff
rows charged on Next Round and passing time (steps 13 to 16). A power whose initial cost is
`contact` is refused (`needs_contact`), and the ten attack and defence modes can be known and
used only as ordinary checks: nothing yet makes an attack meet a defence.

What the book adds (all under `cpsh/ch02-psionic-combat/` unless named otherwise; no book text is
copied here):

- **Contact opens a mind** (`06-telepathic-combat.md`): most telepathic powers need it first. A
  being with little or no psionic talent can be contacted with the contact power; a psionicist or
  psionic creature has a closed mind and can be contacted only through telepathic combat, unless it
  lets the contact in. The contact power itself (`cpsh/ch07-telepathy/02-telepathy-tower-of-iron-will-onward.md`,
  Contact) costs by the target's level or hit dice (`cpsh/tables/29-contact-initial-cost.md`: 1-5
  gives 3, 6-10 gives 5, 11-15 gives 13, 16-20 gives 18), is kept at 1 PSP a round per subject, is
  harder at a distance (`tables/30-contact-distance-modifiers.md`: 0 in sight down to -9 at 10,000
  miles) and with minds far from mammals (`tables/31-contact-life-order-modifiers.md`: -1 to -8),
  and a nonpsionicist who does nothing but resist gives -2 (same section, Resistance). A power whose
  initial cost reads `contact` needs contact made in an earlier round (same section; power
  descriptions, Initial Cost); while another telepathic power on the subject is kept up, the
  contact costs nothing of its own.
- **Attack modes** (`07-telepathic-attack-modes.md`): five powers (mind thrust 2, ego whip 4, id
  insinuation 5, psychic crush 7, psionic blast 10). Each use is two power checks against one
  target for one cost (the one-two punch). Against an open mind each has its own effect (its power
  description); against a closed one it only wears the resistance down.
- **Defence modes** (`08-telepathic-defense-modes.md`; ch. 1, `09-defense-modes.md`): five powers
  (mind blank 0, thought shield 1, mental barrier 3, intellect fortress 4, tower of iron will 6),
  in force from the start of the round, the one cost covering every attack of that round; they work
  against attack modes only. Mind blank is apart (its description in
  `cpsh/ch07-telepathy/04-telepathy-mental-barrier-onward.md`): it costs nothing to keep, PSPs are
  recovered while it is up, it holds even asleep, and it is down only when the player says so or
  another defence mode is used.
- **The psychic contest** (`03-psychic-contests.md`): when an attack meets a defence, each side
  makes its power check; the higher successful roll wins, and the defender wins when neither
  succeeds or the rolls are equal. A power kept up from an earlier round checks again for the
  contest with +1 to its score, and a failed roll counts as a success of 1. A defender may keep a
  roll he likes for the rest of the round's contests of that power.
- **Table 14** (`09-attack-modifiers.md`; `tables/19-attack-versus-defense-modes.md`): the attack
  mode against the defence mode gives a modifier to the attacker's power score (+5 to -5). It has
  no column for an undefended mind.
- **Tangents** (`10-tangents.md`): an attack that beats the defence (or succeeds against a mind
  with none) makes a tangent; three make full contact, after which the attacker makes no more
  attacks on that mind that round. Keeping one or two tangents costs 1 PSP a round, with one mind
  only; they break when the attacker lets go, does not pay, uses an attack mode on another target,
  or is incapacitated.
- **The round** (`05-the-combat-round.md`): defence modes are outside the initiative order; a power
  kept from an earlier round, and an earlier round's tangents, cannot be disrupted.
- **Psychic lock** (`04-psychic-lock.md`): a tie with no clear defender (two powers on one object)
  is a deadlock, and giving up costs 4d4 PSPs.

The app keeps one character per sheet; it has no record of an opponent. A melee attack already
takes the opponent's AC from the player (combatsvc). Psionic combat follows that model: the
opponent is what the DM tells the player.

## Decision

Book facts are cited by file and section of `critical-rulebooks`, under `cpsh/` as in ADR 0043.
**Test names** are written without their `Test` prefix while the test does not exist (the
`check-adr-tests` gate). Every warning and refusal is structured data (a code and its numbers),
worded by the web. No power id, mode id or class id in production code (the "no special case per
spell" rule, ADR 0040 step 10): a mode is known by its catalogue `kind`, a cell of Table 14 by the
two power ids in its row of data.

### The data

1. **Catalogue format 18: the combat tables** (amends [ADR 0021](0021-spell-catalogue.md), a new
   step 25, on implementation; the builder in `critical-rulebooks`, the reader in the API).
   1. `rule_tables.attack_versus_defence`: 25 rows `{attack, defence, modifier}`, `attack` an
      `attack_mode` power id, `defence` a `defence_mode` power id (Table 14,
      `tables/19-attack-versus-defense-modes.md`). The builder refuses a missing cell or an id of
      another kind.
   2. `rule_tables.contact_cost`: rows `{min_level, max_level, cost}` (Table 29). Levels over 20 are
      Q15.
   3. `rule_tables.contact_modifiers`: rows `{kind, code, modifier}`, `kind` `distance` (Table 30:
      `sight`, `mile_1`, `miles_10`, `miles_100`, `miles_1000`, `miles_10000`), `life_order` (Table
      31: `mammal`, `marsupial`, `bird`, `reptile_amphibian`, `fish`, `arachnid_insect`,
      `monster`, `plant`) or `resistance` (the nonpsionicist's -2, code `resisting`). The codes are
      worded by the web (en, fr), as Table 6's activities are; no per-language field
      ([ADR 0023](0023-localized-text.md)).
   4. **A power's cost may name its table**: `initial_cost` takes the form `{table: contact_cost}`
      (the contact power) where it read `variable`; a cost of `contact` may carry a `plus` amount
      (Switch Personality's "contact +30", which format 13 read as `contact`;
      `catalogue-inputs/POWER_STATS` gains the line).
   5. **Rollout, as ADR 0021 step 18.6**: the API reads format 18 only.
   Tests: `TestSchema_Step25_OnlyFormat18IsValid` (cataloguecheck), `TestParse_Step25_ReadsFormat18AndRefusesFormat17`
   (catalogueadpt), `TestRuleTables_Step1_AttackVersusDefence` (ruletablesadpt: the 25 cells, mind
   thrust against tower of iron will -5, a cell naming a devotion refused at start),
   `TestRuleTables_Step1_ContactCostAndModifiers`.

### The opponent and the combat state

2. **The opponent is the request's** (no opponent record). An attack or a contact names its
   `target`, a label the player gives (1 to 60 characters; user data like a character's name, not
   a text of the app, so ADR 0023 does not apply), and what the DM tells: `mind` `open` or `closed`
   (a psionicist or psionic creature is closed unless it lets the contact in; a nonpsionicist and a
   wild talent with no defence mode are open: `06-telepathic-combat.md`; the Contact power's
   Resistance), its `level` (levels or hit dice, for the contact cost), the `defence` it raised
   (a `defence_mode` power id, or none) and the DM's rolls for that defence (step 7). Nothing of the
   opponent is stored but the rows of step 3.
   The shape is `psionics.Opponent` (`domain/psionics`), checked against the catalogue: the
   label trimmed, `mind`, `level` 0 or more (0: not given), `defence` a `defence_mode`, and the
   DM's rolls given only with a defence, one per check of the attack (two).
   Tests: `TestOpponent_Step2_TheRequestsOpponentIsValidated`, `Attack_Step2_TheOpponentIsTheRequests`
   (psionicsvc: two attacks naming two targets read no opponent store).

3. **Tangents and contact are buff rows** (amends [ADR 0004](0004-buffs.md) step 17 as ADR 0043
   step 13 did; the buff list is where maintained psionics already live). A row gains `target`
   (the label) and `tangents` (1 or 2; absent on a full contact). Both kinds are maintained at
   1 PSP a round (`10-tangents.md`, Maintaining Tangents; Contact's maintenance), charged by ADR
   0043 steps 14 and 15 with their first charge on the Next Round after the round they are made
   (ADR 0043 Q10).
   1. **Tangents**: one row at most (one mind at a time). A won contest against a closed mind adds
      a tangent to the target's row (step 7); the third turns the row into a contact row.
   2. **Contact rows**: any number, one per target. A row of the same target is renewed, never
      doubled.
   3. **Breaking** (`10-tangents.md`): dismissing the row lets go (ADR 0043 step 16: free, logged,
      undoable); an unpaid charge ends it (ADR 0043 step 14's `maintenance_unpaid`); an attack mode
      used on another target ends the tangent row in the attack's transaction (contact rows stay);
      an incapacitated attacker is the player's dismiss. An opponent's ejection is the DM's word,
      the player's dismiss (Ejection stays a layer-1 power with its text).
   4. **The rows' shape**: a row with a `target` and no power id is a tangent row (`tangents` 1
      or 2) or a contact row (`tangents` absent); a row with both a power id and a `target` is
      step 10's. The row's name is the label; no buff of that name renews it. The fields live in
      the buff list's JSON like `defence` (no migration). buffsvc's `AddTangents` is the
      attack's one call (it breaks another mind's tangent row, then adds the won tangents) and
      gives the attack's entry one snapshot. Maintenance, unpaid and dismiss entries name the
      row's `target`; a long rest's entry lists its `targetsEnded`.
   Tests: `TestTangents_Step3_ARowPerTargetChargedOnePSP` (buffsvc), `TestTangents_Step3_AnotherTargetBreaksThem`
   (buffsvc, the rule the attack calls), `TestTangents_Step3_UnpaidBreaks` (buffsvc), `TestTangents_Step3_UndoRestoresTheRow` (composition).

### Defence

4. **Raising a defence mode** (amends ADR 0043 step 10, "every activation is checked", for the
   `defence_mode` kind; `08-telepathic-defense-modes.md`). Raising pays the mode's initial cost
   with **no roll**: the mode's power checks are the contests' rolls (step 8) _(Q1)_. It is a
   `defence_raised` entry with the PSP delta, refused `not_enough_psp` under the cost, refused
   `not_in_combat` with the round counter at 0 _(Q3)_. The mode is the character's **raised
   defence** for the round it is raised in: a row that ends at the next move of the round counter
   (Next Round, End combat, Set round, passing time). One a round: raising another replaces it and
   pays again _(Q2)_. A standing mode (step 5, cost 0) is exempt from `not_in_combat`: it is
   raised in or out of combat. **Stop** (ADR 0043's stop) dismisses any defence row, raised or
   standing, though a zero-cost row is not "maintained". It does not count toward ADR 0043 step 12
   (already built).
   Tests: `RaiseDefence_Step4_PaysTheCostNoRoll` (psionicsvc, dice mock called 0 times),
   `TestRaiseDefence_Step4_LastsTheRound` (buffsvc), `TestRaiseDefence_Step4_ASecondReplacesTheFirst`,
   `RaiseDefence_Step4_RefusedOutOfCombat`.

5. **A standing defence: a defence mode kept at no cost** (mind blank; amends ADR 0043 steps 6 and
   7). A `defence_mode` power whose catalogue `maintenance` costs 0 is raised as step 4 says
   (except that it needs no combat round, _Q3_) but its row is untimed (ADR 0043 step 13) and stays across rounds _(Q4)_. A maintained row whose
   cost is 0 neither refuses PSP recovery (ADR 0043 step 6's `psp_recovery_while_maintaining`)
   nor ends at the long rest (step 7): the book recovers PSPs under mind blank and keeps it asleep.
   The character's **current defence** is the mode raised this round, else the standing one, else
   none. A rule on the cost, never on the power's id. A standing row counts as **kept** (the +1 of
   step 8) when its `defence_raised` entry is not in this round: the app reads the character's
   last 20 log entries for it, and a raise beyond that lookback counts as kept too.
   Tests: `Recover_Step5_AZeroCostRowAllowsRecovery` (psionicsvc), `LongRest_Step5_KeepsAZeroCostRow`
   (restsvc), `CurrentDefence_Step5_ARaisedModeOverTheStandingOne` (psionicsvc).

### The contest

6. **The psychic contest is a pure rule** (`domain/psionics`, beside `Validate`;
   `03-psychic-contests.md`). Given two power checks (ADR 0043 step 10's outcome and roll for each)
   and which side defends: the higher **successful** roll wins; the defender wins when neither
   succeeds or both rolls are equal. A side whose power is kept from an earlier round has +1 on its
   score and a failed roll that counts as a success of 1 (Maintained Powers). ADR 0043 step 10's
   outcomes hold: a natural 20 fails, a natural 1 succeeds (as a roll of 1), a roll equal to the
   score succeeds (its text shown).
   Tests: `TestContest_Step6_HigherSuccessWins` (the book's example: an attack at 15 against a defence
   at 12; 14 against 9 wins, 9 against 11 loses, 8 against 8 the defender, both failing the
   defender), `TestContest_Step6_AKeptPowerPlusOneAndFailedIsOne`.

### Attacking

7. **An attack mode's use** (`07-telepathic-attack-modes.md`; `09-attack-modifiers.md`;
   `10-tangents.md`). The request: the mode (a known `attack_mode` power), the opponent (step 2),
   the labelled modifiers of ADR 0043 step 10 (the mode's range steps, read from its text by the
   player, _(Q13)_), and, when the opponent's defence is raised, the DM's roll for each of the two
   contests as `{roll, success}` _(Q6)_.
   1. **One cost, two checks**: one `psionic_attack` entry, one transaction, the PSP delta the
      mode's initial cost, refused `not_enough_psp` before any roll (ADR 0043 Q6). Two d20 through
      the dice port, each against the mode's score plus Table 14's cell for the opponent's defence
      (a labelled line `attack_versus_defence` with the two ids) plus the modifiers. Half the cost
      when both checks fail, the whole cost otherwise _(Q5)_. Each use pays again: a round's two
      attacks per cost _(Q10)_. The use counts toward ADR 0043 step 12.
   2. **Each check against the defence**: a contest (step 6) with the DM's roll when a defence is
      raised; with none, a successful check wins (Table 14 has no column for it: no modifier)
      _(Q14)_.
   3. **A closed mind**: each won contest adds a tangent to the target's row (step 3), after
      ending a tangent row of another target. The third tangent is full contact: the row becomes a
      contact row and the second check is **not rolled** (no more attacks that round; the dice mock
      counts one roll) _(Q11)_.
   4. **An open or contacted mind** (an open target, or one with a contact row): a won contest is
      the mode's effect, its power description shown, the DM's to apply; no tangent _(Q8, Q9)_.
   5. The entry carries both checks' breakdowns, each contest's sides and winner, the tangents
      before and after and the target. Undo gives back the PSPs and the rows (ADR 0018).
   Tests: `Attack_Step7_TwoChecksOneCost` (psionicsvc, dice called twice, 4 PSPs for ego whip),
   `Attack_Step7_TableFourteenModifiesTheScore` (mind thrust against tower of iron will: -5 on
   both targets), `Attack_Step7_HalfTheCostWhenBothFail`, `Attack_Step7_ATangentPerWonContest`,
   `Attack_Step7_TheThirdTangentIsContactNoMoreAttacks` (two tangents held, the first check wins:
   one roll, a contact row), `Attack_Step7_NoDefenceASuccessWins`, `Attack_Step7_AnOpenMindShowsTheEffect`,
   `Attack_Step7_UndoRestoresPSPAndTangents` (composition).

### Defending

8. **A contest against the character** (`08-telepathic-defense-modes.md`; `03-psychic-contests.md`,
   Multiple Conflicts). The request: the DM's attack mode (an `attack_mode` power id, any, not
   necessarily known) and its one or two rolls as `{roll, success}`. Against the current defence
   (step 5) the app rolls one check per incoming roll (the defence's score, +1 and a failure as 1
   when the defence is the standing row raised in an earlier round, step 6), decides each contest
   and logs a `psionic_defence` entry with no PSP delta (the cost was paid at the raise). The
   player may **keep** a roll: `keep` is the index of a contest of the request, and a second keep in
   the same round is refused (the field error "a check is already kept this round"). The next
   contests of that defence in the same round take it with no die, read from the round's earlier `psionic_defence` entries (as ADR 0043 step 12 reads the
   round's checks: no stored column). With no current defence nothing is rolled and the
   attacker's successes win. The answer names, per contest, the winner; a lost contest is a
   tangent the DM keeps: the character's sheet tracks no opponent's tangents _(Q7)_. Table 14's
   cell is shown beside the DM's roll (the DM applies it to his own score).
   Tests: `Defend_Step8_AContestPerIncomingRoll` (psionicsvc: two rolls, two dice),
   `Defend_Step8_AKeptRollForTheRound` (the third contest of the round rolls no die),
   `Defend_Step8_TheStandingDefencePlusOne`, `Defend_Step8_NoDefenceLoses`,
   `Defend_Step8_UndoRemovesTheEntry` (composition).

### Contact

9. **The contact power against an open mind** (Contact, `cpsh/ch07-telepathy/02-telepathy-tower-of-iron-will-onward.md`;
   Tables 29 to 31). A use of a power whose cost names `contact_cost` (step 1.4) takes the
   opponent (step 2): the cost is Table 29's row for its level, the score takes the distance row,
   the life-order row and, when the DM says so, `resisting` as labelled lines (from
   `contact_modifiers`), then ADR 0043 steps 10 and 11 as for any power (half the cost on a
   failure; a failure may be tried again next round). A success puts the target's contact row (step
   3). Refused `mind_closed` against a closed mind (the book: the power never works on one) and
   `level_out_of_table` for a level Table 29 has no row for, unless the request gives the DM's
   cost (at least 1, refused for a level the table has) _(Q15)_; that use still puts the contact
   row. The contact power goes through this route only: the use route refuses a power whose cost
   names a table (`use_contact_route`). The book's optional language rule is not taken _(Q16)_.
   Tests: `Contact_Step9_CostByTheTargetsLevel` (level 7: 8, or 4 on a failure),
   `Contact_Step9_DistanceLifeOrderAndResistance` (a bird at 10 miles resisting: -3, -3, -2),
   `Contact_Step9_AClosedMindRefused`, `Contact_Step9_ASuccessPutsTheContactRow`,
   `Contact_Q15_TheDMsCostPastTheTable` (level 21: the contact row, inflict pain on it the next
   round), `Use_Step9_TheContactPowerGoesThroughContact`.

10. **A power whose cost is contact** (amends ADR 0043 step 11, which refused it `needs_contact`;
    power descriptions, Initial Cost; Contact). Its use names a target holding a contact row made
    in an **earlier round** (refused `no_contact` otherwise, before the roll). Its initial cost is
    its `plus` amount, 0 when none (the contact paid for the rest) _(Q12)_; then ADR 0043 steps 10
    to 13. A maintained one is a row carrying the `target`. Each maintained use on a subject is its
    own row and pays its own maintenance (Contact, `cpsh/ch07-telepathy/02-telepathy-tower-of-iron-will-onward.md`):
    such a row is renewed only by the same power on the same target (amends ADR 0043 step 13), so
    inflict pain on two minds is two rows. In a round where such a row on the
    target pays its maintenance, the target's contact row charges nothing (the book: contact is
    free while another telepathic power uses it). When the contact row ends (dismissed or unpaid),
    the rows that use it end in the same transaction _(Q12)_.
    Tests: `ContactPower_Step10_NeedsContactFromAnEarlierRound` (psionicsvc: contact made this
    round refused, last round accepted), `ContactPower_Step10_ThePlusAmount` (switch personality: 30),
    `ContactPower_Step10_ContactFreeWhileAPowerUsesIt` (buffsvc: inflict pain at 2 on a contacted
    target charges 2 a round, not 3; at 2 PSPs the power pays first and the contact is free),
    `ContactPower_Step10_EndingContactEndsItsPowers`, `Add_Step10_APowerOnTwoMindsIsTwoRows`,
    `ContactPower_Step10_TwoMindsPayTheirOwnMaintenance` (2 + 2, both contacts free).

### The screen and the journal

11. **Wording and UI.** New entries `defence_raised`, `psionic_attack`, `psionic_defence`, each
    with a journal view (en, fr) and a fixture; new refusals `not_in_combat`, `mind_closed`,
    `no_contact`, `level_out_of_table`, `use_contact_route`, each a render row and an `apiError` key (en, fr); the
    contact modifier codes of step 1.3 worded (en, fr). The web adds psionic combat to the Powers
    panel: the current defence and a raise control for each known defence mode; an attack form
    (mode, target picked from the rows or typed, open or closed, the opponent's defence and the
    DM's two rolls); a defend form (the DM's mode and rolls, keep); the tangent and contact rows
    with their target. Game logic stays in Go: the forms send intent and render the answer.
    Tests: `PsionicCombat_Step11_Routes` (router: the wire shapes and refusals), the forms' unit
    tests, one e2e flow (raise a defence, attack twice to contact, defend once, undo) in
    `psionic-combat.spec`.

12. **Out of scope**: psychic lock and its 4d4 backlash (no two-sided power on one object in the
    app; a backlash is a manual PSP change, ADR 0043 step 5) _(Q14)_; the attack modes' effects as
    computed effects (ego whip's -5, psychic crush's 1d8 HP, psionic blast's 80%): the opponent's
    are the DM's, the character's own are entered with the existing HP and buff entries; an
    intellect fortress's cover of allies (other characters); the opponent's tangents on the
    character (Q7); wild talents (ADR 0043 step 23).

## Open questions

User, 2026-10-22: "default". Every default below is accepted; the steps stand as written.

Each is a reading where the book is unclear, silent or leaves the choice to the DM, or a choice
the app must make; the step is written with the default.

1. **Q1, raising a defence mode** (step 4). The book makes every power's use a power check, and the
   defence's check is also what each contest compares. Default: raising pays the cost with no roll,
   the checks are the contests'. Other reading: a check at the raise (ADR 0043 step 10, half the
   cost on a failure, no defence that round), then the contests' checks.
2. **Q2, one defence mode a round** (step 4). The book lets a defence go with one other power, and
   bars a second defence only beside mind blank. Default: one raised defence a round; raising
   another replaces it and pays again. Other reading: several defences, the defender choosing per
   attack.
3. **Q3, defence modes out of combat** (step 4). Default: a raised mode is refused with the round
   counter at 0 (a round-scoped power needs a round); a standing mode (cost 0, step 5) is not, as
   it is untimed. Other reading: allowed, lasting until the next round move.
4. **Q4, mind blank always up** (step 5). The book: known, it is nearly always active, down only
   when the player says so. Default: raised once by the player (free, no roll), up until dismissed,
   across rests. Other reading: up from the moment it is known, with an off switch.
5. **Q5, the cost of a double attack** (step 7.1). The book halves the cost of a failed check and
   gives one cost for two checks. Default: half when both fail, the whole cost otherwise. Other
   reading: the whole cost always.
6. **Q6, the opponent's rolls** (steps 7 and 8). Default: the player enters the DM's roll and
   whether it succeeded (the DM knows his score and Table 14). Other reading: the opponent's score
   entered, the app judging the roll.
7. **Q7, the opponent's tangents on the character** (step 8). Default: not tracked (the DM's);
   the journal shows each lost contest. Other reading: a row of tangents against the character.
8. **Q8, an attack on a contacted mind** (step 7.4). The book: once contacted, a mind may be
   crippled by another assault. Default: still a contest against its defence; a win shows the
   mode's effect, no tangent. Other reading: no contest once contacted.
9. **Q9, attack modes on an open mind** (step 7.4). The book does not bar them; three modes' texts
   speak of another psionicist's mind. Default: allowed, a successful check (no contest) shows the
   effect. Other reading: refused, the contact power being the way in.
10. **Q10, an attack mode's cost each round** (step 7.1). The book: the cost includes the double
    attack, an attack mode has no maintenance. Default: each use pays. Other reading: none (the
    same reading, said for the record).
11. **Q11, the third tangent on the first check** (step 7.3). Default: the second check is not
    rolled and the cost is not reduced. Other reading: the second check is rolled and shown with no
    effect.
12. **Q12, a power whose cost is contact** (step 10). Default: it costs its `plus` amount only (0
    for most), the contact is free while a power on the target is kept up, and ending the contact
    ends those powers. Other reading: the contact row charges its 1 PSP besides, or the powers
    outlive the contact.
13. **Q13, range modifiers** (step 7). Each attack mode's text gives -2 at medium and -5 at long
    range. Default: the player adds them as labelled modifiers. Other reading: range bands as
    catalogue data.
14. **Q14, an undefended mind and psychic lock** (steps 7.2 and 12). Default: Table 14 gives no
    modifier against no defence, and psychic lock is out of scope. Other reading: lock built with
    two-sided powers.
15. **Q15, a target above 20 levels or hit dice** (steps 1.2 and 9). Table 29 stops at 20.
    Default: refused `level_out_of_table`; the DM gives the cost on the contact route instead (step
    9), which still makes the contact row. Other
    reading: 18 for every level over 20.
16. **Q16, the optional language rule** (step 9). The book's option raises the contact cost for
    very intelligent minds. Default: not taken. Other reading: a ruleset switch (ADR 0041 step 2)
    with the target's languages entered.

## Proposed rounds

Each round holds at most three deliverables: the backend, its tests and wording, the web and its
e2e. The order follows the data each step needs.

1. **The data** (step 1): the format 18 builder in `critical-rulebooks` with its `scripts/check`
   cases; the API reader, schema and fixture; the rule-table lookups and their tests.
2. **Defence and the contest** (steps 4 to 6 and 8): backend (raise, the standing defence, the
   contest, defend, the zero-cost row's amendments); tests and wording (render rows, `apiError`
   keys, journal views); web (the current defence, raise, the defend form) and its e2e.
3. **Attack and tangents** (steps 2, 3 and 7): backend (the attack, the tangent rows and their
   charge); tests and wording; web (the attack form, the rows with their target) and its e2e.
4. **Contact** (steps 9 and 10): backend (the contact power, contact-cost powers, free contact,
   ending contact); tests and wording; web (the contact form, using a contact power on a target)
   and the e2e flow of step 11.

## Consequences

- A catalogue format 18 (three rule tables and a cost that names a table), built first in
  `critical-rulebooks`, read only by the API after it.
- Buff rows gain `target` and `tangents` (in the buff list's JSON, no migration); no opponent table.
- New entries `defence_raised`, `psionic_attack`, `psionic_defence`; new refusals `not_in_combat`,
  `mind_closed`, `no_contact`, `level_out_of_table`, `use_contact_route`; `needs_contact` no
  longer refuses a power whose initial cost is contact (step 10) and stays only for a maintenance
  cost of contact, which no catalogue power has. Each
  with its render row and `apiError` / journal key (en, fr).
- A new pure rule (`psionics` contest) and new psionicsvc operations (raise, attack, defend,
  contact); buffsvc charges the tangent and contact rows through ADR 0043's maintenance.
- The vocabulary: `GLOSSARY.md` and `docs/domain-model.md` take *tangent* (fr *tangente*),
  *contact*, *psychic contest* (fr *lutte psychique*, the French book's heading; psychic lock is its *impasse psychique*), *raised defence*
  and *standing defence* with the implementing items.
- Amended in place on implementation: ADR 0043 steps 6, 7, 10 and 11 (steps 4, 5 and 10) and its
  steps 20 to 22 (replaced by this ADR), ADR 0004 step 17 (step 3), ADR 0021 (step 1, a step 25).
