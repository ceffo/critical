// The level_up entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/level_up.json";

describeKind("level_up", fixtures, {
  rolled: {
    en: {
      text: "Level 6: d6 4, CON +1, max HP +5 (30 → 35)",
    },
    fr: {
      text: "Niveau 6 : d6 4, CON +1, PV max +5 (30 → 35)",
    },
    value: {
      text: "+5",
      tone: "heal",
    },
    rolled: false,
  },
  fixed: {
    en: {
      text: "Level 16: max HP +2, fixed gain (85 → 87)",
    },
    fr: {
      text: "Niveau 16 : PV max +2, gain fixe (85 → 87)",
    },
    value: {
      text: "+2",
      tone: "heal",
    },
  },
});
