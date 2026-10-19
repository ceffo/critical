# ADR 0034: Onboarding screens: intro, full-screen creation, roster and cog menu

**Status**: accepted 2026-10-18 (the user approved the design round after three rounds: "approved";
the points marked _user_ are his words, the points marked _proposed_ are the designer's, accepted
with "the rest is good"). Not implemented. Settles the layout that
[ADR 0032](0032-character-identity-and-creation-rolls.md) left to the design round and amends
[ADR 0030](0030-classes-and-races.md) step 1 and [ADR 0029](0029-users-login-and-access.md) steps
7.2 and 7.3 (as amended by 0032). Follows [ADR 0027](0027-space-follows-frequency-of-use.md) (French
is the layout reference).

## Context

[ADR 0032](0032-character-identity-and-creation-rolls.md) made character creation a full-screen,
stepped route, moved the character actions out of the cog menu into a roster, and left every layout
to a design round. That round produced the mock-up pages (`web/design/onboarding/`), the screenshots
(`docs/design/onboarding/shots/`, `just onboarding-shots`) and the table of decisions in
[docs/design/onboarding/README.md](../design/onboarding/README.md), which names each screen's
content. This ADR records the decisions; the README and the shots are the picture, French first.

## Decision

1. **Intro** (_proposed_). One card: the brand, "Bienvenue à la table", one sentence on what the app
   does, the six steps listed, and "roll here or copy your paper sheet; the DM has the last word".
   It carries the language choice, Log out and **Commencer**. It shows on the first login only;
   Commencer goes to step 1 of the creation.
2. **Creation frame** (_proposed_, except 2.2).
   1. A header band (brand, "Nouveau personnage", **Quitter**); below, three columns: the step list
      (a done step shows its choice), the step card (title and lead; its content scrolls inside it;
      **Retour** / **Suivant** at its foot, the reason Suivant is blocked beside it) and "Votre
      personnage", the character filling in, as wide as the dashboard's character card
      (`clamp(280px, 21vw, 384px)`, [ADR 0027](0027-space-follows-frequency-of-use.md) step 5.1).
   2. (_user_) The frame is **capped at 1680 px and centred** on wide screens (2560x1440); below
      that it is full width.
3. **Steps** (_proposed_), in this order: class, race, scores, "Points de vie et talents", identity,
   review.
   1. **Class**: the class cards (hit die, minimums, alignment limit, races) and the starting level
      or XP (item 4).
   2. **Race**: all six races are shown; the ones the chosen class refuses are dimmed and marked
      ("Pas barde"), not pickable. Each card: adjustments, native language, height range, starting
      age.
   3. **Scores**: a **Tirer / Saisir** switch; the six methods of
      [ADR 0032](0032-character-identity-and-creation-rolls.md) step 6 as chips; the roll shows
      every die, the dropped one struck through; an arrangement table (the roll, the race
      adjustment, the score, the class minimums checked).
   4. **Points de vie et talents** is never empty: the HP roll, one row per level up to the starting
      level (die, Constitution, gain, total; the field stays editable), then the class extras (the
      bard's and thief's skill points against their budget, the armour worn, a fighter's 18/xx).
   5. **Identity**: name; alignment as a 3x3 grid; sex; languages; height, weight and age each with
      **Tirer**, their range and dice; hair; eyes.
   6. **Review**: the warnings on top, then one block per step with an edit button each; **Créer**.
4. **Starting level or XP** (_user_; amends [ADR 0030](0030-classes-and-races.md) step 1). Step 1
   offers **Niveau de départ** as a Niveau / XP switch, **Niveau by default**. Niveau shows the
   levels 1 to 20 as chips; **a level stands for the lowest XP of that level for the class** ("Niveau
   5 · 10 000 XP, le minimum du niveau 5 pour un barde"). XP lets the player type the exact value of
   a paper sheet. Either way the request carries **XP**: a picked level is sent as that XP, and the
   rules of ADR 0030 steps 1 and 3 (ranges, target level) are unchanged.
   1. **The level to XP table comes from the server** with the form's options, per class
      ([ADR 0032](0032-character-identity-and-creation-rolls.md) conventions: a rule the web shows
      comes from the server, the web computes nothing). It is the XP table of ADR 0030 step 3 read
      at each level's lower bound, for levels 1 to 20.
5. **Rolls** (_user_). **Relancer** stays on the score rolls and on the HP roll: the player may roll
   again. A roll is still a helper that writes nothing ([ADR 0032](0032-character-identity-and-creation-rolls.md)
   conventions).
6. **Refusals and warnings** (_proposed_; the rules are
   [ADR 0032](0032-character-identity-and-creation-rolls.md)'s). A race or an alignment the class
   refuses is shown, marked, not hidden. Clicking a forbidden alignment shows the **refusal in red**
   and blocks Suivant. **Warnings are amber and never block** (for example the languages over the
   Intelligence count, shown on the count).
7. **Roster** (_proposed_; amends [ADR 0029](0029-users-login-and-access.md) steps 7.2 and 7.3 as
   amended by 0032). A full-screen "Vos personnages": a card per character (name, class and level,
   race and alignment, HP, "En jeu" on the current one, a tag for a missing alignment or sex) with
   **Jouer**, **Renommer** and **Retirer**; a dashed **Créer** card opens the creation at step 1,
   and **Quitter** there returns to the roster. **Retour** goes back to the game.
   1. **Its way in**: a button right of the level in the character card's header.
   2. **Rename** is also in the character's details, which the name opens; **Nom** is the first
      field there.
8. **Cog menu** (_proposed_). Language, appearance, theme, account (email, Log out) and admin. No
   character action.

## Consequences

- The form's options gain the per-class level to XP table (item 4.1); the implementing item
  proposes its shape and tests it against ADR 0030 step 3's table, one level per class at least.
- Nothing about a rule changes: the picked level is only a way to type an XP, so the stored
  character, the API's creation inputs and ADR 0030 step 3's level derivation stay as they are.
- The UI items after the design round (the creation route, the identity step and review, the roster
  and cog menu clean-up) build to the README's table and the shots, and update `docs/frontend.md`.
- Not covered: the endpoints and rules (ADR 0029 steps 7.2 and 7.3 as built, ADR 0032), colours and
  themes ([ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md)), and narrow or mobile
  layouts ([ADR 0009](0009-dashboard-layout-and-theme-system.md) step 3).
