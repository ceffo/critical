// The initiative correction's view (kinds/initiative.tsx) on its logged fixtures, in English and
// French (ADR 0035 item 6.2): a spell's casting time, a spell of a round or more, a magic item.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/initiative_correct.json";

describeKind("initiative_correct", fixtures, {
  spell: {
    en: {
      text: "Round 2 initiative corrected: 8 → 4 (Magic Missile: +1)",
      subtitle: "Round 2",
      card: ["<li>Magic Missile: +1</li>"],
    },
    fr: {
      text: "Initiative du tour 2 corrigée : 8 → 4 (Magic Missile : +1)",
      subtitle: "Tour 2",
    },
    value: { text: "4", tone: "ink" },
    rolled: false,
  },
  endOfRound: {
    en: {
      text: "Round 2 initiative corrected: 8 → 1 (Fly: end of round)",
      subtitle: "Round 2",
      card: ["<li>Fly: end of round</li>", "<li>Haste: −2</li>"],
    },
    fr: {
      text: "Initiative du tour 2 corrigée : 8 → 1 (Fly : fin du tour)",
      subtitle: "Tour 2",
    },
    value: { text: "1", tone: "ink" },
  },
  item: {
    en: {
      text: "Round 4 initiative corrected: 6 → 9 (Wand of lightning: +3)",
      subtitle: "Round 4",
    },
    fr: {
      text: "Initiative du tour 4 corrigée : 6 → 9 (Wand of lightning : +3)",
      subtitle: "Tour 4",
    },
    value: { text: "9", tone: "ink" },
  },
});
