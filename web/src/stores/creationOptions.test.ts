import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useCreationOptionsStore } from "./creationOptions";

const complete = {
  classes: [{ class: "bard", races: ["human"], alignments: ["true_neutral"] }],
  sexes: ["male", "female"],
};

function answer(body: unknown) {
  const fetch = vi.fn(
    async () => new Response(JSON.stringify(body), { status: 200 }),
  );
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

beforeEach(() => useCreationOptionsStore.setState({ options: null }));
afterEach(() => vi.unstubAllGlobals());

describe("creation options store", () => {
  it("reads the options once per page load, however many callers ask", async () => {
    const fetch = answer(complete);
    const { load, loadChoices } = useCreationOptionsStore.getState();
    // Two callers at once share the read, a later one is served from the store.
    await Promise.all([load(), load()]);
    await load();
    await loadChoices();
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("does not keep a failed read: the next call asks again", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("{}", { status: 500 })),
    );
    await expect(useCreationOptionsStore.getState().load()).rejects.toThrow();
    const fetch = answer(complete);
    await expect(
      useCreationOptionsStore.getState().load(),
    ).resolves.toMatchObject({ sexes: ["male", "female"] });
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("rejects the options of a release that predates the alignment and sex lists, and asks again next time", async () => {
    answer({ classes: [{ class: "bard", races: ["human"] }] });
    await expect(
      useCreationOptionsStore.getState().loadChoices(),
    ).rejects.toThrow();
    const fetch = answer(complete);
    await expect(
      useCreationOptionsStore.getState().loadChoices(),
    ).resolves.toMatchObject({ sexes: ["male", "female"] });
    expect(fetch).toHaveBeenCalledTimes(1);
  });
});
