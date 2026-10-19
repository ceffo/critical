// The attack entry's view (kinds/attack.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/attack.json";

describeKind("attack", fixtures, {
  hit: {
    en: {
      text: "Long Sword +2 vs AC 5: hit, 8 damage",
      short: "Attack · hit",
      subtitle: "Long Sword +2",
      card: ['data-testid="attack-result"', "Hits AC -3"],
    },
    fr: {
      text: "Long Sword +2 contre CA 5 : attaque réussie, 8 dégâts",
      short: "Attaque · touché",
      subtitle: "Long Sword +2",
      card: ["Touche CA -3"],
    },
    value: {
      text: "8",
      tone: "ink",
    },
    rolled: true,
  },
  crit: {
    en: {
      text: "Long Sword +2 vs AC 5: critical (check 7 · multiplier 63 → ×2), 16 damage",
      short: "Attack · critical",
      subtitle: "Long Sword +2",
      card: ['data-testid="crit-badge"'],
    },
    fr: {
      text: "Long Sword +2 contre CA 5 : critique (jet 7 · multiplicateur 63 → ×2), 16 dégâts",
      short: "Attaque · critique",
      subtitle: "Long Sword +2",
      card: ['data-testid="crit-badge"'],
    },
    value: {
      text: "16",
      tone: "crit",
    },
    rolled: true,
  },
  unknownAc: {
    en: {
      text: "Long Sword +2 vs unknown AC: hits AC 3",
      short: "Attack",
      subtitle: "Long Sword +2",
      card: ['data-testid="tell-gm"'],
    },
    fr: {
      text: "Long Sword +2 contre CA inconnue : touche CA 3",
      short: "Attaque",
      subtitle: "Long Sword +2",
      card: ['data-testid="tell-gm"'],
    },
    value: {
      text: "?",
      tone: "muted",
    },
    rolled: true,
  },
  corrected: {
    en: {
      text: "Long Sword +2 vs AC 7: miss (AC 5 → 7: HIT → MISS)",
      short: "Attack · missed",
      subtitle: "Long Sword +2",
      card: ['data-testid="ac-corrected"'],
    },
    fr: {
      text: "Long Sword +2 contre CA 7 : attaque ratée (CA 5 → 7 : TOUCHÉ → RATÉ)",
      short: "Attaque · raté",
      subtitle: "Long Sword +2",
      card: ['data-testid="ac-corrected"'],
    },
    value: {
      text: "–",
      tone: "muted",
    },
    rolled: true,
  },
});
