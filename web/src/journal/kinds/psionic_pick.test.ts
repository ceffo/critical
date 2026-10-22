// The psionic_pick entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/psionic_pick.json";

describeKind("psionic_pick", fixtures, {
  firstDiscipline: {
    en: { text: "Known powers: 0 → 1 picks" },
    fr: { text: "Pouvoirs connus : 0 → 1 choix" },
  },
  powerAndRelearn: {
    en: { text: "Known powers: 2 → 3 picks" },
    fr: { text: "Pouvoirs connus : 2 → 3 choix" },
  },
});
