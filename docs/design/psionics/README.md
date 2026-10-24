# Psionicist design round: the Powers tab on Act, rest, browsing (CRI-241)

**Round 2, not approved yet.** Every layout here is a proposal until the user approves it.

Settled by the user before round 2: psionics play like combat and spells (see what triggers, what
is kept up, the consequences); play lives on Act, browsing under Spells; single and multi-class;
attack, defence, power use, out-of-combat powers and an easy rest are required. Round 2's rule from
the user: "you can't design with specific powers. the app should be DATA-DRIVEN. spells and their
rules are the data" (ADR 0043 step 8, "a power's rules are its data").

- **Pages**: `web/design/psionics/` (React on the app's stylesheet, `Button`, Phosphor icons). Data in
  `data.ts`, shaped as catalogue format 18: the powers and the rule-table rows (Table 6, 14, 29 to 31)
  are copied from the catalogue; two fixtures with different known powers. `?screen=<id>&lang=fr|en`.
- **Shots**: `just psionics-shots` writes `shots/<screen>--<fr|en>-<WxH>.png` at 1920x1080 and
  2560x1440, and fails on cut text or a page scroll (`--fr`, `--only=a,b`, `--self-test`).

## The tab is a template over the data

| On screen | Comes from |
|---|---|
| The tab exists | a `psionics` class feature (any class entry) |
| "Ma défense" chips, one tap, no roll | known powers of `kind` `defence_mode`; price `initial_cost`; `maintenance.cost` 0 = standing (raised out of combat too) |
| "Cible" band (minds, open/closed, level, defence) | shown only when a known power takes a mind: `kind` `attack_mode`, or `initial_cost` a table or contact |
| "Modes d'attaque" group, score with Table 14 | `kind` `attack_mode`; `attack_versus_defence` cell for the target's defence |
| Other groups, in the player's order | `science` / `devotion`, grouped by `discipline` |
| Row tags | `kind`, `initial_cost` (number, variable, by level, on contact +N), `maintenance` (+N / tour, / N tours, variable, permanent) |
| Action bar (docked under the lists, one place) | attack mode: the DM's two rolls; `variable`: cost (and maintenance) fields; `{table}`: the table's row by level and one control per `contact_modifiers` kind (menu for several rows, toggle for one); `contact`: the minds with a contact from an earlier round; Incoming attack: the DM's mode and rolls |
| "Maintenus" card, Next turn's charge | the buff rows: maintained powers, tangent rows, contact rows (free while a power uses it) |
| Rest activities | Table 6 rows; a row with `requires_proficiency` shows when the proficiency is known (proposed field) |

## Screens

| Screen | Fixture | What it shows |
|---|---|---|
| `act-attack` | Joran | an attack mode's bar: the DM's two rolls against the target's defence |
| `act-attack-result` | Joran | the answer: third tangent, contact, second check not rolled |
| `act-defend` | Joran | defence raised this round, "Attaque subie" bar, the defend result with Keep |
| `act-table-cost` | Joran | a cost read from a table: level row and the modifier controls |
| `act-variable` | Joran | a variable cost and maintenance |
| `act-calm` | Joran | out of combat: paid defences wait for a round, a contact-cost power on an ally |
| `act-other` | Ysolde | no attack mode, no mind-taking power: no target band, other disciplines, same code |
| `rest-kept` | Joran | rest while a paying power is kept up: one button stops it and recovers |
| `rest-proficiency` | Ysolde | the proficiency-gated Table 6 row (Méditation, 12 / heure) |
| `lookup-powers` | Joran | browsing under Sorts > Pouvoirs: catalogue and known powers |
