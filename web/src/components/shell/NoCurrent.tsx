import { useEffect, useState } from "react";
import { fetchCharacters } from "@/api/characters";
import { Roster } from "@/components/characters/Roster";
import { CreationRoute } from "@/components/creation/CreationRoute";
import { usePath } from "@/lib/navigation";
import { CREATE_PATH } from "@/lib/session";

/**
 * A user with no current character (ADR 0029 step 7.2, ADR 0034 steps 1 and 7). With no character at
 * all, the intro: a new player's first login, or every character retired. With others (the current one
 * retired elsewhere), the roster without Retour, there being no game to go back to; its Créer opens the
 * creation, whose Quitter returns to it. A failed read of the list shows the intro, which needs none.
 */
export function NoCurrent() {
  const path = usePath();
  const [others, setOthers] = useState<boolean | null>(null);
  useEffect(() => {
    let live = true;
    fetchCharacters().then(
      (l) => live && setOthers(l.characters.length > 0),
      () => live && setOthers(false),
    );
    return () => {
      live = false;
    };
  }, []);
  if (others === null) return null;
  if (!others) return <CreationRoute withIntro />;
  return path === CREATE_PATH ? (
    <CreationRoute withIntro={false} />
  ) : (
    <Roster />
  );
}
