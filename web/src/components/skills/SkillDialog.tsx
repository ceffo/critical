import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { getSkillDescription } from "@/api/skills";
import {
  DescriptionBody,
  DescriptionPopup,
  type Figure,
} from "@/components/proficiencies/ProficiencyDialog";
import { localized } from "@/i18n/localized";
import { skillName } from "@/journal/words";
import { formatSigned } from "@/lib/format";
import type { RogueSkill } from "@/types/skills";
import type { LocalizedText } from "@/types/localized";

/** What the description lookup gave for one skill of one set. */
type Load =
  { key: string; description: LocalizedText } | { key: string; failed: true };

/**
 * The pop-up of a skill of a skill set (ADR 0037 step 19.6), the proficiencies' pop-up for a
 * thief's or a bard's skill: its name, the description of that set for that skill in the UI
 * language (English where there is none), then the terms of its percentage, all the server's
 * figures (the web computes nothing): base, points, race, Dexterity, armour and the total. The
 * description is read when the pop-up opens (the API module keeps it).
 */
export function SkillDialog({
  set,
  skill,
  onClose,
}: {
  /** The skill set the skill is read in: the bard's Pick Pockets is not the thief's. */
  set: string;
  /** The skill with its terms; null: closed. */
  skill: RogueSkill | null;
  onClose: () => void;
}) {
  const { t, i18n } = useTranslation();
  const [load, setLoad] = useState<Load | null>(null);
  const id = skill?.id;
  const key = id === undefined ? undefined : `${set}/${id}`;
  useEffect(() => {
    if (key === undefined || id === undefined) return;
    let live = true;
    getSkillDescription(set, id).then(
      (found) => live && setLoad({ key, description: found.description }),
      () => live && setLoad({ key, failed: true }),
    );
    return () => {
      live = false;
    };
  }, [set, id, key]);
  const shown = load && load.key === key ? load : null;

  const rows: Figure[] = skill
    ? [
        {
          key: "base",
          label: t("skills.details.base"),
          value: `${skill.base}`,
        },
        {
          key: "points",
          label: t("skills.details.points"),
          value: `${skill.points}`,
        },
        {
          key: "race",
          label: t("skills.details.race"),
          value: formatSigned(skill.race),
        },
        {
          key: "dexterity",
          label: t("skills.details.dexterity"),
          value: formatSigned(skill.dexterity),
        },
        {
          key: "armor",
          label: t("skills.details.armor"),
          value: formatSigned(skill.armor),
        },
        {
          key: "total",
          label: t("skills.details.total"),
          value: t("skills.percent", { value: skill.percent }),
        },
      ]
    : [];

  return (
    <DescriptionPopup
      open={skill !== null}
      onClose={onClose}
      title={id === undefined ? "" : skillName(t, id)}
      closeLabel={t("skills.details.close")}
      testId="skill-details"
    >
      {skill && (
        <DescriptionBody
          prefix="skill"
          description={
            shown && "description" in shown
              ? localized(i18n.language, shown.description)
              : undefined
          }
          failed={shown !== null && "failed" in shown}
          failedText={t("skills.details.failed")}
          emptyText={t("skills.details.empty")}
          rows={rows}
        />
      )}
    </DescriptionPopup>
  );
}

/**
 * A skill's name as the button that opens its pop-up: keyboard-focusable, its own target beside
 * the row's other actions (the roll). It owns the pop-up's open state.
 */
export function SkillNameButton({
  set,
  skill,
  className,
  children,
}: {
  set: string;
  skill: RogueSkill;
  className?: string;
  children?: React.ReactNode;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const name = skillName(t, skill.id);
  const label = t("skills.details.open", { name });
  return (
    <>
      <button
        type="button"
        className={className}
        aria-label={label}
        title={label}
        onClick={() => setOpen(true)}
        data-testid={`skill-name-${set}-${skill.id}`}
      >
        {children ?? <span className="break-words">{name}</span>}
      </button>
      {open && (
        <SkillDialog set={set} skill={skill} onClose={() => setOpen(false)} />
      )}
    </>
  );
}
