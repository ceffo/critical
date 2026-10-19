// The skill_points entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/skill_points.json";

describeKind("skill_points", fixtures, {
  spent: {
    en: {
      text: "Skill points: Open Locks 30 → 45",
    },
    fr: {
      text: "Points de talent : Crochetage 30 → 45",
    },
  },
});
