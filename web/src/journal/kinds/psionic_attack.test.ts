// The psionic_attack entry's view (kinds/psionicAttack.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/psionic_attack.json";

describeKind("psionic_attack", fixtures, {
  tangent: {
    en: {
      text: "Psionic attack (Ego Whip on Orc): d20 12 vs 13 (Success): attacker won; d20 2 vs 13 (Success): defended (tangents 1 → 2, PSPs 4)",
      short: "Psionic attack · attacker won",
    },
    fr: {
      text: "Attaque psionique (Cinglement de l'ego sur Orc) : d20 12 contre 13 (Réussi) : l’attaquant l’emporte ; d20 2 contre 13 (Réussi) : défendu (tangentes 1 → 2, PFP 4)",
      short: "Attaque psionique · l’attaquant l’emporte",
    },
    rolled: true,
  },
  contact: {
    en: {
      text: "Psionic attack (Mind Thrust on Goblin shaman): d20 7 vs 12 (Success): attacker won (contact made, PSPs 2, second power this round)",
      short: "Psionic attack · attacker won",
    },
    fr: {
      text: "Attaque psionique (Pression mentale sur Goblin shaman) : d20 7 contre 12 (Réussi) : l’attaquant l’emporte (contact établi, PFP 2, deuxième pouvoir ce tour)",
      short: "Attaque psionique · l’attaquant l’emporte",
    },
    rolled: true,
  },
  openMindBothFail: {
    en: {
      text: "Psionic attack (Ego Whip on Peasant): d20 19 vs 13 (Failed): defended; d20 20 vs 13 (Natural 20): defended (tangents on Orc ended, paid 2 of 4 PSPs)",
      short: "Psionic attack · defended",
    },
    fr: {
      text: "Attaque psionique (Cinglement de l'ego sur Peasant) : d20 19 contre 13 (Échec) : défendu ; d20 20 contre 13 (20 naturel) : défendu (tangentes sur Orc rompues, 2 PFP payés sur 4)",
      short: "Attaque psionique · défendu",
    },
    rolled: true,
  },
  openMindEffect: {
    en: {
      text: "Psionic attack (Ego Whip on Peasant): d20 9 vs 13 (Success): attacker won, effect to apply; d20 15 vs 13 (Failed): defended (PSPs 4)",
      short: "Psionic attack · attacker won",
    },
    fr: {
      text: "Attaque psionique (Cinglement de l'ego sur Peasant) : d20 9 contre 13 (Réussi) : l’attaquant l’emporte, effet à appliquer ; d20 15 contre 13 (Échec) : défendu (PFP 4)",
      short: "Attaque psionique · l’attaquant l’emporte",
    },
    rolled: true,
  },
});
