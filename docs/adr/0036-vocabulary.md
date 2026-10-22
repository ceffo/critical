# ADR 0036: Vocabulary: the glossary, the UI words and the spelling

**Status**: accepted 2026-10-18 (the user's rulings in the glossary round; the glossary itself
proposed by the agent and checked against the ADRs, the code and both locale files). Amended
2026-10-21: steps 1 and 8, the glossary is terms only and the relations move to a domain model.

## Context

Thirty-five ADRs, a Go domain of twenty-odd services, a React web and two locale files had grown
their words separately. The same thing had several names: a buff was "Effect", "Buff", "Bonus",
"effet" or "buff" depending on the screen; the combat round was "round" in the API and the ADRs,
"Tour" in 35 French strings and "round" in 23 others, and `turn` in the web's code; the DM was
"GM" or "DM" in English and "MJ" or "MD" in French; a saving throw was "sauvegarde" or
"protection"; a possession was an item, a piece, a possession or equipment; the character sheet
was "fiche" or "feuille". The French PHB in the rulebooks clone was consulted (it says round, jet
de protection, MD) and offered; the user chose the words the table uses.

User, 2026-10-18: "it's probably time we get some order in this repo"; on the sheet: "what counts
is: character sheet/feuille de personnage. the google thing we don't care, it's not part of the
domain, it was just for ingestion, to kickstart the whole thing".

## Decision

1. **One glossary, terms only.** [`GLOSSARY.md`](../../GLOSSARY.md) at the repo root is the
   vocabulary of the one context. A term gives the English name, the French UI word, a one-line
   meaning and the ADR that holds its rules; it never restates a rule and never explains how
   terms relate (amended 2026-10-21; user: "the domain model is not the glossary [...] so that an
   agent can consult the glossary without bloating extra information"). A term for a concept not
   built yet carries _(planned)_ or _(future)_. A PR that introduces a concept adds its term in the same PR;
   a PR that renames one updates the glossary and this ADR's list below when a UI word changes.
   `docs/all.md` points to it instead of keeping its own list.
2. **The domain term and the UI word may differ.** Code, the API's codes and the ADRs use the
   domain term; the UI uses the UI word in both languages. The glossary lists both. Today's
   pairs: Buff is shown as Effect / Effet (saved buffs as Saved effects / Effets enregistrés,
   the journal kind as Effect / Effet); the action log is shown as the Journal / Journal; a
   received spell is "Received spell" / "Sort reçu".
3. **The French UI follows the table, not the French PHB.** The user's rulings: a round is
   **tour** (Tour suivant, Numéro du tour; "round" leaves the French, exposure included); a
   saving throw is **jet de sauvegarde** (abbreviation JS); the DM is **MJ**; a check is a
   **jet** (Jet de FOR, Ajuster ce jet; "test" leaves the French); the character sheet is the
   **feuille de personnage** ("fiche" leaves the French); the spellbook is the **grimoire**.
4. **The English UI follows the ADRs.** The DM is **DM** (never GM); the round is **round**;
   a buff is an **Effect**; one possession is an **Item** and the list is the **Inventory**.
5. **Item, Inventory, Equipment.** An item is one possession of any category (weapon, ammunition,
   armour, shield, gem…, [ADR 0033](0033-equipment.md) step 2 once built); the inventory is the
   character's list and its screen; "equipment" names ADR 0033's topic and the catalogue's
   reference data (format 7), never one item. "Piece" and "possession" are not used in prose or
   UI; ADR 0033 and ADR 0035 are read with this substitution.
6. **Spelling.** British in prose, UI and new identifiers (armour, catalogue, favourite,
   localised texts excepted where a library fixes the spelling). Existing identifiers and wire
   codes (`skill_armor`, `armor_type`, `spelleffectsvc.Catalog`) are not renamed for spelling
   alone.
7. **Legacy names are not vocabulary.** `fil`, `moi` (the ADR 0009 columns), `turn` for the
   round in the web are legacy (and the `hp_events` table was, until it was dropped); nothing new
   uses them, and renaming them is an ordinary refactor item, not a rule change.

8. **A domain model beside the glossary** (2026-10-21; user: "from now on, we have to be careful to
   build a coherent and complete glossary and domain model"). [`docs/domain-model.md`](../domain-model.md)
   holds the entities, their relations (Mermaid `erDiagram` / `classDiagram`), their invariants,
   the flow of the mechanics (which ADR computes what) and an "Ambiguities resolved" section. It
   never restates an ADR's numbers. A PR that adds or changes a concept updates both files.
9. **The books' words win for rules, the user's for the table.** Where the user's word differs
   from the book's (multi-class and dual-class, which the user calls "dual" and "serial"), the
   glossary uses the book's term and notes the user's.

Checks: `web/src/i18n/avoidedWords.test.ts` reads the values of both locale files and refuses the
avoided words (fr: `MD`, `fiche`, `round`, `test de`, `buff` and a `bonus` that means a buff, `livre
de sorts`, `possession`, `sort lancé sur moi`, `jet de protection`; en: `GM`, `buff`, `spell cast on me`, `possession`, `piece`); keys
and magic bonuses (`bonus au toucher`) are not words of the text. A new avoided word is a line in
that test.

## Consequences

- The locale files follow steps 3 and 4; `avoidedWords.test.ts` keeps them there. The
  weapon form's kind field (melee or missile) reads "Mêlée ou tir" / "Melee or missile", so that
  "Type" stays free for ADR 0033's category and the weapon type field. A duration the
  book gives in turns is shown in rounds (ten per turn), in both languages, so "tour" means one
  thing in French.
- ADR 0004's buff, ADR 0005's round, ADR 0006's saving throw and ADR 0016's item keep their
  names; each carries a one-line pointer to this ADR for its UI word.
- The "sheet differences" of ADR 0030 step 24 compare the figures imported from the Google
  sheet; the user does not count that sheet as domain. The feature was retired on 2026-10-19
  (ADR 0030 step 24); the glossary does not name it as a term.
- The glossary is not a spec: implementation details, endpoints and table names stay in
  `docs/backend.md` and the ADRs.
