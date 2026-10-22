import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { LevelGrants, LevelUp } from "@/types/character";
import { GrantRows, LevelUpGrants } from "./LevelUpGrants";
import { ToSpendTag } from "./ToSpendTag";
import { LevelUpPrompt } from "./LevelUpPrompt";

const none: LevelGrants = {
  weaponSlots: 0,
  nonWeaponSlots: 0,
  skillPoints: 0,
  freeSpells: [],
};

async function i18n(lng: string) {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return inst;
}

async function render(
  levelUp: LevelUp,
  lng: string,
  realizedLevel = 15,
  targetLevel = 15,
) {
  const inst = await i18n(lng);
  return renderToStaticMarkup(
    <I18nextProvider i18n={inst}>
      <LevelUpPrompt
        levelUp={levelUp}
        realizedLevel={realizedLevel}
        targetLevel={targetLevel}
        busy={false}
        onRoll={() => Promise.resolve(null)}
        onOpenEditor={() => {}}
        onOpenFreeSpell={() => {}}
      />
    </I18nextProvider>,
  );
}

// ADR 0030 step 7: the server sends the pending levels (lowest first) with their die or fixed
// gain; the card prompts for the first one, French first.
describe("LevelUpPrompt (ADR 0030 step 7)", () => {
  it("prompts for the lowest pending level's hit die", async () => {
    const lu: LevelUp = {
      pending: [
        {
          class: "bard",
          classes: 1,
          level: 9,
          die: 6,
          fixedGain: 0,
          grants: none,
        },
        {
          class: "bard",
          classes: 1,
          level: 10,
          die: 6,
          fixedGain: 0,
          grants: none,
        },
      ],
      realizedAboveTarget: false,
      toSpend: none,
    };
    const fr = await render(lu, "fr", 8, 10);
    expect(fr).toContain("Niveau 9 atteint");
    expect(fr).toContain("Dé de vie d6, plus l&#x27;ajustement de CON");
    expect(fr).toContain(">Lancer<");
    expect(fr).toContain(">Saisir<");
    expect(fr).toContain("+1 autre niveau à lancer ensuite");
    const en = await render(lu, "en", 8, 10);
    expect(en).toContain("Level 9 reached");
    expect(en).toContain(">Roll<");
  });

  it("offers the fixed gain above the name level, with no die to enter", async () => {
    const lu: LevelUp = {
      pending: [
        {
          class: "bard",
          classes: 1,
          level: 16,
          die: 0,
          fixedGain: 2,
          grants: none,
        },
      ],
      realizedAboveTarget: false,
      toSpend: none,
    };
    const fr = await render(lu, "fr", 15, 16);
    expect(fr).toContain("Niveau 16 atteint");
    expect(fr).toContain("Au-delà du niveau de nom : +2 PV max, sans dé");
    expect(fr).toContain(">Ajouter<");
    expect(fr).not.toContain(">Saisir<");
    expect(fr).not.toContain("autre niveau");
  });

  it("shows nothing when no level is owed", async () => {
    expect(
      await render(
        { pending: [], realizedAboveTarget: false, toSpend: none },
        "fr",
      ),
    ).toBe("");
  });

  it("warns when the XP gives a level under the realized one (step 7.4)", async () => {
    const lu: LevelUp = {
      pending: [],
      realizedAboveTarget: true,
      toSpend: none,
    };
    expect(await render(lu, "fr", 16, 15)).toContain(
      "Niveau 16 acquis, l&#x27;XP donne le niveau 15",
    );
    expect(await render(lu, "en", 16, 15)).toContain(
      "Level 16 taken, XP gives level 15",
    );
  });
});

async function markup(lng: string, node: React.ReactNode) {
  const inst = await i18n(lng);
  return renderToStaticMarkup(
    <I18nextProvider i18n={inst}>{node}</I18nextProvider>,
  );
}

