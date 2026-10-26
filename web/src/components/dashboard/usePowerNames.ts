import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { getPower } from "@/api/powers";
import { localized } from "@/i18n/localized";
import type { LocalizedText } from "@/types/localized";

/** Catalogue names already read, by power id: a name does not change while the page is open. */
const known = new Map<string, LocalizedText>();

/**
 * The catalogue names of power ids in the player's language, read once each. Until a name is read
 * (or when the catalogue has none) the caller's own fallback shows.
 */
export function usePowerNames(
  ids: string[],
): (id: string) => string | undefined {
  const { i18n } = useTranslation();
  const [, bump] = useState(0);
  const key = [...new Set(ids)].sort().join(",");
  useEffect(() => {
    let live = true;
    for (const id of key ? key.split(",") : []) {
      if (known.has(id)) continue;
      getPower(id).then(
        (p) => {
          known.set(id, p.name);
          if (live) bump((n) => n + 1);
        },
        () => undefined,
      );
    }
    return () => {
      live = false;
    };
  }, [key]);
  return (id) => {
    const name = known.get(id);
    return name ? localized(i18n.language, name) : undefined;
  };
}
