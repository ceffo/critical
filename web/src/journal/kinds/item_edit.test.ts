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
});
