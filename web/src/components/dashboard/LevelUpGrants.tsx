import { useTranslation } from "react-i18next";
import { X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import type { FreeSpell, LevelGrants } from "@/types/character";
import type { SkillsEditor } from "@/stores/skillsEditor";

/** The editor each grant is spent in (the Skills tab's, ADR 0037 steps 7 and 12; ADR 0030 steps
 * 17 and 18), in the order the card lists them. */
const ROWS: {
  key: "weaponSlots" | "nonWeaponSlots" | "skillPoints";
  editor: SkillsEditor;
  action: "choose" | "spend";
}[] = [
  { key: "weaponSlots", editor: "weapon", action: "choose" },
  { key: "nonWeaponSlots", editor: "nonWeapon", action: "choose" },
  { key: "skillPoints", editor: "skillPoints", action: "spend" },
];

/** A row's link is as tall as a line of text (18 px), so a one-line row stays one line high and
 * the card does not scroll for the panel at 1440x900 (ADR 0022 step 2.8). */
const ROW_BUTTON = "h-[1.125rem]";

/**
 * Slots and skill points beyond hit points (ADR 0037 step 15) as the server counts them, one row
 * per count above 0 with the way into its editor ("Choisir" / "Dépenser"), then a mage's free
 * spells (ADR 0030 step 20.8), each with the way into the spell lookup on its spell level:
 * `gained` words them as what a level grants ("+n"), otherwise as what is left.
 */
export function GrantRows({
  grants,
  gained,
  onOpenEditor,
  onOpenFreeSpell,
}: {
  grants: LevelGrants;
  gained: boolean;
  onOpenEditor: (editor: SkillsEditor) => void;
  onOpenFreeSpell: (fs: FreeSpell) => void;
}) {
  const { t } = useTranslation();
  return (
    <ul className="flex flex-col gap-0.5">
      {ROWS.filter((r) => grants[r.key] > 0).map((r) => {
        const what = t(`app.levelUp.grant.${r.key}`, {
          count: grants[r.key],
        });
        const actionTitle = t(`app.levelUp.open.${r.editor}`);
        return (
          <li
            key={r.key}
            className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3"
            data-testid={`grant-${r.key}`}
          >
            <span className="min-w-0 break-words text-muted-foreground">
              {gained ? t("app.levelUp.gained", { what }) : what}
            </span>
            <Button
              variant="quiet"
              size="xs"
              className={ROW_BUTTON}
              aria-label={actionTitle}
              title={actionTitle}
              onClick={() => onOpenEditor(r.editor)}
              data-testid={`grant-open-${r.editor}`}
            >
              {t(`app.levelUp.${r.action}`)}
            </Button>
          </li>
        );
      })}
      {grants.freeSpells.map((fs) => {
        const what = t("app.levelUp.grant.freeSpell", {
          spellLevel: fs.spellLevel,
        });
        const actionTitle = t("app.levelUp.open.freeSpell", {
          spellLevel: fs.spellLevel,
        });
        return (
          <li
            key={`free-${fs.level}-${fs.spellLevel}`}
            className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3"
            data-testid="grant-freeSpell"
          >
            <span className="min-w-0 break-words text-muted-foreground">
              {gained ? t("app.levelUp.gained", { what }) : what}
            </span>
            <Button
              variant="quiet"
              size="xs"
              className={ROW_BUTTON}
              aria-label={actionTitle}
              title={actionTitle}
              onClick={() => onOpenFreeSpell(fs)}
              data-testid={`grant-open-freeSpell-${fs.level}-${fs.spellLevel}`}
            >
              {t("app.levelUp.choose")}
            </Button>
          </li>
        );
      })}
    </ul>
  );
}

/**
 * What the level just taken grants beyond hit points (ADR 0037 step 15), under the level-up prompt
 * until closed: a title with a close icon, the rows and their links, each line one text line high
 * (ADR 0022 step 2.8: the card does not scroll for it at 1440x900, a three-line name included).
 * Nothing here blocks play. The caller shows it
 * only when a count is above 0.
 */
export function LevelUpGrants({
  grants,
  level,
  onOpenEditor,
  onOpenFreeSpell,
  onDismiss,
}: {
  grants: LevelGrants;
  level: number;
  onOpenEditor: (editor: SkillsEditor) => void;
  onOpenFreeSpell: (fs: FreeSpell) => void;
  onDismiss: () => void;
}) {
  const { t } = useTranslation();
  const title = t("app.levelUp.grantsTitle", { level });
  return (
    <section
      aria-label={title}
      data-testid="level-up-grants"
      className="flex flex-col rounded-md bg-raised px-2.5 py-1 text-body"
    >
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3">
        <p className="font-semibold">{title}</p>
        <Button
          variant="ghost"
          size="icon-xs"
          className="size-[1.125rem]"
          aria-label={t("app.levelUp.dismissTitle")}
          title={t("app.levelUp.dismissTitle")}
          onClick={onDismiss}
          data-testid="level-up-grants-dismiss"
        >
          <X aria-hidden weight="bold" />
        </Button>
      </div>
      <GrantRows
        grants={grants}
        gained
        onOpenEditor={onOpenEditor}
        onOpenFreeSpell={onOpenFreeSpell}
      />
    </section>
  );
}
