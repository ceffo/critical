import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Eye } from "@phosphor-icons/react";
import { fetchOthers } from "@/api/characters";
import { Button } from "@/components/ui/button";
import { groupByOwner } from "@/lib/others";
import { identityLine } from "@/lib/rosterCard";
import { viewPath } from "@/lib/viewed";
import type { SharedCharacter } from "@/types/character";

/**
 * The other users' characters under the roster's own cards (ADR 0029 step 11), grouped by owner. Each card
 * opens that character read-only (a full load on `/view/<id>`, so the user's current character is
 * never touched). Nothing shows while the list loads or when there is none; a failed read says so
 * without blocking the user's own characters.
 */
export function Others() {
  const { t } = useTranslation();
  const [list, setList] = useState<SharedCharacter[] | null>(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let live = true;
    fetchOthers().then(
      (l) => live && setList(l.characters),
      () => live && setFailed(true),
    );
    return () => {
      live = false;
    };
  }, []);
  if (failed)
    return (
      <p role="alert" className="text-body text-danger">
        {t("roster.othersError")}
      </p>
    );
  if (!list || list.length === 0) return null;
  return (
    <section
      aria-label={t("roster.othersTitle")}
      className="flex flex-col gap-3"
      data-testid="roster-others"
    >
      <h2 className="section-label">{t("roster.othersTitle")}</h2>
      {groupByOwner(list).map((g) => (
        <div
          key={g.id}
          className="flex flex-col gap-2"
          data-testid="roster-owner"
        >
          <h3 className="section-label [overflow-wrap:anywhere]">
            {g.name || t("roster.unnamedOwner")}
          </h3>
          <ul className="grid grid-cols-[repeat(auto-fill,minmax(19rem,1fr))] gap-3">
            {g.characters.map((c) => (
              <OtherCard key={c.id} character={c} />
            ))}
          </ul>
        </div>
      ))}
    </section>
  );
}

/** One other user's character: name, class and level, race · alignment, HP and Consulter. */
function OtherCard({ character: c }: { character: SharedCharacter }) {
  const { t } = useTranslation();
  const identity = identityLine(c, (key, code) =>
    t(`${key}.${code}`, { defaultValue: code }),
  );
  return (
    <li className="zone-card flex flex-col" data-testid="other-card">
      <div className="flex flex-col gap-1 px-4 pt-3">
        <span className="text-key [overflow-wrap:anywhere]">{c.name}</span>
        <span className="text-body">
          {c.classes
            .map((e) =>
              t("characters.classLevel", {
                className: t(`details.classes.${e.class}`, {
                  defaultValue: e.class,
                }),
                level: e.level,
              }),
            )
            .join(" / ")}
        </span>
        {identity && (
          <span className="text-body text-ink-muted [overflow-wrap:anywhere]">
            {identity}
          </span>
        )}
        {c.currentHp !== undefined && c.maxHp !== undefined && (
          <span className="text-body font-semibold">
            {t("roster.hp", { current: c.currentHp, max: c.maxHp })}
          </span>
        )}
      </div>
      <div className="mt-auto flex items-center gap-2 px-4 pt-3 pb-3">
        <Button
          variant="quiet"
          aria-label={t("roster.viewHint", { name: c.name })}
          title={t("roster.viewHint", { name: c.name })}
          onClick={() => globalThis.location.assign(viewPath(c.id))}
        >
          <Eye aria-hidden />
          {t("roster.view")}
        </Button>
      </div>
    </li>
  );
}
