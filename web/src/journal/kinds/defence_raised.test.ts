// The defence_raised entry's view (kinds/defenceRaised.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/defence_raised.json";

describeKind("defence_raised", fixtures, {
  raised: {
    en: {
      text: "Defence raised (Mind Blank): PSPs −1",
      short: "Defence raised · Mind Blank",
    },
    fr: {
      text: "Défense dressée (Esprit vide) : PFP −1",
      short: "Défense dressée · Esprit vide",
    },
    rolled: false,
  },
  standing: {
    en: {
      text: "Defence raised (Thought Shield, standing): PSPs 0",
      short: "Defence raised · Thought Shield",
    },
    fr: {
      text: "Défense dressée (Bouclier de pensée, permanente) : PFP 0",
      short: "Défense dressée · Bouclier de pensée",
    },
    rolled: false,
  },
});
