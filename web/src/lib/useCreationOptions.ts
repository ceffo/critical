import { useEffect, useState } from "react";
import { hasIdentityChoices } from "@/api/characters";
import { useCreationOptionsStore } from "@/stores/creationOptions";
import type { CreationOptions } from "@/types/character";

/**
 * The creation options from the store, read on first use. `failed` after a failed read until
 * `retry`; `choices` also refuses options without the alignment and sex lists (ADR 0032).
 */
export function useCreationOptions(choices = false): {
  options: CreationOptions | null;
  failed: boolean;
  retry: () => void;
} {
  const [state, setState] = useState<{
    options: CreationOptions | null;
    failed: boolean;
  }>(() => {
    // Already read: shown at once, no loading flash.
    const held = useCreationOptionsStore.getState().options;
    return {
      options: held && (!choices || hasIdentityChoices(held)) ? held : null,
      failed: false,
    };
  });
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let live = true;
    const s = useCreationOptionsStore.getState();
    (choices ? s.loadChoices() : s.load()).then(
      (options) => live && setState({ options, failed: false }),
      () => live && setState({ options: null, failed: true }),
    );
    return () => {
      live = false;
    };
  }, [choices, attempt]);
  return {
    ...state,
    retry: () => {
      setState((p) => ({ ...p, failed: false }));
      setAttempt((n) => n + 1);
    },
  };
}
