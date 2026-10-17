import { beforeEach, describe, expect, it, vi } from "vitest";

const request = vi.fn();
vi.mock("./http", () => ({ request: (...a: unknown[]) => request(...a) }));

import { NO_QUERY, getSpell, searchSpells } from "./spells_db";

const params = () => new URL(request.mock.calls[0][0] as string).searchParams;

beforeEach(() => {
  vi.stubGlobal("window", { location: { origin: "http://localhost" } });
  request.mockReset();
  request.mockResolvedValue({ items: [], total: 0 });
});

describe("searchSpells", () => {
  it("sends the class, levels, school and sphere the lookup chose, empty ones left out", async () => {
    await searchSpells({
      ...NO_QUERY,
      cls: "wizard",
      levels: [1, 3],
      school: "abjuration",
    });
    expect(params().get("class")).toBe("wizard");
    expect(params().get("level")).toBe("1,3");
    expect(params().get("school")).toBe("abjuration");
    expect(params().has("sphere")).toBe(false);
    expect(params().has("q")).toBe(false);
  });

  it("sends a Priest sphere by its English name", async () => {
    await searchSpells({
      ...NO_QUERY,
      cls: "priest",
      sphere: "Elemental Fire",
    });
    expect(params().get("class")).toBe("priest");
    expect(params().get("sphere")).toBe("Elemental Fire");
  });

  it("sends the typed text as is: the server matches French names too", async () => {
    await searchSpells({ ...NO_QUERY, q: " mains brûlantes " });
    expect(params().get("q")).toBe("mains brûlantes");
    expect([...params().keys()].some((k) => k.startsWith("match_"))).toBe(
      false,
    );
  });

  it("sends the caster level the receive rolls at, none when not given", async () => {
    await searchSpells({ ...NO_QUERY, q: "meteor" }, 1, 20, 18);
    expect(params().get("caster_level")).toBe("18");
    request.mockClear();
    await searchSpells({ ...NO_QUERY, q: "meteor" }, 1, 20);
    expect(params().has("caster_level")).toBe(false);
  });
});

describe("getSpell", () => {
  it("loads one spell by its id, not by a name search", async () => {
    request.mockResolvedValue({ id: "fire_shield" });
    expect(await getSpell("fire_shield")).toEqual({ id: "fire_shield" });
    expect(request).toHaveBeenCalledExactlyOnceWith("/api/spells/fire_shield");
  });
});
