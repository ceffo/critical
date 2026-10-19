import App from "@/App";
import { CreationRoute } from "@/components/creation/CreationRoute";
import { usePath } from "@/lib/navigation";
import { CREATE_PATH } from "@/lib/session";

/** A player with a current character moves between the app and `/create` without a reload. */
export function Playing() {
  const path = usePath();
  return path === CREATE_PATH ? <CreationRoute hasCurrent /> : <App />;
}
