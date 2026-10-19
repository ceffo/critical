// The xp_edit entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/xp_edit.json";

describeKind("xp_edit", fixtures, {
  edited: {
    en: {
      text: "XP edited: Goblin cave, 1,500 → 1,800 XP",
    },
    fr: {
      text: "XP modifiés : Goblin cave, 1\u202f500 → 1\u202f800 XP",
    },
  },
});
