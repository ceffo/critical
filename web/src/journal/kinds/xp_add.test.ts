// The xp_add entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/xp_add.json";

describeKind("xp_add", fixtures, {
  added: {
    en: {
      text: "XP +1,500: Goblin cave",
    },
    fr: {
      text: "XP +1\u202f500 : Goblin cave",
    },
  },
});
