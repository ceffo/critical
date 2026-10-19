// The item_edit entry's view (kinds/inventory.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/item_edit.json";

describeKind("item_edit", fixtures, {
  edited: {
    en: {
      text: "Edited Rope: count 1 → 2",
    },
    fr: {
      text: "Objet modifié : Rope, quantité 1 → 2",
    },
  },
  // ADR 0033 step 17: a power change holds the stored lists; the journal words them.
  editedPowers: {
    en: {
      text: "Edited Chain mail: powers none → magic +2 ; Saves +1 · AC −1 (without magic armour)",
    },
    fr: {
      text: "Objet modifié : Chain mail, pouvoirs aucun → magie +2 ; Sauvegardes +1 · CA −1 (sans armure magique)",
    },
  },
});
