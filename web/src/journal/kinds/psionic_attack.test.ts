// The psionic_attack entry's view (kinds/psionicAttack.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/psionic_attack.json";

describeKind("psionic_attack", fixtures, {
  tangent: {
    en: {
      text: "Psionic attack (Ego Whip on Orc): d20 12 vs 13 (Success): attacker won; d20 2 vs 13 (Success): defended",
      short: "Psionic attack · attacker won",
    },
    fr: {
      text: "Attaque psionique (Cinglement de l'ego sur Orc) : d20 12 contre 13 (Réussi) : l’attaquant l’emporte ; d20 2 contre 13 (Réussi) : défendu",
      short: "Attaque psionique · l’attaquant l’emporte",
    },
    rolled: true,
  },
  contact: {
    en: {
      text: "Psionic attack (Mind Thrust on Goblin shaman): d20 7 vs 12 (Success): attacker won",
      short: "Psionic attack · attacker won",
    },
    fr: {
      text: "Attaque psionique (Pression mentale sur Goblin shaman) : d20 7 contre 12 (Réussi) : l’attaquant l’emporte",
      short: "Attaque psionique · l’attaquant l’emporte",
    },
    rolled: true,
  },
  openMindBothFail: {
    en: {
      text: "Psionic attack (Ego Whip on Peasant): d20 19 vs 13 (Failed): defended; d20 20 vs 13 (Natural 20): defended",
      short: "Psionic attack · defended",
    },
    fr: {
      text: "Attaque psionique (Cinglement de l'ego sur Peasant) : d20 19 contre 13 (Échec) : défendu ; d20 20 contre 13 (20 naturel) : défendu",
      short: "Attaque psionique · défendu",
    },
    rolled: true,
  },
});
