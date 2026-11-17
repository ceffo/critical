import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { offeredIds } from "@/components/proficiencies/pickEditor";
import { ProficiencyPicker } from "@/components/proficiencies/ProficiencyPicker";
import { servedCatalogue } from "@/components/proficiencies/catalogue";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { ABILITIES } from "@/types/character";
import { WeaponPicker } from "@/components/proficiencies/WeaponPicker";
import { Note } from "./kit";
import { LanguagesGroup } from "./languages";
import { classKey, draftClasses, type StepProps } from "./flow";

/**
 * The creation's proficiency step (ADR 0037 step 14, ADR 0034): the weapon editor and the
 * non-weapon editor of the Skills tab on the
 * draft. The slots and each pick's figures are the draft's derived block (`POST
 * /api/characters/draft`), asked again as the picks change; Créer sends the picks and the server
 * checks them as the editor's save does (nothing picked before: a draft over budget is refused).
 * Leftover slots never block: they are reminded. The languages are picked here too, next to the
 * non-weapon picks whose pool they spend (ADR 0034 step 3.5 as amended by ADR 0037 step 18). The
 * proficiencies the class grants free (ADR 0037 step 22.4, the draft's `free_proficiency_to_take`)
 * start in the picks at 0 slots, once per class: the player may remove one, which is then offered.
 */
export function ProficienciesStep(props: StepProps) {
  const { options, draft, update, errorFor, answer } = props;
  const { t } = useTranslation();
  const figures = answer.latest?.derived?.proficiencySlots ?? null;
  // The server's list for the draft (all its classes judged together, the weapon lift too).
  const weapons = figures?.weapons;
  const offered = offeredIds(answer.latest?.warnings);
  const offeredKey = offered.join(",");
  const classes = classKey(draft);
  const { picks, freeSeeded } = draft;
  useEffect(() => {
    if (!offeredKey || freeSeeded === classes) return;
    const free = offeredKey
      .split(",")
      .filter((id) => !picks.some((p) => p.id === id))
      .map((id) => ({ id, slots: 0 }));
    update({ picks: [...free, ...picks], freeSeeded: classes });
  }, [offeredKey, classes, freeSeeded, picks, update]);
  const classNames = draftClasses(draft)
    .map((c) => t(`details.classes.${c}`, { defaultValue: c }))
    .join(" / ");
  const [tab, setTab] = useState<"weapon" | "nonWeapon">("weapon");
  const scores: Record<string, number> = {};
  for (const a of ABILITIES) {
    const n = Number(draft.scores[a]);
    if (draft.scores[a] !== "" && Number.isFinite(n)) scores[a] = n;
  }
  return (
    <div className="flex min-w-0 flex-col gap-3">
      <Tabs
        value={tab}
        onValueChange={(id) => setTab(id as typeof tab)}
        className="contents"
      >
        <TabsList
          aria-label={t("creation.proficiencies.tabs")}
          className="flex w-fit gap-1 rounded-lg bg-raised p-1"
        >
          {(["weapon", "nonWeapon"] as const).map((id) => (
            <TabsTab
              key={id}
              value={id}
              data-testid={`creation-proficiency-tab-${id}`}
              className={cn(
                "rounded-md px-2 py-0.5 text-body",
                tab === id
                  ? "chip-chosen font-medium"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {id === "weapon"
                ? t("weaponProficiencies.section")
                : t("skills.proficiencies")}
            </TabsTab>
          ))}
        </TabsList>
      </Tabs>
      {tab === "weapon" ? (
        !weapons ? null : weapons.length > 0 ? (
          <WeaponPicker
            weapons={weapons}
            picks={draft.weaponPicks}
            onChange={(weaponPicks) => update({ weaponPicks })}
            figures={figures}
            errorFor={errorFor}
            classes={classNames}
            barClassName="bg-elevated"
          />
        ) : (
          <Note kind="info">{t("creation.proficiencies.weaponsNone")}</Note>
        )
      ) : !options.proficiencies || options.proficiencies.length === 0 ? (
        <Note kind="info">{t("creation.proficiencies.none")}</Note>
      ) : (
        <ProficiencyPicker
          catalogue={servedCatalogue(options, draft.cls, figures)}
          picks={draft.picks}
          onChange={(picks) => update({ picks })}
          figures={figures}
          errorFor={errorFor}
          offered={offered}
          scores={scores}
          barClassName="bg-elevated"
          aside={<LanguagesGroup {...props} pool={figures?.nonWeapon} />}
        />
      )}
    </div>
  );
}
