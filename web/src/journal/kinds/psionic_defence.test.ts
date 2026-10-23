// The psionic_defence entry's view (kinds/psionicDefence.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/psionic_defence.json";

describeKind("psionic_defence", fixtures, {
  contest: {
    en: {
      text: "Psychic contest (Psionic Blast against Mind Blank): d20 10 vs 10 (Success, +1 kept): defended; d20 17 vs 10 (Failed, +1 kept, a failure counts as 1): attacker won; check kept for the round (d20 10 vs 10)",
      short: "Psychic contest · attacker won",
    },
    fr: {
      text: "Duel psychique (Explosion psionique contre Esprit vide) : d20 10 contre 10 (Réussi, +1 gardé) : défendu ; d20 17 contre 10 (Échec, +1 gardé, un échec compte pour 1) : l’attaquant l’emporte ; jet gardé pour le tour (d20 10 contre 10)",
      short: "Duel psychique · l’attaquant l’emporte",
    },
    rolled: true,
  },
  fromKept: {
    en: {
      text: "Psychic contest (Psionic Blast against Mind Blank): d20 10 vs 10 (Success, +1 kept, kept for the round, no die): attacker won",
      short: "Psychic contest · attacker won",
    },
    fr: {
      text: "Duel psychique (Explosion psionique contre Esprit vide) : d20 10 contre 10 (Réussi, +1 gardé, gardé pour le tour, pas de dé) : l’attaquant l’emporte",
      short: "Duel psychique · l’attaquant l’emporte",
    },
    rolled: false,
  },
  noDefence: {
    en: {
      text: "Psychic contest (Psionic Blast, no defence): attacker won",
      short: "Psychic contest · attacker won",
    },
    fr: {
      text: "Duel psychique (Explosion psionique, sans défense) : l’attaquant l’emporte",
      short: "Duel psychique · l’attaquant l’emporte",
    },
    rolled: false,
  },
});
