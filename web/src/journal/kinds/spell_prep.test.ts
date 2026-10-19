// The spell_prep entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/spell_prep.json";

describeKind("spell_prep", fixtures, {
  prepared: {
    en: {
      text: "Prepared: Sleep ×2, Charm Person",
    },
    fr: {
      text: "Préparé : Sleep ×2, Charm Person",
    },
  },
  none: {
    en: {
      text: "Prepared no spells",
    },
    fr: {
      text: "Aucun sort préparé",
    },
  },
});
