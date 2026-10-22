// The save entry's view (kinds/save.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/save.json";

describeKind("save", fixtures, {
  failed: {
    en: {
      text: "Save vs Spells: d20 12 +1 = 13 vs 14, FAILED",
      short: "Spells save · failed",
      subtitle: "Spells",
      card: ['data-testid="save-result"', "13 vs 14+"],
    },
    fr: {
      text: "Sauvegarde contre Sorts : d20 12 +1 = 13 contre 14, ÉCHOUÉE",
      short: "Sauvegarde Sorts · échouée",
      subtitle: "Sorts",
      card: ["13 contre 14+"],
    },
    value: {
      text: "12",
      tone: "fail",
    },
    rolled: true,
  },
  adjusted: {
    en: {
      text: "Save vs Spells: d20 12 +1 = 13 vs 12 (target 14, adjusted +2 Gaia): FAILED → PASSED",
      short: "Spells save · passed",
      subtitle: "Spells",
      card: ['data-testid="save-adjustment-line"', "13 vs 12+"],
    },
    fr: {
      text: "Sauvegarde contre Sorts : d20 12 +1 = 13 contre 12 (seuil 14, ajusté +2 Gaia) : ÉCHOUÉE → RÉUSSIE",
      short: "Sauvegarde Sorts · réussie",
      subtitle: "Sorts",
      card: ["13 contre 12+"],
    },
    value: {
      text: "12",
      tone: "pass",
    },
    rolled: true,
  },
  // ADR 0006 step 10.2: immune to the spell named, no d20, nothing to adjust.
  immune: {
    en: {
      text: "Save vs Spells against charm_person: immune (Wisdom 19), PASSED",
      short: "Spells save · immune",
      subtitle: "Spells",
      card: [
        'data-testid="save-immune"',
        "Save vs Spells · charm_person",
        "Wisdom 19: immune to charm_person, nothing to roll",
      ],
    },
    fr: {
      text: "Sauvegarde contre Sorts face à charm_person : immunité (Sagesse 19), RÉUSSIE",
      short: "Sauvegarde Sorts · immunité",
      subtitle: "Sorts",
      card: ["Sagesse 19 : immunité contre charm_person, aucun jet"],
    },
    value: null,
    rolled: false,
  },
});
