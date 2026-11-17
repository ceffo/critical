import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ThiefPointsEditor } from "@/components/actions/ThiefPointsEditor";
import { PickEditor } from "@/components/proficiencies/PickEditor";
import {
  ProficienciesRead,
  SkillSetRead,
  WeaponsRead,
} from "@/components/skills/SkillsRead";
import { localized } from "@/i18n/localized";
import { primaryEntry } from "@/lib/classEntries";
import { sectionPending } from "@/lib/skillsPending";
import { useCharacterStore } from "@/stores/character";
import { useSkillsEditor } from "@/stores/skillsEditor";
import { useSkillsStore } from "@/stores/skills";
import { useTableStore } from "@/stores/table";
import type { ClassRef } from "@/types/character";
import type { SkillSet } from "@/types/skills";

/** The section being edited: a pool of picks, or `set:<code>` for a skill set's points. */
type Editing = "weapon" | "nonWeapon" | `set:${string}`;

/** One editor's block; `anchor` is what the level-up's links ask for and focus. */
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
 * The Skills activity (ADR 0037 step 15, amended): everything the character has, read at once: the
 * proficiencies with the languages, the weapon proficiencies and each skill set, every section
 * with its budget. A section's Edit turns the stage into that section's editor (the pick editor or
 * the points editor, with its own Cancel and Save), at any time and not only while something is
 * left to spend; a Save or a Cancel returns to the reading, from what is stored. An editor follows
 * what the server last derived (it remounts when the stored picks or points change, so an undo or
 * a redo shows). The level-up's links ask for an editor through the skills-editor store: the stage
 * opens it and takes focus.
 */
export function SkillsPanel() {
  const { t, i18n } = useTranslation();
  const refresh = useTableStore((s) => s.refresh);
  const skills = useSkillsStore((s) => s.skills);
  const character = useCharacterStore((s) => s.character);
  const cls = primaryEntry(character?.classes)?.class ?? "";
  const slots = character?.derived.proficiencySlots;
  const race = character?.raceCode;
  const languages = character?.languages;
  const asked = useSkillsEditor((s) => s.editor);
  const askedSet = useSkillsEditor((s) => s.set);
  const clearAsk = useSkillsEditor((s) => s.close);
  const [editing, setEditing] = useState<Editing | null>(null);
  // Bumped by an editor's Save and Cancel: it starts again from what is stored.
  const [fresh, setFresh] = useState<Record<string, number>>({});
  const done = (key: Editing) => {
    setFresh((f) => ({ ...f, [key]: (f[key] ?? 0) + 1 }));
    setEditing(null);
  };
  const panel = useRef<HTMLDivElement>(null);

  // The points editors read the skill sets as the server derived them, read again whenever the
  // sheet changes (a level, an undo, a redo, a pick).
  useEffect(() => {
    void refresh(["skills"]);
  }, [refresh, character]);

  const sets: SkillSet[] = skills?.skillSets ?? [];
  const entries = character?.classes;
  const className = (code: string) =>
    t(`details.classes.${code}`, { defaultValue: code });
  // The tag names the entries the server counted each pool on (a frozen entry is not one); shown
  // only for a character with several class entries, a multi-class or a dual-class one (a display
  // choice, no rule: the server decides which entries).
  const multi = (entries ?? []).length >= 2;
  const tagOf = (refs: ClassRef[] | undefined) =>
    multi && refs && refs.length > 0
      ? refs
          .map((r) =>
            t("skillsActivity.classTag", {
              class: className(r.class),
              level: r.level,
            }),
          )
          .join(" + ")
      : undefined;
  const classNames = (entries ?? []).map((e) => className(e.class)).join(" / ");

  // A request names an editor; the points editor's names a set or, without one, the first set
  // with points left. It waits for the sets to arrive. Adjusted while rendering (React's way to
  // derive state from a changing input); the effect below clears the store's request.
  const [taken, setTaken] = useState(false);
  if (!asked && taken) setTaken(false);
  if (asked && !taken) {
    if (asked === "skillPoints") {
      const target = askedSet
        ? sets.find((x) => x.set === askedSet)
        : (sets.find((x) => (x.budget?.left ?? 0) > 0) ?? sets[0]);
      if (target) {
        setTaken(true);
        setEditing(`set:${target.set}`);
      }
    } else {
      setTaken(true);
      setEditing(asked);
    }
  }
  useEffect(() => {
    if (asked && taken) clearAsk();
  }, [asked, taken, clearAsk]);

  // The editor that opened takes focus (the level-up's links, a keyboard user's Edit).
  useEffect(() => {
    if (!editing) return;
    const el = [
      ...(panel.current?.querySelectorAll<HTMLElement>("[data-anchor]") ?? []),
    ].find((b) => b.dataset.anchor === editing);
    el?.scrollIntoView({ block: "start" });
    el?.focus({ preventScroll: true });
  }, [editing]);

  const pickKey = (pool: "weapon" | "nonWeapon") =>
    JSON.stringify([
      pool === "weapon" ? slots?.weaponPicks : slots?.picks,
      pool === "weapon" ? null : languages,
      fresh[pool] ?? 0,
    ]);

  const editor = (() => {
    if (!editing) return null;
    if (editing === "weapon" || editing === "nonWeapon") {
      if (!slots || !cls || (editing === "weapon" && !slots.weapon))
        return null;
      return (
        <Block id={`skills-${editing}`} anchor={editing}>
          <PickEditor
            key={pickKey(editing)}
            pool={editing}
            cls={cls}
            race={race ?? ""}
            languages={languages ?? []}
            slots={slots}
            classes={classNames}
            onDone={() => done(editing)}
          />
        </Block>
      );
    }
    const set = sets.find((x) => `set:${x.set}` === editing);
    if (!set) return null;
    const name = localized(i18n.language, set.name);
    return (
      <Block
        id={`skills-points-${set.set}`}
        anchor={editing}
        title={sets.length > 1 ? name : undefined}
      >
        {sets.length > 1 && (
          <h3 className="px-2.5 text-body font-semibold">{name}</h3>
        )}
        <ThiefPointsEditor
          key={JSON.stringify([
            set.skills.map((s) => [s.id, s.points]),
            fresh[editing] ?? 0,
          ])}
          set={set.set}
          skills={set.skills}
          budget={set.budget}
          onDone={() => done(editing)}
        />
      </Block>
    );
  })();

  return (
    <div
      ref={panel}
      className="zone-card flex min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-y-auto"
      data-testid="skills-panel"
    >
      {editor ?? (
        <div
          className="grid grid-cols-1 items-start gap-3 min-[1400px]:grid-cols-2"
          data-testid="skills-read"
        >
          {slots && (
            <ProficienciesRead
              slots={slots}
              proficiencies={skills?.proficiencies ?? []}
              languages={languages ?? []}
              race={race ?? ""}
              tag={tagOf(slots.nonWeaponClasses)}
              pending={sectionPending(character, "nonWeapon")}
              onModify={() => setEditing("nonWeapon")}
            />
          )}
          {slots?.weapon && (
            <WeaponsRead
              slots={slots}
              tag={tagOf(slots.weaponClasses)}
              pending={sectionPending(character, "weapon")}
              onModify={() => setEditing("weapon")}
            />
          )}
          {sets.map((set) => (
            <SkillSetRead
              key={set.set}
              set={set}
              tag={tagOf([set.class])}
              pending={sectionPending(character, { set: set.set })}
              onModify={() => setEditing(`set:${set.set}`)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
