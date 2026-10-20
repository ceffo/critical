// The proficiencies entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/proficiencies.json";

describeKind("proficiencies", fixtures, {
  picked: {
    en: { text: "Proficiency slots: Tumbling 1 → 2, Tracking 0 → 2" },
    fr: { text: "Emplacements de compétence : Acrobatie 1 → 2, Pistage 0 → 2" },
  },
  removed: {
    en: { text: "Proficiency slots: old_skill 1 → 0" },
    fr: { text: "Emplacements de compétence : old_skill 1 → 0" },
  },
});
