// The ac_override entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/ac_override.json";

describeKind("ac_override", fixtures, {
  set: {
    en: {
      text: "AC overridden: 4 (Shield spell)",
    },
    fr: {
      text: "CA forcée : 4 (Shield spell)",
    },
  },
  cleared: {
    en: {
      text: "AC override cleared",
    },
    fr: {
      text: "CA forcée effacée",
    },
  },
});
