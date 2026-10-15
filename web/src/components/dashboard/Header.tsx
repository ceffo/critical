import { BuffStrip } from "./BuffStrip";
import { RoundControl } from "./RoundControl";

/** Header region, full width above the rail and columns: the round and the active buffs. */
export function Header() {
  return (
    <header className="flex min-w-0 shrink-0 items-center gap-6 bg-card px-4 py-2">
      <RoundControl />
      <div className="min-w-0 flex-1">
        <BuffStrip />
      </div>
    </header>
  );
}
