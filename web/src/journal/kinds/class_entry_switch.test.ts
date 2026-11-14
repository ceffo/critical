// The class_entry_switch entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/class_entry_switch.json";

describeKind("class_entry_switch", fixtures, {
  switch: {
    en: { text: "Class changed: Mage to Transmuter" },
    fr: { text: "Classe changée : Magicien devient Transmutateur" },
  },
});
