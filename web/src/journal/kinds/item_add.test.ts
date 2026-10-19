// The item_add entry's view (kinds/inventory.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/item_add.json";

describeKind("item_add", fixtures, {
  added: {
    en: {
      text: "Added Rope (1)",
    },
    fr: {
      text: "Objet ajouté : Rope (1)",
    },
  },
});
