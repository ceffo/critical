// The ability_check entry's view (kinds/abilityCheck.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/ability_check.json";

describeKind("ability_check", fixtures, {
  success: {
    en: {
      text: "STR check: d20 15 vs 16, success",
      short: "STR · success",
      subtitle: "Strength",
      card: ['data-testid="ability-result"'],
    },
    fr: {
      text: "Jet de FOR : d20 15 contre 16, réussi",
      short: "FOR · réussi",
      subtitle: "Force",
      card: ["15 contre 16"],
    },
    value: {
      text: "15",
      tone: "pass",
    },
    rolled: true,
  },
  adjusted: {
    en: {
      text: "DEX check: d20 14 vs 13 (15 −2 ledge), failure",
      short: "DEX · failed",
      subtitle: "Dexterity",
      card: ['data-testid="skill-adjustment-line"'],
    },
    fr: {
      text: "Jet de DEX : d20 14 contre 13 (15 −2 ledge), échec",
      short: "DEX · échec",
      subtitle: "Dextérité",
    },
    value: {
      text: "14",
      tone: "fail",
    },
    rolled: true,
  },
});
