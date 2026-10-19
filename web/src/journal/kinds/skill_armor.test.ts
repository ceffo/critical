// The skill_armor entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/skill_armor.json";

describeKind("skill_armor", fixtures, {
  changed: {
    en: {
      text: "Skill armour: None → Elven chain",
    },
    fr: {
      text: "Armure des talents : Aucune → Cotte de mailles elfique",
    },
  },
});
