import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Power } from "@/types/powers";

const searchPowers = vi.fn();
vi.mock("@/api/powers", async (orig) => ({
  ...(await orig<typeof import("@/api/powers")>()),
  searchPowers: (...a: unknown[]) => searchPowers(...a),
}));

import { NO_POWER_QUERY } from "@/api/powers";
import { isPowerQuerySet, powerQueryKey, usePowerStore } from "./powers";

const power = (id: string) => ({ id }) as Power;
const page = (ids: string[], total = ids.length) => ({
  items: ids.map(power),
  total,
  page: 1,
  page_size: 200,
});

beforeEach(() => {
  searchPowers.mockReset();
  usePowerStore.setState({
    active: false,
    query: NO_POWER_QUERY,
    items: [],
    total: 0,
    pages: 0,
    shownKey: "",
    loading: false,
    error: null,
    selectedId: null,
  });
});

// ADR 0043 step 8: the powers lookup searches the catalogue, selects the first match and does not
// search again for the rows it already shows.
describe("power lookup store", () => {
  it("selects the first row of a search and skips an unchanged query", async () => {
    searchPowers.mockResolvedValue(page(["aura_sight", "telekinesis"]));
    await usePowerStore.getState().search();
    expect(usePowerStore.getState().selectedId).toBe("aura_sight");
    expect(usePowerStore.getState().total).toBe(2);
    await usePowerStore.getState().search();
    expect(searchPowers).toHaveBeenCalledTimes(1);
  });

  it("moves the selection within the rows, stopping at the ends", async () => {
    searchPowers.mockResolvedValue(page(["a", "b"]));
    await usePowerStore.getState().search();
    usePowerStore.getState().move(1);
    usePowerStore.getState().move(1);
    expect(usePowerStore.getState().selectedId).toBe("b");
    usePowerStore.getState().move(-5);
    expect(usePowerStore.getState().selectedId).toBe("a");
  });

  it("knows when the query differs from the default", () => {
    expect(isPowerQuerySet(NO_POWER_QUERY)).toBe(false);
    expect(isPowerQuerySet({ ...NO_POWER_QUERY, kind: "science" })).toBe(true);
    expect(powerQueryKey({ ...NO_POWER_QUERY, q: " aura " })).toBe(
      powerQueryKey({ ...NO_POWER_QUERY, q: "aura" }),
    );
  });
});