// ADR 0037 step 15: what a level grants beyond hit points, and what is left to spend, as the
// server counts them, each with the way into its editor; French first.
describe("level-up grants and the to-spend tag (ADR 0037 step 15)", () => {
  const rogue4: LevelGrants = {
    weaponSlots: 1,
    nonWeaponSlots: 1,
    skillPoints: 30,
    freeSpells: [],
  };
  const noop = () => {};

  it("lists the level's grants with a link to each editor", async () => {
    const panel = (
      <LevelUpGrants
        grants={rogue4}
        level={4}
        onOpenEditor={noop}
        onOpenFreeSpell={noop}
        onDismiss={noop}
      />
    );
    const fr = await markup("fr", panel);
    expect(fr).toContain("Le niveau 4 apporte");
    expect(fr).toContain("+1 emplacement de maîtrise d’arme");
    expect(fr).toContain("+1 emplacement de compétence");
    expect(fr).toContain("+30 points de talent");
    expect(fr).toContain('aria-label="Choisir les maîtrises d’armes"');
    expect(fr).toContain('aria-label="Choisir les compétences"');
    expect(fr).toContain('aria-label="Dépenser les points de talent"');
    expect(fr.match(/>Choisir</g)).toHaveLength(2);
    expect(fr).toContain(">Dépenser<");
    expect(fr).toContain('aria-label="Fermer ce que le niveau apporte"');
    expect(fr).not.toContain(">Fermer<");
    const en = await markup("en", panel);
    expect(en).toContain("Level 4 grants");
    expect(en).toContain("+30 skill points");
    expect(en).toContain("+1 weapon proficiency slot<");
  });

  it("leaves out a count of 0, and words what is left without the +", async () => {
    const fr = await markup(
      "fr",
      <GrantRows
        grants={{ ...none, nonWeaponSlots: 2 }}
        gained={false}
        onOpenEditor={noop}
        onOpenFreeSpell={noop}
      />,
    );
    expect(fr).toContain(">2 emplacements de compétence<");
    expect(fr).not.toContain("+2");
    expect(fr).not.toContain("maîtrise");
    expect(fr).not.toContain(">Dépenser<");
  });

  it("shows the to-spend tag while any count is above 0, none at 0", async () => {
    const tag = (toSpend: LevelGrants) => (
      <ToSpendTag
        toSpend={toSpend}
        onOpenEditor={noop}
        onOpenFreeSpell={noop}
      />
    );
    const fr = await markup("fr", tag({ ...none, skillPoints: 5 }));
    expect(fr).toContain(">À dépenser<");
    expect(fr).toContain('data-testid="to-spend-tag"');
    expect(await markup("en", tag({ ...none, weaponSlots: 1 }))).toContain(
      ">To spend<",
    );
    expect(await markup("fr", tag(none))).toBe("");
  });

  it("leaves the prompt alone: no reminder panel under it", async () => {
    const toSpend = { ...none, weaponSlots: 1 };
    const owed = await render(
      {
        pending: [
          {
            class: "bard",
            classes: 1,
            level: 5,
            die: 6,
            fixedGain: 0,
            grants: rogue4,
          },
        ],
        realizedAboveTarget: false,
        toSpend,
      },
      "fr",
      4,
      5,
    );
    expect(owed).toContain("Niveau 5 atteint");
    expect(owed).not.toContain("emplacement");
    expect(
      await render({ pending: [], realizedAboveTarget: false, toSpend }, "fr"),
    ).toBe("");
  });
});

// ADR 0030 step 20.8: a mage's free spell, in the level's grants and in what is left, with the way
// into the spell lookup on its spell level; French first.
describe("the level-up free spell (ADR 0030 step 20.8)", () => {
  const noop = () => {};
  const third = { level: 3, spellLevel: 2 };

  it("lists the level's free spell with a link that opens the lookup on its spell level", async () => {
    const panel = (
      <LevelUpGrants
        grants={{ ...none, nonWeaponSlots: 1, freeSpells: [third] }}
        level={3}
        onOpenEditor={noop}
        onOpenFreeSpell={noop}
        onDismiss={noop}
      />
    );
    const fr = await markup("fr", panel);
    expect(fr).toContain("Le niveau 3 apporte");
    expect(fr).toContain("+1 sort gratuit, niv. 2");
    expect(fr).toContain(
      'aria-label="Choisir le sort gratuit de niveau 2 dans la recherche de sorts"',
    );
    expect(fr).toContain('data-testid="grant-open-freeSpell-3-2"');
    const en = await markup("en", panel);
    expect(en).toContain("+1 free level 2 spell");
  });

  it("shows the to-spend tag for an owed free spell alone, and lists it without the +", async () => {
    const owed = { ...none, freeSpells: [third] };
    const fr = await markup(
      "fr",
      <ToSpendTag toSpend={owed} onOpenEditor={noop} onOpenFreeSpell={noop} />,
    );
    expect(fr).toContain(">À dépenser<");
    const rows = await markup(
      "fr",
      <GrantRows
        grants={owed}
        gained={false}
        onOpenEditor={noop}
        onOpenFreeSpell={noop}
      />,
    );
    expect(rows).toContain(">1 sort gratuit, niv. 2<");
    expect(rows).not.toContain("+1");
  });
});
