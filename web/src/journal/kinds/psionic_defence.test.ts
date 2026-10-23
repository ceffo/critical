// The psionic_defence entry's view (kinds/psionicDefence.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/psionic_defence.json";

describeKind("psionic_defence", fixtures, {
  contest: {
    en: {
      text: "Psychic contest (Psionic Blast against Mind Blank): defended, attacker won",
      short: "Psychic contest · attacker won",
    },
    fr: {
      text: "Duel psychique (Explosion psionique contre Esprit vide) : défendu, l’attaquant l’emporte",
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
