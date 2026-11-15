import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ThiefPointsEditor } from "@/components/actions/ThiefPointsEditor";
import { PickEditor } from "@/components/proficiencies/PickEditor";
import { localized } from "@/i18n/localized";
import { primaryEntry } from "@/lib/classEntries";
import { useCharacterStore } from "@/stores/character";
import { useSkillsEditor } from "@/stores/skillsEditor";
import { useSkillsStore } from "@/stores/skills";
import { useTableStore } from "@/stores/table";
import type { SkillSet } from "@/types/skills";

/** One editor's block; `anchor` is what the level-up's links scroll to and focus. */
function Block({
  id,
  anchor,
  title,
  children,
}: {
  id: string;
  anchor: string;
  title?: string;
  children: ReactNode;
}) {
  return (
    <section
      data-anchor={anchor}
      tabIndex={-1}
      aria-label={title}
      data-testid={id}
      className="flex min-w-0 flex-col gap-2 rounded-md bg-raised py-3 outline-none"
    >
      {children}
    </section>
  );
}

/**
 * The Skills activity (ADR 0037 step 15, amended): the weapon proficiencies, the proficiencies
 * with the languages and each skill set's points, the editors as they are, open at any time and
 * not only while something is left to spend. Each block follows what the server last derived
 * (it remounts when the stored picks or points change, so an undo or a redo shows), and a Save or
 * a Cancel starts it again from what is stored. The level-up's links ask for an editor through
 * the skills-editor store: it scrolls there and takes focus.
 */
export function SkillsPanel() {
  const { i18n } = useTranslation();
  const refresh = useTableStore((s) => s.refresh);
  const skills = useSkillsStore((s) => s.skills);
  const cls = useCharacterStore(
    (s) => primaryEntry(s.character?.classes)?.class ?? "",
  );
  const slots = useCharacterStore((s) => s.character?.derived.proficiencySlots);
  const race = useCharacterStore((s) => s.character?.raceCode);
  const languages = useCharacterStore((s) => s.character?.languages);
  const asked = useSkillsEditor((s) => s.editor);
  const askedSet = useSkillsEditor((s) => s.set);
  const clearAsk = useSkillsEditor((s) => s.close);
  // Bumped by an editor's Save and Cancel: it starts again from what is stored.
  const [fresh, setFresh] = useState<Record<string, number>>({});
  const restart = (key: string) =>
    setFresh((f) => ({ ...f, [key]: (f[key] ?? 0) + 1 }));
  const panel = useRef<HTMLDivElement>(null);

  // The points editors read the skill sets as the server derived them, read again whenever the
  // sheet changes (a level, an undo, a redo, a pick).
  const character = useCharacterStore((s) => s.character);
  useEffect(() => {
    void refresh(["skills"]);
  }, [refresh, character]);

  const sets: SkillSet[] = skills?.skillSets ?? [];
  // A request names the points editor of a set; without one, the first set with points left.
  const askedKey =
    asked === "skillPoints"
      ? `set:${
          (askedSet
            ? sets.find((x) => x.set === askedSet)
            : (sets.find((x) => (x.budget?.left ?? 0) > 0) ?? sets[0])
          )?.set ?? ""
        }`
      : asked;
  useEffect(() => {
    if (!askedKey) return;
    const el = [
      ...(panel.current?.querySelectorAll<HTMLElement>("[data-anchor]") ?? []),
    ].find((b) => b.dataset.anchor === askedKey);
    // The points editors arrive with the skills: keep the request until their block exists.
    if (!el && asked === "skillPoints") return;
    el?.scrollIntoView({ block: "start" });
    el?.focus({ preventScroll: true });
    clearAsk();
  }, [askedKey, asked, sets.length, clearAsk]);

  const pickKey = (pool: string) =>
    JSON.stringify([
      pool === "weapon" ? slots?.weaponPicks : slots?.picks,
      pool === "weapon" ? null : languages,
      fresh[pool] ?? 0,
    ]);

  return (
    <div
      ref={panel}
      className="zone-card flex min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-y-auto"
      data-testid="skills-panel"
    >
      {slots?.weapon && cls && (
        <Block id="skills-weapon" anchor={"weapon"}>
          <PickEditor
            key={pickKey("weapon")}
            pool="weapon"
            cls={cls}
            race={race ?? ""}
            languages={languages ?? []}
            slots={slots}
            onDone={() => restart("weapon")}
          />
        </Block>
      )}
      {slots && cls && (
        <Block id="skills-nonWeapon" anchor={"nonWeapon"}>
          <PickEditor
            key={pickKey("nonWeapon")}
            pool="nonWeapon"
            cls={cls}
            race={race ?? ""}
            languages={languages ?? []}
            slots={slots}
            onDone={() => restart("nonWeapon")}
          />
        </Block>
      )}
      {sets.map((set) => (
        <Block
          key={set.set}
          id={`skills-points-${set.set}`}
          anchor={`set:${set.set}`}
          title={
            sets.length > 1 ? localized(i18n.language, set.name) : undefined
          }
        >
          {sets.length > 1 && (
            <h3 className="px-2.5 text-body font-semibold">
              {localized(i18n.language, set.name)}
            </h3>
          )}
          <ThiefPointsEditor
            key={JSON.stringify([
              set.skills.map((s) => [s.id, s.points]),
              fresh[`set:${set.set}`] ?? 0,
            ])}
            set={set.set}
            skills={set.skills}
            budget={set.budget}
            onDone={() => restart(`set:${set.set}`)}
          />
        </Block>
      ))}
    </div>
  );
}
