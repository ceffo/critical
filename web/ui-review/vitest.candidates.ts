import base from "../vite.config";

// The new-themes round's contrast check (`just ui-themes`): the app's theme.test.ts over the
// candidates of candidate-themes.ts. Only the candidates' rows matter; the list-pinning tests fail.
export default {
  ...base,
  test: {
    include: ["src/lib/theme.test.ts"],
    setupFiles: ["ui-review/candidates.setup.ts"],
  },
};
