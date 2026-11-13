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
  weaponPicked: {
    en: { text: "Weapon proficiency slots: Long sword 0 → 1, Dagger 0 → 1" },
    fr: {
      text: "Emplacements de maîtrise d’arme : Épée longue 0 → 1, Dague 0 → 1",
    },
  },
  specialized: {
    en: {
      text: "Proficiency slots: Tracking 0 → 2 · Weapon proficiency slots: Long sword 1 → 2 (specialization)",
    },
    fr: {
      text: "Emplacements de compétence : Pistage 0 → 2 · Emplacements de maîtrise d’arme : Épée longue 1 → 2 (spécialisation)",
    },
  },
  // ADR 0037 step 21.7: picks by catalogue code, named from the equipment catalogue (the code
  // when it is not loaded, the type's word when the code is a type's).
  weaponCodes: {
    en: {
      text: "Weapon proficiency slots: Long sword 1 → 2 (specialization), heavy_horse_lance 0 → 1",
    },
    fr: {
      text: "Emplacements de maîtrise d’arme : Épée longue 1 → 2 (spécialisation), heavy_horse_lance 0 → 1",
    },
  },
  languages: {
    en: { text: "Proficiency slots: Tumbling 1 → 2 · Languages: + gnomish" },
    fr: {
      text: "Emplacements de compétence : Acrobatie 1 → 2 · Langues : + gnomish",
    },
  },
  languagesOnly: {
    en: { text: "Languages: − orcish, − gnomish" },
    fr: { text: "Langues : − orcish, − gnomish" },
  },
});
