import App from "@/App";
import { Roster } from "@/components/characters/Roster";
import { CreationRoute } from "@/components/creation/CreationRoute";
import { navigate, usePath } from "@/lib/navigation";
import { CREATE_PATH, ROSTER_PATH } from "@/lib/session";

/**
 * A player with a current character moves between the app, the roster (ADR 0034 step 7) and
 * `/create` without a reload.
 */
export function Playing() {
  const path = usePath();
  if (path === CREATE_PATH) return <CreationRoute withIntro={false} />;
  if (path === ROSTER_PATH) return <Roster onBack={() => navigate("/")} />;
  return <App />;
}
