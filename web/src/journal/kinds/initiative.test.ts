// The initiative entry's view (kinds/initiative.tsx) on its logged fixtures, in English and French:
// an entry from before the modifiers (the d10 alone), and one with the weapon in hand's speed and an
// initiative effect (ADR 0035 item 6.3).
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
  withModifiers: {
    en: {
      text: "Initiative 10 for round 3 (d10 7, Long sword +5, Haste −2)",
      short: "Initiative",
      subtitle: "Round 3",
      card: ["<li>Long sword +5</li>", "<li>Haste −2</li>", ">10<"],
    },
    fr: {
      text: "Initiative 10 pour le tour 3 (d10 7, Long sword +5, Haste −2)",
      short: "Initiative",
      subtitle: "Tour 3",
      card: ['data-testid="initiative-terms"'],
    },
    value: {
      text: "10",
      tone: "ink",
    },
    rolled: true,
  },
});
