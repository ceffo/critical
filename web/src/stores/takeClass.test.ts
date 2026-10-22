import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/api/http";

const takeNewClass = vi.hoisted(() => vi.fn());
vi.mock("@/api/character", () => ({
  fetchCharacter: vi.fn(() => Promise.resolve(null)),
  patchCharacterDetails: vi.fn(),
  rollLevelUp: vi.fn(),
  takeNewClass,
}));

import { useCharacterStore } from "./character";
import { useTableStore } from "./table";

describe("takeNewClass", () => {
  beforeEach(() => {
    takeNewClass.mockReset();
    useTableStore.setState({ error: null });
  });

  it("returns the refusal for the dialog to word and raises no global error", async () => {
    const refusal = new ApiError("too low", 409, undefined, "level_too_low");
    takeNewClass.mockRejectedValue(refusal);
    const out = await useCharacterStore.getState().takeNewClass("mage");
    expect(out).toBe(refusal);
    expect(useTableStore.getState().error).toBeNull();
  });

  it("returns null when the class was taken", async () => {
    takeNewClass.mockResolvedValue({});
    expect(await useCharacterStore.getState().takeNewClass("mage")).toBeNull();
    expect(takeNewClass).toHaveBeenCalledWith("mage");
  });

  it("sends a server failure to the action module's error notice", async () => {
    takeNewClass.mockRejectedValue(new ApiError("boom", 500));
    expect(await useCharacterStore.getState().takeNewClass("mage")).toBeNull();
    expect(useTableStore.getState().error).toBe("boom");
  });
});
