// The item_remove entry's view (kinds/inventory.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/item_remove.json";

describeKind("item_remove", fixtures, {
  removed: {
    en: {
      text: "Removed Rope (2)",
    },
    fr: {
      text: "Objet retiré : Rope (2)",
    },
  },
});
