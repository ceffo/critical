import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { hasGrants } from "@/lib/grants";
import { cn } from "@/lib/utils";
import type { LevelGrants } from "@/types/character";
import type { SkillsEditor } from "@/stores/skillsEditor";
import { GrantRows } from "./LevelUpGrants";

/**
 * The "to spend" reminder of the character card (ADR 0037 step 15, user ruling: it stays visible
 * while anything is left, with links to the editors): a tag beside the class while any of the
 * server's left counts is above 0, gone at 0. A tag, not a panel (ADR 0027 step 1: a rare control
 * takes no room from the every-turn ones; the card never scrolls for it, ADR 0022 step 2.8); its
 * popover lists what is left, each with the way into its editor.
 */
export function ToSpendTag({
  toSpend,
  onOpenEditor,
}: {
  toSpend: LevelGrants;
  onOpenEditor: (editor: SkillsEditor) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  if (!hasGrants(toSpend)) return null;
  const title = t("app.levelUp.toSpendTitle");
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        className={cn(
          "inline-flex h-5 shrink-0 items-center rounded-full bg-selected px-2 text-label font-medium text-foreground outline-none transition-colors hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50",
          open && "bg-raised-hover",
        )}
        title={t("app.levelUp.toSpendHint")}
        data-testid="to-spend-tag"
      >
        {title}
      </PopoverTrigger>
      <PopoverContent
        side="right"
        align="start"
        className="flex w-[22rem] max-w-[calc(100vw-2rem)] flex-col gap-2 text-body"
        data-testid="level-up-to-spend"
      >
        <PopoverTitle className="section-label">{title}</PopoverTitle>
        <GrantRows
          grants={toSpend}
          gained={false}
          onOpenEditor={(editor) => {
            setOpen(false);
            onOpenEditor(editor);
          }}
        />
      </PopoverContent>
    </Popover>
  );
}
