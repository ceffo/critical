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
import type { FreeSpell, LevelGrants } from "@/types/character";
import type { SkillsEditor } from "@/stores/skillsEditor";
import { GrantRows, type SpendPools } from "./LevelUpGrants";

export type { SpendPools };

/**
 * The "to spend" reminder of the character card (ADR 0037 step 15, user ruling: it stays visible
 * while anything is left, with links to the editors): a tag beside the class while any of the
 * server's left counts is above 0, gone at 0. A tag, not a panel (ADR 0027 step 1: a rare control
 * takes no room from the every-turn ones; the card never scrolls for it, ADR 0022 step 2.8); its
 * popover lists what is left, each with the way into its editor.
 */
export function ToSpendTag({
  toSpend,
  pools,
  onOpenEditor,
  onOpenFreeSpell,
}: {
  toSpend: LevelGrants;
  /** The pools the character has: the entry stays while there is one (ADR 0037 step 15). */
  pools?: SpendPools;
  onOpenEditor: (editor: SkillsEditor) => void;
  /** Opens the spell lookup on a free spell owed (ADR 0030 step 20.8). */
  onOpenFreeSpell: (fs: FreeSpell) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  if (!hasGrants(toSpend) && !Object.values(pools ?? {}).some(Boolean))
    return null;
  const title = t("app.levelUp.toSpendTitle");
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        className={cn(
          "hit-24 inline-flex h-5 shrink-0 items-center rounded-full bg-selected px-2 text-label font-medium text-foreground transition-colors hover:bg-raised-hover",
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
          pools={pools}
          gained={false}
          onOpenEditor={(editor) => {
            setOpen(false);
            onOpenEditor(editor);
          }}
          onOpenFreeSpell={(fs) => {
            setOpen(false);
            onOpenFreeSpell(fs);
          }}
        />
      </PopoverContent>
    </Popover>
  );
}
