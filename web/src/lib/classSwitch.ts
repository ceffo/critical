import { useEffect, useState } from "react";
import { fetchSwitchTargets } from "@/api/character";
import type { Character, CreationOptions } from "@/types/character";

// One answer per character object: the three places that show the link share a single round trip
// per entry, and a changed character (a new object in the store) asks again.
const answers = new WeakMap<Character, Promise<number[]>>();

function ask(character: Character): Promise<number[]> {
  let p = answers.get(character);
  if (!p) {
    p = Promise.all(
      character.classes.map((_, i) =>
        fetchSwitchTargets(i).then(
          (r) => r.targets.length > 0,
          () => false,
        ),
      ),
    ).then((has) => has.flatMap((h, i) => (h ? [i] : [])));
    answers.set(character, p);
  }
  return p;
}

/**
 * The entries of the character with at least one class to switch to (ADR 0050 step 15.8): the
 * server's own list (`GET /api/character/class-entry/targets`), so the web holds no copy of the
 * switch's rules. Empty until the answer is in.
 */
export function useSwitchableEntries(character: Character): number[] {
  const [got, setGot] = useState<{ of: Character; entries: number[] } | null>(
    null,
  );
  useEffect(() => {
    let live = true;
    ask(character).then(
      (entries) => live && setGot({ of: character, entries }),
    );
    return () => {
      live = false;
    };
  }, [character]);
  return got?.of === character ? got.entries : [];
}

/** The school code of a class (a specialist's, ADR 0050 step 1), "" for a mage or any other class. */
export function schoolOf(options: CreationOptions | null, cls: string): string {
  return options?.classes.find((c) => c.class === cls)?.schools?.[0] ?? "";
}
