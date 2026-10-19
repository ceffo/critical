import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchCreationChoices } from "./characters";

afterEach(() => vi.unstubAllGlobals());

function answer(body: unknown) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(JSON.stringify(body), { status: 200 })),
  );
}

describe("fetchCreationChoices", () => {
  it("rejects the options of a release that predates the alignment and sex lists", async () => {
    answer({ classes: [{ class: "bard", races: ["human"] }] });
    await expect(fetchCreationChoices()).rejects.toThrow();
  });

  it("accepts options carrying them", async () => {
    answer({
      classes: [
        { class: "bard", races: ["human"], alignments: ["true_neutral"] },
      ],
      sexes: ["male", "female"],
    });
    await expect(fetchCreationChoices()).resolves.toMatchObject({
      sexes: ["male", "female"],
    });
  });
});
