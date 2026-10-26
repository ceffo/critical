import { useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { Brain, Lightning, Sliders } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { useCharacterStore } from "@/stores/character";
import { useTableStore } from "@/stores/table";
import { cn } from "@/lib/utils";
import { fieldClass, groupHeadClass } from "./fieldStyles";
import { RestPopover } from "./RestPopover";

/**
 * The PSP block (ADR 0043 steps 5 and 6, ADR 0047 items 7 and 11), for a character whose class lists
 * psionics: current over max from the derived character (no fetch of its own; a write's answer sets
 * it, an undo's "psp" key reads the character again) and a bar, then Rest (the rest popover) and
 * Adjust (a hand edit: Enter spends, Shift+Enter adds, as the HP change). The value line leaves room
 * on its right for the next round's balance. Nothing for a character with no pool. Every number is
 * the server's.
 */
export function PspPanel() {
  const { t } = useTranslation();
  const pool = useCharacterStore((s) => s.character?.derived?.psp ?? null);
  const { recordPsp, busy } = useTableStore();
  const [adjusting, setAdjusting] = useState(false);
  const [amount, setAmount] = useState("");
  const [source, setSource] = useState("");
  if (!pool) return null;

  const n = Number.parseInt(amount, 10);
  const valid = n > 0;
  const pct = pool.max > 0 ? Math.min(100, (pool.current / pool.max) * 100) : 0;

  async function apply(sign: 1 | -1) {
    if (!valid || busy) return;
    const sent = { amount, source };
    if (await recordPsp(sign * n, source.trim())) {
      // The write's answer lands after a while: a field the player changed meanwhile keeps its text.
      setAmount((a) => (a === sent.amount ? "" : a));
      setSource((s) => (s === sent.source ? "" : s));
      setAdjusting(false);
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key !== "Enter" || e.nativeEvent.isComposing) return;
    e.preventDefault();
    void apply(e.shiftKey ? 1 : -1);
  }

  return (
    <section
      className="flex shrink-0 flex-col gap-2"
      role="group"
      aria-label={t("psp.title")}
      data-testid="psp-panel"
    >
      <h2 className={cn(groupHeadClass, "section-label")}>{t("psp.title")}</h2>
      <div className="flex flex-col gap-1">
        <div className="flex flex-wrap items-baseline justify-between gap-x-2">
          <div
            className="flex items-baseline gap-1 tabular-nums"
            data-testid="psp-value"
          >
            <span className="text-hero">{pool.current}</span>
            <span className="text-body text-muted-foreground">
              {" "}
              / {pool.max}
            </span>
          </div>
        </div>
        <div
          className="flex h-2 w-full overflow-hidden rounded bg-raised"
          role="meter"
          aria-label={t("psp.title")}
          aria-valuemin={0}
          aria-valuemax={pool.max}
          aria-valuenow={pool.current}
        >
          <div
            className="h-full bg-magic transition-all duration-300"
            style={{ width: `${pct}%` }}
          />
        </div>
      </div>
      <div className="grid grid-cols-2 items-center gap-x-1.5">
        <RestPopover />
        <Popover open={adjusting} onOpenChange={setAdjusting}>
          <PopoverTrigger
            render={<Button className="min-w-0 gap-1 border-dashed px-2" />}
            data-testid="psp-adjust"
            title={t("psp.adjustHint")}
          >
            <Sliders aria-hidden />
            {t("psp.adjust")}
          </PopoverTrigger>
          <PopoverContent
            side="right"
            align="start"
            className="flex w-72 max-w-[calc(100vw-2rem)] flex-col gap-2 p-4"
            aria-label={t("psp.adjust")}
            data-testid="psp-adjust-popover"
          >
            <PopoverTitle className="section-label">
              {t("psp.adjust")}
            </PopoverTitle>
            <input
              aria-label={t("psp.amount")}
              className={`${fieldClass} tabular-nums`}
              type="number"
              min={1}
              inputMode="numeric"
              placeholder={t("psp.amountPlaceholder")}
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              onKeyDown={onKeyDown}
            />
            <input
              className={cn(fieldClass, "h-6 text-label")}
              type="text"
              aria-label={t("psp.source")}
              title={t("psp.keysHint")}
              placeholder={t("psp.sourcePlaceholder")}
              value={source}
              onChange={(e) => setSource(e.target.value)}
              onKeyDown={onKeyDown}
            />
            <div className="grid grid-cols-2 gap-1.5">
              <Button
                className="min-w-0 gap-1 px-2 text-danger"
                disabled={!valid || busy}
                title={t("psp.spendHint")}
                onClick={() => void apply(-1)}
              >
                <Lightning aria-hidden />
                {t("psp.spend")}
              </Button>
              <Button
                className="min-w-0 gap-1 px-2 text-success"
                disabled={!valid || busy}
                title={t("psp.recoverHint")}
                onClick={() => void apply(1)}
              >
                <Brain aria-hidden />
                {t("psp.recover")}
              </Button>
            </div>
          </PopoverContent>
        </Popover>
      </div>
    </section>
  );
}
