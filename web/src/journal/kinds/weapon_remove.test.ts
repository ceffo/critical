// The weapon_remove entry's view (kinds/inventory.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/weapon_remove.json";

describeKind("weapon_remove", fixtures, {
  removed: {
    en: {
      text: "Removed weapon Dagger",
    },
    fr: {
      text: "Arme Dagger retirée",
    },
  },
});
