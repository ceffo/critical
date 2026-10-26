# ADR 0047: Psionicist screens: a data-driven Powers tab, one action bar, rest that stops what it pays

**Status**: accepted 2026-10-26 (the user approved the design round after three rounds: "yes" to
the whole design as listed; the points marked _user_ are his decisions, the points marked _proposed_
are the designer's, accepted by that "yes"). Not implemented. Settles the layout that
[ADR 0043](0043-psionics.md) and [ADR 0044](0044-psionic-combat.md) left to the design round
(CRI-241) and **amends ADR 0043 step 6** (Table 6 gains a proficiency-gated row, item 6 below).
Follows [ADR 0022](0022-activity-focus-layout.md), [ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md)
and [ADR 0027](0027-space-follows-frequency-of-use.md) (French is the layout reference).

## Context

ADR 0043 and ADR 0044 decided the rules (PSPs, powers, checks, maintenance, defence and attack modes,
tangents, contact) and left every screen to a design round. That round produced the mock-up pages
(`web/design/psionics/`), the screenshots (`docs/design/psionics/shots/`, `just psionics-shots`: ten
screens, French and English, two sizes) and the table of screens in
[docs/design/psionics/README.md](../design/psionics/README.md). This ADR records the decisions; the
README and the shots are the picture, French first. Rules of ADR 0043 and 0044 not named here stand.

What the user fixed before the last round (rounds 1 and 2):

- Psionics play like combat and spells: see what triggers, what is kept up, the consequences. Play
  lives on Act, browsing under Spells (the same split as combat and spells), for a single-class and a
  multi-class character.
- Attack, defence, power use, out-of-combat powers and an easy rest are all required.
- _User_: "you can't design with specific powers. the app should be DATA-DRIVEN. spells and their
  rules are the data" (ADR 0043 step 8, "a power's rules are its data").

## Decision

### The data-driven rule

1. **Every element on the Powers tab is keyed to a catalogue field, never to a power id.** A screen is
   a template over the catalogue data; a power the catalogue adds later appears with no screen change.
   The design pages hold two fixtures with different known powers to prove it (one with attack modes
   and a mind-taking power, one with neither). The mapping, from the README's table:
   - the tab exists when any class entry has a `psionics` feature;
   - "Ma défense" chips: known powers of `kind` `defence_mode`, priced by `initial_cost`; a
     `maintenance.cost` of 0 is a standing defence (ADR 0044 step 5), shown raised out of combat too;
   - the "Cible" band (the target's minds, open or closed, level, defence) shows only when a known
     power takes a mind: `kind` `attack_mode`, or an `initial_cost` of a table or contact;
   - the "Modes d'attaque" group, its score from Table 14 with the `attack_versus_defence` cell for the
     target's defence; the other groups are the known sciences and devotions by `discipline`;
   - row tags come from `kind`, `initial_cost` (a number, variable, by level, on contact +N) and
     `maintenance` (+N per round, per N rounds, variable, permanent);
   - the action bar's form follows the shape of `initial_cost` (item 2), and a Table 6 row with
     `requires_proficiency` shows when the character knows that proficiency (item 6).

   Contract for the attack rows' score to beat (built 2026-10-26): served, never computed by the web.
   `POST /api/psionics/attack/preview` with `{defence, modifiers}` (the defence the target raised, ""
   for none; the labelled modifiers as an attack takes them) answers `{defence, attacks: [{power,
   score, attackVersusDefence?, target}]}`, one entry per known attack mode in pick order: `score`
   is the mode's score with relearns and modifiers, `attackVersusDefence` Table 14's cell (absent
   with no defence), `target` their sum, the one each check of the attack rolls under. A mode the
   attack itself would refuse before any roll (a variable cost, a score only in its text, a
   maintenance paid on contact) keeps its row, with no figure and `refusal: {error, code, field?}`,
   the body that refusal has as an error, so the row words it through its `apiError` key; the call
   itself is refused only for a bad `defence` or `modifiers` and `not_a_psionicist`. The figure
   comes from the computation `Attack` rolls from
   (`TestAttackPreview_Step7_ServedFigureIsTheAttacksTarget`).

### The user's decisions

2. **One action bar, docked under the Powers lists, for every entry made before a roll** (_user_,
   answering "1A"): the DM's rolls for an attack mode, a variable cost (and maintenance), a table cost's
   row with one control per `contact_modifiers` kind (a menu for several rows, a toggle for one), the
   minds with a contact from an earlier round for a contact power, and an incoming attack. The row
   stays highlighted and the list never moves. The shape of `initial_cost` picks the form; the
   attack and the defence use the same bar.
3. **The tab header keeps "Pouvoir du tour : libre / <power>" and the "Ajustement" stepper** (_user_,
   "2 keep"). The first states the book's one power a round (the limit ADR 0044 Q2 works around for defence modes) as a
   warning, never a refusal. **"Attaque subie"**, in the "Ma défense" band, opens the same bar (_user_, "3 yes"): the
   DM's mode and rolls, then the defend result with Keep.
4. **Rest while a paying power is kept up is one button**, "Arrêter <power> et récupérer N h" (_user_,
   "4A"). ADR 0043 step 6 refuses a recovery while a power is maintained
   (`psp_recovery_while_maintaining`); the button stops the paying power(s) and recovers in **one
   transaction**, so one undo restores both. Rows that cost nothing (a standing defence, a free
   contact) stay up and do not block the rest. This is a new backend action (build item).
5. **"Méditation 12 / heure" shows only with the Rejuvenation proficiency** (_user_, "5 yes"), through
   a `requires_proficiency` field on the Table 6 rows (item 6).
6. **Table 6 gains the row the book prints and the data lacked** (amends ADR 0043 step 6): the
   rejuvenating rate of 12 PSPs an hour, a data row (id left to the build) beside `exertion`, `walking`,
   `resting` and `sleeping`, with `requires_proficiency` naming the proficiency. The `psp_recovery`
   request may name that activity only when the character knows it (otherwise refused, like an unknown
   activity); the field is generic, so another row may be gated the same way. Catalogue format 18
   needs the row (a data change in the rulebooks repo).
   Contract (built 2026-10-26): `GET /api/characters/{id}`'s `psp` pool carries
   `recovery: [{activity, perHour, requiresProficiency?}]`, every Table 6 row in the catalogue's
   order, so the rest control reads the rates and gates with the character; the key is omitted when
   the pool has none (a draft's). The answers of `POST /api/psp/*` stay `{current, max}`. No route
   serves the table alone. The start refuses a `requires_proficiency` that is not an id of
   `seeds/proficiencies.json` (`TestStartup_ADR0047Item6_RefusesAnUnknownRecoveryGate`); the gate
   itself is `TestRecover_ADR0047Item6_GatedActivityNeedsProficiency`.
7. **The character card reads, top to bottom: hit points, the HP change (field, Dégâts, Soins), then
   the PSP block** (_user_: "HP modifying should stay close to HP bar"). The PSP block holds the bar,
   the next round's balance, **Repos** and **Ajuster**.

### Proposed, accepted by the "yes"

8. **Groups in two columns**: the attack modes first, then the disciplines in learning order (the
   player's order, as ADR 0016 does for possessions).
9. **Defence chips are one tap, no roll.** A standing defence (maintenance 0) is shown as up; a paid
   one is dashed and reads "en combat" out of combat, since ADR 0044 step 4 refuses it with the round counter at 0.
10. **A "Maintenus" card** lists every buff row that keeps a power up (maintained powers, tangents,
    contacts; a contact is free while a power uses it), each with **Stop** and the next round's
    balance, which Next turn charges (ADR 0043 step 13).
11. **The rest popover** lists Table 6's rates, offers 1, 4 and 8 hours, previews the result ("71 → 89")
    and carries the long-rest line (ADR 0043 step 7: the night's sleep recovers 96, capped).
12. **Browsing is under Sorts > Pouvoirs** (the catalogue and the known powers); the Powers rail
    activity is removed.

## Migration path

Build items, one slice each, in this order:

1. Catalogue: the Table 6 meditating row and `requires_proficiency` (item 6; ADR 0043 step 6 data).
2. Backend: the stop-and-recover action (item 4), and `psp_recovery` honouring `requires_proficiency`.
3. Act > Pouvoirs: the defence band, the target band, the groups and the row tags (items 1, 3, 8, 9).
4. The action bar: the forms per `initial_cost` shape, the attack and the defence (item 2).
5. Contact (table cost) and contact power screens.
6. The Maintenus card and the upkeep shown on Next turn (item 10).
7. The PSP block and the rest popover (items 4, 5, 7, 11).
8. Sorts > Pouvoirs browsing (item 12).

Tests cite `ADR 0047 item N` where they build a decision here.

## Consequences

- A new power in the catalogue shows on the tab, in the bar and in the rest flow with no web change,
  provided its fields are the format 18 ones; a field a screen needs and the data lacks is a data
  change, never a special case on an id.
- Resting with a paid power up is one action instead of stop, then rest, with nothing to forget; the
  stop is undone with the recovery.
- The rest activity list depends on what the character knows; the web hides a gated row rather than
  showing it disabled.
- The design pages stay in `web/design/psionics/` as the artifacts of this round, as the equipment
  round's did.
