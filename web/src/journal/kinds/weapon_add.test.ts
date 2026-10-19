// The weapon_add entry's view (kinds/inventory.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/weapon_add.json";

describeKind("weapon_add", fixtures, {
  added: {
    en: {
      text: "Added weapon Dagger",
    },
    fr: {
      text: "Arme Dagger ajoutée",
    },
  },
});
