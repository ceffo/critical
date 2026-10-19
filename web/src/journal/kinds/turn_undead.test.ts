// The turn_undead entry's view (kinds/turnUndead.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/turn_undead.json";

describeKind("turn_undead", fixtures, {
  turned: {
    en: {
      text: "Turning undead (Ghoul): 7 turned, d20 12 vs 10",
      short: "Turning Ghoul · turned",
      subtitle: "Ghoul",
      card: ["Cleric level 3", 'data-testid="turn-turned"'],
    },
    fr: {
      text: "Renvoi des morts-vivants (Goule) : 7 repoussés, d20 12 contre 10",
      short: "Renvoi Goule · repoussés",
      subtitle: "Goule",
      card: ["Clerc de niveau 3"],
    },
    value: {
      text: "12",
      tone: "pass",
    },
    rolled: true,
  },
  destroyMore: {
    en: {
      text: "Turning undead (Skeleton): 5 destroyed, 3 turned, no roll",
      short: "Turning Skeleton · destroyed, 3 turned",
      subtitle: "Skeleton",
      card: ['data-testid="turn-destroyed"', 'data-testid="turn-turned"'],
    },
    fr: {
      text: "Renvoi des morts-vivants (Squelette) : 5 détruits, 3 repoussés, sans jet",
      short: "Renvoi Squelette · détruits, 3 repoussés",
      subtitle: "Squelette",
    },
    value: {
      text: "5",
      tone: "ink",
    },
    rolled: true,
  },
});
