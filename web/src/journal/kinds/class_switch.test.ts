// The class_switch entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/class_switch.json";

describeKind("class_switch", fixtures, {
  switch: {
    en: {
      text: "Took a new class: Mage (left Fighter at level 6 with 21,000 XP)",
    },
    fr: {
      text: "Nouvelle classe : Magicien (ancienne classe : Guerrier, quittée au niveau 6 avec 21\u202f000 PX)",
    },
  },
});
