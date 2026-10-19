// The hp_change entry's view (kinds/hpChange.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/hp_change.json";

describeKind("hp_change", fixtures, {
  damage: {
    en: {
      text: "orc axe",
      short: "Damage",
      card: ['data-testid="info-value"', "orc axe"],
    },
    fr: {
      text: "orc axe",
      short: "Dégâts",
    },
    value: {
      text: "−6",
      tone: "damage",
    },
    rolled: false,
  },
  cappedHeal: {
    en: {
      text: "potion",
      short: "Healing",
      card: ["+12 requested, capped at max HP"],
    },
    fr: {
      text: "potion",
      short: "Soins",
      card: ["+12 demandés, plafonnés aux PV max"],
    },
    value: {
      text: "+5",
      tone: "heal",
    },
    rolled: false,
  },
  noSource: {
    en: {
      text: "Damage 4",
      short: "Damage",
    },
    fr: {
      text: "Dégâts 4",
      short: "Dégâts",
    },
    value: {
      text: "−4",
      tone: "damage",
    },
  },
});
