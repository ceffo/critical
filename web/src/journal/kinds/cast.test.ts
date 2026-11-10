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
  // ADR 0008 step 17: a heal on the bearer, cut by the max-HP cap (15 rolled, 8 applied).
  healCapped: {
    en: {
      text: "Cast Cure Serious Wounds: heals 8 HP (15 rolled, capped at max HP)",
      short: "Cure Serious Wounds",
      subtitle: "Cure Serious Wounds",
      card: [
        'data-testid="cast-entry-heal"',
        "HP 12 → 20",
        "15 rolled, capped at max HP",
      ],
    },
    fr: {
      text: "Lancé : Cure Serious Wounds: soigne 8 PV (15 obtenus, plafonnés aux PV max)",
      short: "Cure Serious Wounds",
      subtitle: "Cure Serious Wounds",
      card: ["PV 12 → 20", "15 obtenus, plafonnés aux PV max"],
    },
    value: { text: "+8", tone: "heal" },
    rolled: true,
  },
  // A full heal received: nothing rolled, the gap is the heal.
  healFull: {
    en: {
      text: "Heal received (caster level 12): heals 14 HP",
      short: "Heal",
      subtitle: "Heal",
      card: ['data-testid="cast-entry-heal-total"', "HP 6 → 20"],
    },
    fr: {
      text: "Sort reçu : Heal (niveau du lanceur 12): soigne 14 PV",
      short: "Heal",
      subtitle: "Heal",
      card: ["PV 6 → 20"],
    },
    value: { text: "+14", tone: "heal" },
    rolled: false,
  },
  // Cast on someone else: rolled and shown, no HP changed.
  healOther: {
    en: {
      text: "Cast Cure Light Wounds: heals 5, not on you",
      short: "Cure Light Wounds",
      subtitle: "Cure Light Wounds",
      card: [
        'data-testid="cast-entry-heal-other"',
        "Heals 5, not on you: no HP changed.",
      ],
    },
    fr: {
      text: "Lancé : Cure Light Wounds: soigne 5, pas sur vous",
      short: "Cure Light Wounds",
      subtitle: "Cure Light Wounds",
      card: ["Soigne 5, pas sur vous : aucun PV modifié."],
    },
    value: null,
    rolled: true,
  },
  // ADR 0006 step 10.3: received by a character immune to it, nothing applied or rolled.
  immune: {
    en: {
      text: "Charm Person received (caster level 5): immune (Wisdom 19)",
      short: "Charm Person",
      subtitle: "Charm Person",
      card: [
        'data-testid="cast-entry-immune"',
        "Your Wisdom (19) makes you immune to this spell: nothing applied.",
      ],
    },
    fr: {
      text: "Sort reçu : Charm Person (niveau du lanceur 5): immunité (Sagesse 19)",
      short: "Charm Person",
      subtitle: "Charm Person",
      card: [
        "Votre Sagesse (19) vous immunise contre ce sort : rien n’est appliqué.",
      ],
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
