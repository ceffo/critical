import type { ComponentType } from "react";
import { useTranslation } from "react-i18next";
import { Backpack, BookOpenText, HandFist, type IconProps } from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import { WORKSPACES, type Workspace } from "@/lib/workspaces";
import { SettingsPopover } from "./SettingsPopover";

const ICONS: Record<Workspace, ComponentType<IconProps>> = {
  act: HandFist,
  inventory: Backpack,
  spells: BookOpenText,
};

/** Left rail: switches the workspace; the gear at the bottom opens the settings. */
export function Rail({
  active,
  onSelect,
}: {
  active: Workspace;
  onSelect: (w: Workspace) => void;
}) {
  const { t } = useTranslation();
  return (
    <nav
      aria-label={t("shell.workspaces")}
      className="flex min-h-0 flex-col items-stretch gap-1 py-1"
    >
      {WORKSPACES.map((w) => {
        const Icon = ICONS[w];
        const on = active === w;
        return (
          <button
            key={w}
            type="button"
            aria-current={on ? "page" : undefined}
            className={cn(
              "relative flex flex-col items-center gap-1 rounded-md px-0.5 py-2 text-xs transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50 [&_svg]:size-6",
              on
                ? "bg-amb-900 text-amb-200 before:absolute before:inset-y-2 before:left-0 before:w-0.5 before:rounded-full before:bg-tint"
                : "text-muted-foreground hover:bg-neu-900 hover:text-foreground",
            )}
            onClick={() => onSelect(w)}
          >
            <Icon weight={on ? "fill" : "regular"} aria-hidden />
            <span className="max-w-full truncate">{t(`shell.workspace.${w}`)}</span>
          </button>
        );
      })}
      <div className="mt-auto flex justify-center pb-1">
        <SettingsPopover />
      </div>
    </nav>
  );
}
