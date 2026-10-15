import { beforeEach, describe, expect, it, vi } from "vitest";

const request = vi.fn();
vi.mock("./http", () => ({ request: (...a: unknown[]) => request(...a) }));

import { NO_QUERY, searchSpells } from "./spells_db";

const params = () => new URL(request.mock.calls[0][0] as string).searchParams;

beforeEach(() => {
  vi.stubGlobal("window", { location: { origin: "http://localhost" } });
  request.mockReset();
  request.mockResolvedValue({ items: [], total: 0 });
});

describe("searchSpells", () => {
  it("sends the class, levels, school and sphere the lookup chose, empty ones left out", async () => {
    await searchSpells({ ...NO_QUERY, cls: "wizard", levels: [1, 3], school: "abjuration" });
    expect(params().get("class")).toBe("wizard");
    expect(params().get("level")).toBe("1,3");
    expect(params().get("school")).toBe("abjuration");
    expect(params().has("sphere")).toBe(false);
    expect(params().has("q")).toBe(false);
    expect(params().has("match_exact")).toBe(false);
  });

  it("sends a Priest sphere by its English name", async () => {
    await searchSpells({ ...NO_QUERY, cls: "priest", sphere: "Elemental Fire" });
    expect(params().get("class")).toBe("priest");
    expect(params().get("sphere")).toBe("Elemental Fire");
  });

  it("sends the French catalogue keys of the typed text with their tier", async () => {
    await searchSpells({ ...NO_QUERY, q: "mains brûlantes" });
    expect(params().get("q")).toBe("mains brûlantes");
    expect(params().get("match_exact")).toBe("burning_hands");
  });
});
