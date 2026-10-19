// The weapon_edit entry's view (kinds/inventory.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/weapon_edit.json";

describeKind("weapon_edit", fixtures, {
  edited: {
    en: {
      text: "Edited weapon Dagger +1: to-hit bonus 0 → 1",
    },
    fr: {
      text: "Arme Dagger +1 modifiée : bonus au toucher 0 → 1",
    },
  },
});
