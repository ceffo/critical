# Onboarding design round: intro, full-screen creation, roster, cog menu (2026-10-18)

The screens of the onboarding epic, for the UI items that build them (the creation route, the
identity step and review, the roster and cog menu clean-up). The rules behind every screen are
[ADR 0032](../../adr/0032-character-identity-and-creation-rolls.md) and
[ADR 0030](../../adr/0030-classes-and-races.md); this folder decides only where things sit.

- **Pages**: `web/design/onboarding/` (React on the app's own stylesheet, theme code, `Button`
  and Phosphor icons; data in `data.ts`: a half-elf bard created at level 5, PHB values).
- **Shots**: `just onboarding-shots` serves them on the app's Vite and writes
  `shots/<screen>--<fr|en>-<WxH>.png` at 1280x800 and 2560x1440, plus `-full.png` when a step card
  scrolls at that size. It fails on cut text or a page scroll; `--self-test` checks the audit.
  French is the layout reference (ADR 0027 step 2); English is audited too.

## Decisions

`user`: the user said it. `proposed`: the designer chose it and the user accepted it ("the rest is
good", round 1).

| Screen | Decision | By |
|---|---|---|
| `intro` | One card: the brand, "Bienvenue à la table", one sentence on what the app does, the six steps listed, "roll here or copy your paper sheet; the DM has the last word". Language, Log out, **Commencer**. First login only; then step 1. | proposed |
| creation frame | Header band (brand, "Nouveau personnage", **Quitter**); below, three columns: the step list (done steps show their choice), the step card (title, lead; content scrolls inside; **Retour** / **Suivant** at its foot, a blocking reason beside Suivant), and "Votre personnage", the character filling in, as wide as the dashboard's character card (`clamp(280px, 21vw, 384px)`). | proposed |
| creation frame | **Capped at 1680 px and centred** on wide screens (2560x1440); full width below that. | user |
| `1-class` | Class cards (hit die, minimums, alignment limit, races). **Niveau de départ**: a Niveau / XP switch, Niveau by default; levels 1 to 20 as chips; a level counts as the class's lowest XP for it ("Niveau 5 · 10 000 XP, le minimum du niveau 5 pour un barde"); XP takes the sheet's exact value. | user |
| `2-race` | All six races shown; the ones the class refuses dimmed and marked ("Pas barde"), not pickable. Each card: adjustments, native language, height range, starting age. | proposed |
| `3-scores` | **Tirer / Saisir**; the six methods as chips; the roll shows every die, the dropped one struck through; an arrangement table (roll, race adjustment, score, the class minimums checked). | proposed |
| `3-scores`, `4-hp-skills` | **Relancer** kept on the score and HP rolls. | user |
| `4-hp-skills` | Step 4 is "Points de vie et talents", never empty: the HP roll one row per level up to the starting level (die, CON, gain, total; the field stays editable), and the class extras (bard and thief skill points against the budget, armour worn; a fighter's 18/xx). | proposed |
| `5-identity` | Name, alignment as a 3×3 grid (forbidden ones marked; clicking one shows the **refusal** in red and blocks Suivant), sex, languages (native locked, the race's list bright, the others dimmed; the **warning** over the Intelligence count, never blocking), height / weight / age each with **Tirer**, the range and the dice, hair, eyes. | proposed |
| `6-review` | The warnings on top, then blocks with an edit button each; **Créer**. | proposed |
| `roster` | Full-screen "Vos personnages": a card per character (name, class · level, race · alignment, HP, "En jeu" on the current one, a tag for missing alignment or sex) with **Jouer** / **Renommer** / **Retirer**; a dashed **Créer** card opens the creation at step 1 (Quitter returns to the roster). **Retour** goes back to the game. | proposed |
| `character-card` | The roster opens from a button right of the level in the character card's header; the name opens the details, whose first field is **Nom** (Rename lives there too). | proposed |
| `cog-menu` | Language, appearance, theme, account (email, Log out), admin; no character actions. | proposed |

**For the ADRs** (the developer writes them): the level → XP table per class comes from the server
with the form's options (ADR 0032 conventions: the web computes no rule); a level picked is sent
as that XP. This adds to ADR 0030 step 1 (creation inputs) and ADR 0029 steps 7.2/7.3 as amended by
ADR 0032 (the roster's way in, Rename in the details).
