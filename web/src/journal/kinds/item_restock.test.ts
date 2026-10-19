// The item_restock entry's view (kinds/inventory.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/item_restock.json";

describeKind("item_restock", fixtures, {
  restocked: {
    en: {
      text: "Restocked Arrows: +2 (12 now)",
    },
    fr: {
      text: "Objet réapprovisionné : Arrows, +2 (12 maintenant)",
    },
    value: null,
  },
});
