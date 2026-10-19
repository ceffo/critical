// The equipment_state entry's view (kinds/inventory.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/equipment_state.json";

describeKind("equipment_state", fixtures, {
  equipped: {
    en: {
      text: "Equipped: Chain mail",
    },
    fr: {
      text: "Équipé : Chain mail",
    },
  },
  swapped: {
    en: {
      text: "Equipped: Plate mail (in place of Chain mail)",
    },
    fr: {
      text: "Équipé : Plate mail (à la place de Chain mail)",
    },
  },
  unequipped: {
    en: {
      text: "Unequipped: Chain mail",
    },
    fr: {
      text: "Retiré : Chain mail",
    },
  },
});
