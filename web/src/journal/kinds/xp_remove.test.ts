// The xp_remove entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/xp_remove.json";

describeKind("xp_remove", fixtures, {
  removed: {
    en: {
      text: "XP removed: XP earned (1,500 XP)",
    },
    fr: {
      text: "XP retirés : XP gagnés (1\u202f500 XP)",
    },
  },
});
