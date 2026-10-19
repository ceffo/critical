// The cast entry's view (kinds/cast.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/cast.json";

describeKind("cast", fixtures, {
  damage: {
    en: {
      text: "Cast Fireball: 17 fire damage",
      short: "Fireball",
      subtitle: "Fireball",
      card: [
        'data-testid="cast-entry-instances"',
        'data-testid="cast-entry-save"',
      ],
    },
    fr: {
      text: "Lancé : Fireball: 17 dégâts (fire)",
      short: "Fireball",
      subtitle: "Fireball",
      card: ['data-testid="cast-entry-damage"'],
    },
    value: {
      text: "17",
      tone: "ink",
    },
    rolled: true,
  },
  received: {
    en: {
      text: "Haste received (caster level 9): 12 rd",
      short: "Haste",
      subtitle: "Haste",
      card: ['data-testid="cast-entry-buff"'],
    },
    fr: {
      text: "Sort reçu : Haste (niveau du lanceur 9): 12 tr.",
      short: "Haste",
      subtitle: "Haste",
      card: ['data-testid="cast-entry-buff"'],
    },
    value: null,
    rolled: false,
  },
  // ADR 0004 step 20.5: a spell cast on an area, its row on the timeline marked as not on him.
  onOther: {
    en: {
      text: "Cast Sleep: 75 rd, not on you",
      short: "Sleep",
      subtitle: "Sleep",
      card: ['data-testid="cast-entry-buff"'],
    },
    fr: {
      text: "Lancé : Sleep: 75 tr., pas sur vous",
      short: "Sleep",
      subtitle: "Sleep",
      card: ['data-testid="cast-entry-buff"'],
    },
    value: null,
    rolled: false,
  },
  failed: {
    en: {
      text: "Cast Sleep: spell failed (d100 3 ≤ 10%)",
      short: "Sleep · spell failed",
      subtitle: "Sleep",
    },
    fr: {
      text: "Lancé : Sleep: échec du sort (d100 3 ≤ 10 %)",
      short: "Sleep · sort raté",
      subtitle: "Sleep",
    },
    value: {
      text: "3",
      tone: "fail",
    },
    rolled: false,
  },
  toHit: {
    en: {
      text: "Cast Shocking Grasp: hit (d20 15 vs AC 4)",
      short: "Shocking Grasp · hit",
      subtitle: "Shocking Grasp",
      card: ["To-hit roll against AC 4"],
    },
    fr: {
      text: "Lancé : Shocking Grasp: attaque réussie (d20 15 contre CA 4)",
      short: "Shocking Grasp · touché",
      subtitle: "Shocking Grasp",
      card: ["Jet pour toucher contre CA 4"],
    },
    value: {
      text: "15",
      tone: "pass",
    },
    rolled: true,
  },
});
