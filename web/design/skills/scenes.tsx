// The Skills round's screen registry (docs/design/skills/README.md): `?screen=<id>&lang=fr|en`.
import type { ReactNode } from "react";
import { BARD, FIGHTER, THIEF, THIEF_NEW } from "./data";
import {
  ActSkills,
  CreationScreen,
  EditorScreen,
  MultiTagged,
  OverviewA,
  PartTabs,
  PointsEditor,
  ProficiencyEditor,
  WeaponEditor,
} from "./screens";

const STEP = {
  class: ["Classe", "Class"],
  race: ["Race", "Race"],
  scores: ["Caractéristiques", "Ability scores"],
  hp: ["Points de vie", "Hit points"],
  profs: ["Compétences", "Proficiencies"],
  talents: ["Talents de voleur", "Thief skills"],
  identity: ["Identité", "Identity"],
  review: ["Récapitulatif", "Review"],
} satisfies Record<string, [string, string]>;
const SEPARATE: [string, string][] = [
  STEP.class,
  STEP.race,
  STEP.scores,
  STEP.hp,
  STEP.profs,
  STEP.talents,
  STEP.identity,
  STEP.review,
];

export const SCREENS: Record<string, () => ReactNode> = {
  "a-overview-bard": () => <OverviewA who={BARD} />,
  "a-overview-thief": () => <OverviewA who={THIEF} />,
  "a-overview-fighter": () => <OverviewA who={FIGHTER} />,
  "a-overview-multi": () => <MultiTagged />,
  "a-edit-proficiencies-bard": () => (
    <EditorScreen who={BARD}>
      <ProficiencyEditor who={BARD} open={["rogue", "priest"]} />
    </EditorScreen>
  ),
  "a-edit-weapons-thief": () => (
    <EditorScreen who={THIEF}>
      <WeaponEditor who={THIEF} open={["sword", "blunt"]} />
    </EditorScreen>
  ),
  "a-edit-weapons-fighter": () => (
    <EditorScreen who={FIGHTER}>
      <WeaponEditor who={FIGHTER} open={["sword", "polearm"]} />
    </EditorScreen>
  ),
  "creation-separate-proficiencies-thief": () => (
    <CreationScreen
      who={THIEF_NEW}
      steps={SEPARATE}
      current={4}
      lead={[
        "Les maîtrises d'armes et les compétences, avec les mêmes éditeurs qu'en jeu.",
        "Weapon proficiencies and proficiencies, with the same editors as in play.",
      ]}
    >
      <PartTabs
        who={THIEF_NEW}
        parts={["weapons", "proficiencies"]}
        current="proficiencies"
      />
      <ProficiencyEditor who={THIEF_NEW} open={["rogue"]} actions={false} />
    </CreationScreen>
  ),
  "creation-separate-points-thief": () => (
    <CreationScreen
      who={THIEF_NEW}
      steps={SEPARATE}
      current={5}
      lit={["dex"]}
      lead={[
        "Les points de talent, avec les curseurs du jeu.",
        "The skill points, with the sliders used in play.",
      ]}
    >
      <PointsEditor set={THIEF_NEW.sets[0]} control="slider" actions={false} />
    </CreationScreen>
  ),
  "act-skills-thief": () => <ActSkills who={THIEF} />,
  "act-skills-fighter": () => <ActSkills who={FIGHTER} />,
  "points-slider-bard": () => (
    <EditorScreen who={BARD} lit={["dex"]}>
      <PointsEditor set={BARD.sets[0]} control="slider" />
    </EditorScreen>
  ),
};

void LANG;
