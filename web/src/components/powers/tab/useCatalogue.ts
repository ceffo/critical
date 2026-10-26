import { useEffect, useState } from "react";
import { searchPowers } from "@/api/powers";
import type { Power, PowerKind } from "@/types/powers";

/** The catalogue's powers of one kind (the DM's attack modes); read once. */
export function useCatalogue(kind: PowerKind): Power[] {
  const [items, setItems] = useState<Power[]>([]);
  useEffect(() => {
    let live = true;
    searchPowers({ q: "", discipline: "", kind }).then(
      (page) => live && setItems(page.items),
      () => undefined,
    );
    return () => {
      live = false;
    };
  }, [kind]);
  return items;
}
