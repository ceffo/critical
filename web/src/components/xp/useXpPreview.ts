import { useEffect, useState } from "react";
import { fetchXpPreview } from "@/api/xp";
import type { XpAmount } from "@/types/xp";

/**
 * The server's preview of what an entry of `base` XP adds (ADR 0015 step 1), refetched shortly
 * after the inputs change; null while there is no valid amount. Only the newest answer is kept.
 */
export function useXpPreview(base: number | null, bonus: boolean): XpAmount | null {
  const [answer, setAnswer] = useState<{ bonus: boolean; amount: XpAmount } | null>(null);
  useEffect(() => {
    if (base === null || base < 1) return;
    let stale = false;
    const timer = setTimeout(() => {
      fetchXpPreview(base, bonus)
        .then((amount) => {
          if (!stale) setAnswer({ bonus, amount });
        })
        .catch(() => {
          if (!stale) setAnswer(null);
        });
    }, 120);
    return () => {
      stale = true;
      clearTimeout(timer);
    };
  }, [base, bonus]);
  // An answer for other inputs is never shown.
  return answer && answer.amount.base === base && answer.bonus === bonus ? answer.amount : null;
}
