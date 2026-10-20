# ADR 0034: Onboarding screens: intro, full-screen creation, roster and cog menu

**Status**: accepted 2026-10-18 (the user approved the design round after three rounds: "approved";
the points marked _user_ are his words, the points marked _proposed_ are the designer's, accepted
with "the rest is good"). Steps 1 to 8 implemented (the creation route, the identity step and the
review, the roster and the cog menu), see "As built". Settles the layout that
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
   review. _(As built 2026-10-20: a "Compétences" step, the non-weapon proficiency picks of
   [ADR 0037](0037-proficiencies.md) step 14, sits between "Points de vie et talents" and identity,
   seven steps in all; the triage of the proficiency editor chose "a step or group in the creation
   flow". It uses the Skills tab's picker, its figures from ~~`POST /api/characters/checks`~~ the draft's derived block (`POST /api/characters/draft`), and
   never blocks Suivant: leftover slots are reminded.)_
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
      **Tirer**, their range and dice; hair; eyes. _(The languages move to the proficiencies step
      by [ADR 0037](0037-proficiencies.md) step 18, as built 2026-10-20.)_
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

## As built (the creation route: intro, class, race, scores, HP and talents)

2026-10-18, the item that built the route and its first four steps.

- **Options** (`GET /api/characters/options`, additive: a previous web ignores them). Each class gains
  `hitDie` (the sides of its group's die, ADR 0030 step 7) and `levelXp`, `[{level, xp}]` for levels 1
  to 20, read from ADR 0030 step 3's table at each level's lower bound (item 4.1). The answer gains
  `scoreMethods` (ADR 0032 step 6's table, the roll endpoint's `method` shape: `code`, `rolls`, `dice`,
  `dropLowest`, `assign`, `keep`, `base`, `max`) and `raceAdjustments` (`[{race, adjustments}]`, PHB
  Table 8 for the six races). Tested on the fixture's tables, level 5 of every class.
- **Web** (`web/src/components/creation/`, `docs/frontend.md`): the intro for a user with no current
  character, `/create` for the others (the cog menu's Create; the creation dialog is gone). Five steps
  for now: the identity step carries the former form's name and details inputs and Créer until the
  identity and review screens land. A server refusal opens its step with the field's error; Suivant
  is blocked only by a missing or unreadable value.
- **Leaving and late answers.** With values entered, Quitter, a reload and the browser's Back or
  Forward all ask before discarding (Back stays in the document, so it is held in
  `lib/navigation.ts` and asks the Quitter question). A server answer that lands after the draft
  changed again (a second race picked, a score edited during a roll) is dropped, its refusal too. The
  arrangement's answer is the whole truth: a field it no longer names is emptied unless the player
  typed over it. An HP roll whose class, Constitution or XP changed is dropped with its total (a
  typed total stays) and the step says to roll again (ADR 0032 step 7: one roll is for one class,
  Constitution and level).
- **Not shown yet**, for want of server data: the arrangement table's minimum check, the race cards'
  height and starting age (the identity step's dice), and the skill percentages on step 4 (the server
  warns at creation). The web computes none of them.
- **Class minimums and the skill budget** (added later). The options carry each class's ability
  minimums (PHB Table 13, `minimums` on the class: `ability`, `min`; Go data in `ruletablesvc`, like the
  alignment limits). They are shown, not enforced (ADR 0030 step 21's stance on requirements). The
  verdicts on a raw draft came from ~~`POST /api/characters/checks`~~ (removed 2026-10-20, see the
  draft endpoint below; it took `class`, `scores` entered so far, `xp`, `skillPoints`): `unmetMinimums`
  (for every creatable class, the abilities whose entered score is under the minimum; an ability without
  a score is not unmet) and the thief's or bard's budget at the level the XP gives. "Meets the minimum"
  and "points left" are rules math, so the server answers them and the web only words them (the class
  cards flag the unmet ones, the skills step shows "Restants"). Tests:
  `TestCreationOptions_Table13_EachClassCarriesItsAbilityMinimums`; the draft's tests below.

## As built (the identity step and the review)

2026-10-19, the item that finished the creation route.

- **Preview** (~~`POST /api/characters/preview`~~, removed 2026-10-20 for the draft endpoint below). The creation's body, checked and derived as
  `POST /api/characters` would, storing nothing: `{derived, warnings, languages, missing}`, the derived
  block and warnings in `GET /api/character`'s shapes, the languages with the native one added, and
  `missing` the blank required fields (`name`, `alignment`, `sex`), which are no refusal here so the
  identity step gets the server's warnings while they are blank. Every other refusal is the creation's,
  field and code alike (one validation, `charactersvc` `validate`; tested for parity in composition
  the composition tests, and the preview equals the created character's read for each class). The web asks it
  on the identity step and the review, without an alignment the class's list leaves out (that refusal is
  shown from the options), and drops a reply to an older draft.
- **Options** gain `raceIdentity`: by race, `height` and `weight` per sex and `age`, each
  `{count, sides, min, max}` (the roll's dice and ADR 0032 step 5.1's range, stored units), and
  `startAge` (the starting ages a roll gives). The identity step shows them beside Tirer; the race
  cards' height and starting age (item 3.2) can now be shown and are not yet.
- **Web** (`components/creation/identity.tsx`): items 3.5 and 3.6 as the shots, French first. The
  review's figures are the preview's (THAC0, the five saves, the bard's or thief's percentages); its
  height and weight are the body's numbers, so what shows is what is stored. The interim step is gone.

## As built (the roster and the cog menu)

2026-10-19, the item that built steps 7 and 8. ADR 0029 steps 7.2 and 7.3 (as amended by 0032) now
read as built here; their endpoints are unchanged and no backend change was made.

- **Roster** at `/characters`, reached from the button right of the level in the character card's
  header (step 7.1). Each card has Jouer (but the current one), Renommer and Retirer, the current one is
  "En jeu", and the dashed Créer card opens the creation at step 1, whose Quitter returns to the roster
  (it used to return to the game). Retour goes back to the game. Rename and Retire are dialogs over the
  roster and work on any of the user's characters, not only the one in play (ADR 0029 amendment item 2
  allowed it; the cog menu's dialogs only acted on the current one); their answer, the list, is shown as
  it is, and retiring the last character loads the page again (the intro).
- **Difference with the mock-up: the cards show the name, class and level and "En jeu" only.** The
  race, alignment, HP and the tag for a missing alignment or sex are not in `GET /api/characters` (its
  summary is `id`, `name`, `class`, `level`), and this item used the existing endpoints only. They come
  with an additive change to that summary, a follow-up item.
- **Rename in the details** (step 7.2): Nom is the first row of the details and the first field of their
  form. Saving a changed name renames it on `PATCH /api/characters/{id}` before the details are saved on
  `PATCH /api/character/details`; a refused name stops before the details and shows under Nom.
- **Cog menu** (step 8): language, appearance, theme, account and admin; the Character group and its
  dialogs are gone.
- **No current character.** A user with none and no character sees the intro (step 1); with other
  characters, the roster without Retour (no game to go back to). The intro no longer lists other
  characters. Through the API this second case does not arise (retiring the current character makes the
  first one left current), so it is a guard. A current character retired in another tab is ADR 0029 step
  7.2's `character_changed`: back in the game, the page loads again on the one the server made current,
  or shows the intro when none is left.

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

## Amendment 2026-10-20: the languages leave the identity step (decided; the languages as built 2026-10-20, the draft endpoint on implementation)

[ADR 0037](0037-proficiencies.md) step 18 (architecture review 2): the languages are edited on
the creation's proficiencies step and in the Skills tab's editor, as a pool next to the non-weapon
picks; the identity step and the details card show no language chips to edit (the card lists them
read-only). The creation's draft answers come from one endpoint, `POST /api/characters/draft`,
which replaces `POST /api/characters/checks` and `POST /api/characters/preview` (review item S3):
it takes the draft as far as it is entered and answers the figures it can derive, every refusal by
field and the fields still missing, never a 400 for a draft; `POST /api/characters` answers as
today. What each step shows (step 3) and what blocks Suivant (step 6) are unchanged.
_(As built 2026-10-20, the languages: the chips sit on the proficiencies step between the weapons
and the non-weapon picks, with the pool's line from the checks; the preview now also runs on that
step, so an overspend's refusal shows at the chips as it did on the identity step. The draft
endpoint as built 2026-10-20: see below.)_

## Amendment 2026-10-20: the spellbook step (decided, on implementation)

[ADR 0030](0030-classes-and-races.md) step 20.7: the creation flow of a **mage** or a **bard** has
a **spellbook step**, after the proficiencies step and before the review. The player enters the
spells the DM gave, without learn rolls, within the limits at the starting level; a mage has Read
Magic and Detect Magic pre-ticked, removable. It never blocks Suivant when empty. The draft
endpoint refuses a spell by field and code and the create request stores the entries as
`starting`. A class without a spellbook shows no such step. The step's layout (French first, ADR
0027) and the request's shape are the implementation's.

## As built (the draft endpoint)

2026-10-20, the item that built the amendment's draft endpoint (architecture review 2, S3), under
[ADR 0038](0038-domain-and-wire.md).

- **`POST /api/characters/draft`** replaces `POST /api/characters/checks` and `POST
  /api/characters/preview`, both removed (the web was their only client). The body is the creation's,
  every field optional (a number left out or null is blank). The answer is always 200 for the body's
  content: `derived` (`GET /api/character`'s derived block, one mapping shared with it; null while an
  entry the derivation reads, the class, race, six scores, skill points or XP, is missing or refused;
  a refused `hpMax` or `strExceptional` leaves it, the derivation reads neither), `skillPoints` (see
  "The budget" below), `warnings` and `languages` (the native one added, also while the picks or
  languages are refused: they are then read as entered) as the character's read gives them, `refusals` (`[{field, code}]`, every refusal `POST
  /api/characters` would give, field and code alike: the creation's `error` for an invalid input,
  `over_budget` with `spent` and `limit`), `missing` (the required fields still blank, in the form's
  order: `name`, `class`, `race`, the six scores, `hpMax`, `xp`, `alignment`, `sex`) and
  `unmetMinimums` (for every creatable class). A body that is no JSON or names an unknown field is
  still a 400; a height or weight that is no number is the draft's one refusal. `POST /api/characters`
  answers as before.
- **One validation.** `charactersvc` reads a new character field by field once (`check`): the
  creation stops at the first refusal, the draft goes on and lists them all. The picks and the
  languages are judged by the skills module's one entry point, `skills.Apply` (the editor's preview
  and save use it too), at the starting level. The derived block gains `skillPoints` (the budget with
  its formula's terms, `GET /api/skills`' `budget` shape, now one `rest/wire` DTO), which the extras
  step reads; additive for `GET /api/character`.
- **The budget** (decided in review, 2026-10-20). A thief's or a bard's skill point budget reads the
  class and the level the XP gives, nothing else (ADR 0030 steps 17 and 18), and the scores step lets
  Suivant through with a score out of range. So the draft carries the budget on its own, top-level
  `skillPoints` in the derived block's shape (`rest/wire`'s one DTO), set as soon as the class and XP
  read, whatever the race, scores or refused skill points (then read as none entered); null for another
  class or while the class or XP is missing or refused. With a derived block it equals the block's.
  The over-budget refusal is listed whenever the budget is read. The extras step reads this field, so
  a Strength of 2 still shows the budget there, as the old checks did; every ability-dependent figure
  (the skills' percentages, THAC0, saves) stays in the derived block and waits for valid scores.
- **Web.** One flow module (`components/creation/flow.ts`, the former `lib/creationFlow.ts` folded
  in), one request builder (`buildBody`, the draft's and Créer's; an unreadable value is left out and
  named), one answer type (`CreationDraft`). The class step's minimums, the extras step's budget and
  the proficiency step's pools read the last answer for the draft's class; the warnings, the review's
  figures and the refusals read the answer to the draft as it stands, and the refusals show as the
  preview showed them (once the draft was seen from the proficiency step on). What each step shows and
  what blocks Suivant are unchanged.
- **Tests**: `TestDraft_IsWhatTheCreatedCharacterReads` (the preview's equality, moved),
  `TestDraft_RefusesAsTheCreationRefuses` (a row per creation refusal),
  `TestDraft_PartialDraftsAnswer200`, `TestDraft_Table13_ScoresUnderAClassMinimumAreUnmet`,
  `TestDraft_Steps17And18_TheBudgetAndWhatIsLeft`, `TestDraft_Steps17And18_TheBudgetNeedsTheClassAndXPOnly`,
  `TestDraft_ListsEveryFaultCreateNamesTheFirst`, `TestDraft_RefusedPicksKeepTheFiguresAndTheNativeLanguage`,
  `TestCharacters_DraftWireShape`, `TestCharacters_DraftNeverRefusesContent`; web `flow.test.ts`
  (`buildBody`, `stepOfField`); e2e `layout.spec` (thief: a Strength of 2 keeps the budget on the
  extras step).
