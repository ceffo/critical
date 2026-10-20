// The spell_add entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/spell_add.json";

describeKind("spell_add", fixtures, {
  given: {
    en: {
      text: "Added to the spellbook: sleep (no roll)",
    },
    fr: {
      text: "Ajouté au grimoire : sleep (sans jet)",
    },
    value: null,
    rolled: false,
  },
});
