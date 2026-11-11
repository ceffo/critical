import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { ThiefPointsEditor } from "@/components/actions/ThiefPointsEditor";
import { PickEditor } from "@/components/proficiencies/PickEditor";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { primaryEntry } from "@/lib/classEntries";
import { useCharacterStore } from "@/stores/character";
import { useSkillsEditor } from "@/stores/skillsEditor";
import { useSkillsStore } from "@/stores/skills";
import { useTableStore } from "@/stores/table";

/**
 * The editor of what the character card's "to spend" entry opens (ADR 0037 step 15, amended): the
 * weapon proficiencies, the non-weapon ones or a bard's or thief's skill points, in a dialog over
 * the page so that the Act activity's Skills tab only rolls. It hosts the same editors as ever, so
 * the server's refusals (ADR 0037 step 10) reach the player unchanged; Escape, a click outside or
 * the editor's own Cancel and Save close it.
 */
export function SpendDialog() {
  const { t } = useTranslation();
  const editor = useSkillsEditor((s) => s.editor);
  const editingSet = useSkillsEditor((s) => s.set);
  const close = useSkillsEditor((s) => s.close);
  const refresh = useTableStore((s) => s.refresh);
  const skills = useSkillsStore((s) => s.skills);
  const cls = useCharacterStore(
    (s) => primaryEntry(s.character?.classes)?.class ?? "",
  );
  const slots = useCharacterStore((s) => s.character?.derived.proficiencySlots);
  const race = useCharacterStore((s) => s.character?.raceCode);
  const languages = useCharacterStore((s) => s.character?.languages);
  // The points editor reads the skill sets as the server last derived them.
  useEffect(() => {
    if (editor === "skillPoints") void refresh(["skills"]);
  }, [editor, refresh]);

  // The points editor opened from the level-up's grants names no set: the first with points left.
  const set = skills
    ? ((editingSet
        ? skills.skillSets.find((x) => x.set === editingSet)
        : (skills.skillSets.find((x) => (x.budget?.left ?? 0) > 0) ??
          skills.skillSets[0])) ?? null)
    : null;

  let body = null;
  if ((editor === "weapon" || editor === "nonWeapon") && slots && cls)
    body = (
      <PickEditor
        pool={editor}
        cls={cls}
        race={race ?? ""}
        languages={languages ?? []}
        slots={slots}
        onDone={close}
      />
    );
  else if (editor === "skillPoints" && set)
    body = (
      <ThiefPointsEditor
        key={set.set}
        set={set.set}
        skills={set.skills}
        budget={set.budget}
        onDone={close}
      />
    );

  return (
    <Dialog open={editor !== null} onOpenChange={(open) => !open && close()}>
      <DialogContent
        className="max-w-[56rem] gap-3 overflow-y-auto"
        data-testid="spend-dialog"
      >
        <DialogTitle className="sr-only">
          {t("app.levelUp.toSpendTitle")}
        </DialogTitle>
        {body}
      </DialogContent>
    </Dialog>
  );
}
