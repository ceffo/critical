import { useEffect, useState } from "react";
import type { ApiError } from "@/api/http";
import {
  DisciplineChips,
  KnownPowersList,
} from "@/components/powers/KnownPowersEditor";
import { usePickEdits } from "@/components/powers/usePickEdits";
import { PowerLookup } from "@/components/actions/PowerLookup";
import { usePowerStore } from "@/stores/powers";
import { useCharacterStore } from "@/stores/character";
import { useTableStore } from "@/stores/table";

/**
 * The Pouvoirs tab of Sorts (ADR 0047 step 12): the powers catalogue to browse and add from, beside
 * the character's known powers to relearn or remove. Every change saves the whole list
 * (`PUT /api/psionics/picks`); a refusal is the save's 409 and the list stays as it was. Nothing
 * for a character with no known powers block.
 */
export function PowersBrowser({ tabs }: { tabs: React.ReactNode }) {
  const known = useCharacterStore((s) => s.character?.derived?.knownPowers);
  const save = useCharacterStore((s) => s.saveKnownPicks);
  const busy = useTableStore((s) => s.busy);
  const options = usePowerStore((s) => s.options);
  const items = usePowerStore((s) => s.items);
  const loadOptions = usePowerStore((s) => s.loadOptions);
  const [refusal, setRefusal] = useState<ApiError | null>(null);
  const picks = known?.picks ?? [];

  useEffect(() => {
    void loadOptions();
  }, [loadOptions]);

  const onChange = async (next: typeof picks) => {
    if (busy) return;
    setRefusal(null);
    setRefusal(await save(next));
  };
  const { add, isPicked } = usePickEdits(picks, busy, onChange);
  if (!known) return null;
  return (
    <div className="flex min-h-0 flex-1" data-testid="known-powers">
      <PowerLookup
        tabs={tabs}
        known={{
          has: (id) => isPicked("power", id),
          add: (id) => add({ kind: "power", id }),
          busy,
        }}
        aside={
          <KnownPowersList
            known={known}
            busy={busy}
            refusal={refusal}
            options={options}
            seen={items}
            disciplineChips={
              <DisciplineChips
                options={options}
                known={known}
                busy={busy}
                onChange={onChange}
              />
            }
            onChange={onChange}
          />
        }
      />
    </div>
  );
}
