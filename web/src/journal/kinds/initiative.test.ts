// The initiative entry's view (kinds/initiative.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/initiative.json";

describeKind("initiative", fixtures, {
  rolled: {
    en: {
      text: "Initiative 7 (d10) for round 3",
      short: "Initiative",
      subtitle: "Round 3",
      card: ['data-testid="initiative-total"'],
    },
    fr: {
      text: "Initiative 7 (d10) pour le tour 3",
      short: "Initiative",
      subtitle: "Tour 3",
      card: ['data-testid="initiative-result"'],
    },
    value: {
      text: "7",
      tone: "ink",
    },
    rolled: true,
  },
});
