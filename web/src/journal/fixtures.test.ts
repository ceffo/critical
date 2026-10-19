// The fixtures and the registry cover the same kinds. The Go side proves each fixture is what the
// owning service logs, its roll flag is the server's, and every Go log kind has a fixture (the
// `Test…JournalFixture…` tests in api/internal/domain/actionlog); this proves the web has a view
// and a kind test for every fixture.
import { describe, expect, it } from "vitest";
import { KIND_VIEWS } from "./registry";

const fixtures = import.meta.glob("./fixtures/*.json");
const kindTests = import.meta.glob("./kinds/*.test.ts");

const kindOf = (path: string, ext: string) =>
  path.replace(/^.*\//, "").replace(ext, "");

describe("journal fixtures", () => {
  it("has a fixture for every kind with a view, and a view for every fixture", () => {
    const fixtureKinds = Object.keys(fixtures).map((p) => kindOf(p, ".json"));
    expect(fixtureKinds.sort()).toEqual(Object.keys(KIND_VIEWS).sort());
  });

  it("has a kind test for every fixture", () => {
    const tested = Object.keys(kindTests).map((p) => kindOf(p, ".test.ts"));
    expect(tested.sort()).toEqual(
      Object.keys(fixtures)
        .map((p) => kindOf(p, ".json"))
        .sort(),
    );
  });
});
