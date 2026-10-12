import { useTranslation } from "react-i18next";
import { ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useBuffsStore } from "@/stores/buffs";
import { useTableStore } from "@/stores/table";
import { captionClass } from "./fieldStyles";

/** Round number + Next Round. */
export function RoundControl() {
  const { t } = useTranslation();
  const round = useBuffsStore((s) => s.round);
  const { nextRound, busy } = useTableStore();
  return (
    <div
      className="flex items-center gap-3"
      role="group"
      aria-label={t("round.title")}
    >
      <div className="flex flex-col items-center leading-none">
        <span className={captionClass}>{t("round.title")}</span>
        <span
          className="font-mono text-3xl font-black tabular-nums"
          data-testid="round-value"
        >
          {round}
        </span>
      </div>
      <Button size="lg" className="h-11 px-4 text-base" disabled={busy} onClick={() => void nextRound()}>
        {t("round.next")}
        <ChevronRight />
      </Button>
    </div>
  );
}
