// The skill_check entry's view (kinds/skillCheck.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/skill_check.json";

describeKind("skill_check", fixtures, {
  proficiency: {
    en: {
      text: "Musical instrument: d20 17 vs 16, failure",
      short: "Musical instrument · failed",
      subtitle: "Musical instrument",
      card: ['data-testid="skill-result"', "17 vs 16"],
    },
    fr: {
      text: "Instrument de musique : d20 17 contre 16, échec",
      short: "Instrument de musique · échec",
      subtitle: "Instrument de musique",
      card: ["17 contre 16"],
    },
    value: {
      text: "17",
      tone: "fail",
    },
    rolled: true,
  },
  // ADR 0042 step 13: a dual-class's old-class skill before the crossover; the row and the card say it.
  proficiencyOldClass: {
    en: {
      text: "Musical instrument: d20 17 vs 16, failure. Warning: Your old class’s: the DM may cost you experience",
      short: "Musical instrument · failed",
      subtitle: "Musical instrument",
      card: ["Your old class’s: the DM may cost you experience"],
    },
    fr: {
      text: "Instrument de musique : d20 17 contre 16, échec. Attention : De votre ancienne classe : le MJ peut vous retirer de l’expérience",
      short: "Instrument de musique · échec",
      subtitle: "Instrument de musique",
      card: ["De votre ancienne classe"],
    },
    value: { text: "17", tone: "fail" },
    rolled: true,
  },
  // ADR 0014 step 1 as amended: the score in effect, the sheet's beside it.
  proficiencyUnderEffect: {
    en: {
      text: "Musical instrument: d20 18 vs 18, success",
      short: "Musical instrument · success",
      subtitle: "Musical instrument",
      card: ["d20 under DEX 19 (sheet 17) −1", "18 vs 18"],
    },
    fr: {
      text: "Instrument de musique : d20 18 contre 18, réussi",
      short: "Instrument de musique · réussi",
      subtitle: "Instrument de musique",
      card: ["DEX 19 (feuille 17) −1"],
    },
    value: { text: "18", tone: "pass" },
    rolled: true,
  },
  bardAdjusted: {
    en: {
      text: "Climb walls: d100 42 vs 50 (40 +10 rope), success",
      short: "Climb walls · success",
      subtitle: "Climb walls",
      card: ['data-testid="skill-adjustment-line"', "42 vs 50%"],
    },
    fr: {
      text: "Escalade : d100 42 contre 50 (40 +10 rope), réussi",
      short: "Escalade · réussi",
      subtitle: "Escalade",
      card: ["42 contre 50"],
    },
    value: {
      text: "42",
      tone: "pass",
    },
    rolled: true,
  },
  // ADR 0040 step 15: a skill-set check logs one kind, `rogueSkill`, with the set.
  rogueSkill: {
    en: {
      text: "Open Locks: d100 31 vs 55, success",
      short: "Open Locks · success",
      subtitle: "Open Locks",
      card: ["31 vs 55%"],
    },
    fr: {
      text: "Crochetage : d100 31 contre 55, réussi",
      short: "Crochetage · réussi",
      subtitle: "Crochetage",
      card: ["31 contre 55"],
    },
    value: { text: "31", tone: "pass" },
    rolled: true,
  },
  // ADR 0037 step 20: the buff the success put on, named from the proficiency.
  tumblingBuff: {
    en: {
      text: "Tumbling: d20 12 vs 17, success; Tumbling on: AC −4, 1 rd",
      short: "Tumbling · success",
      subtitle: "Tumbling",
    },
    fr: {
      text: "Acrobatie : d20 12 contre 17, réussi ; Acrobatie actif : CA −4, 1 tr.",
      short: "Acrobatie · réussi",
      subtitle: "Acrobatie",
    },
    value: {
      text: "12",
      tone: "pass",
    },
    rolled: true,
  },
  tumblingRenewed: {
    en: {
      text: "Tumbling: d20 5 vs 17, success; Tumbling renewed: AC −4, 1 rd",
      short: "Tumbling · success",
      subtitle: "Tumbling",
    },
    fr: {
      text: "Acrobatie : d20 5 contre 17, réussi ; Acrobatie renouvelé : CA −4, 1 tr.",
      short: "Acrobatie · réussi",
      subtitle: "Acrobatie",
    },
    value: {
      text: "5",
      tone: "pass",
    },
    rolled: true,
  },
  // ADR 0037 step 20.8: the renewing success adjusted to a failure; the line in effect says no buff.
  tumblingAdjustedToFailure: {
    en: {
      text: "Tumbling: d20 15 vs 14 (17 −3), failure",
      short: "Tumbling · failed",
      subtitle: "Tumbling",
    },
    fr: {
      text: "Acrobatie : d20 15 contre 14 (17 −3), échec",
      short: "Acrobatie · échec",
      subtitle: "Acrobatie",
    },
    value: {
      text: "15",
      tone: "fail",
    },
    rolled: true,
  },
});
