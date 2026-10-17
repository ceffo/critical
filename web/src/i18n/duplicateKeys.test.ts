import { describe, expect, it } from "vitest";

// JSON.parse keeps the last of two equal keys without a word, so a duplicate key hides a text.
// This scans the raw text of every locale file.

/** Keys that appear twice in the same object of a JSON text, as dotted paths. */
function duplicateKeys(text: string): string[] {
  const found: string[] = [];
  const stack: Array<{ name: string; keys: Set<string> } | null> = [];
  const path: string[] = [];
  const token = /"((?:[^"\\]|\\.)*)"(\s*:)?|[{}[\]]/g;
  let pendingName = "";
  for (const m of text.matchAll(token)) {
    const t = m[0];
    if (t === "{") {
      stack.push({ name: pendingName, keys: new Set() });
      path.push(pendingName);
      pendingName = "";
    } else if (t === "[") {
      stack.push(null);
      path.push(pendingName);
      pendingName = "";
    } else if (t === "}" || t === "]") {
      stack.pop();
      path.pop();
    } else if (m[2]) {
      const top = stack[stack.length - 1];
      if (top) {
        if (top.keys.has(m[1])) {
          found.push([...path, m[1]].filter(Boolean).join("."));
        }
        top.keys.add(m[1]);
      }
      pendingName = m[1];
    }
  }
  return found;
}

describe("duplicateKeys", () => {
  it("finds a key repeated in one object, not the same key in two objects", () => {
    expect(duplicateKeys('{"a": {"x": 1, "y": 2, "x": 3}, "b": {"x": 1}}')).toEqual(["a.x"]);
    expect(duplicateKeys('{"a": {"x": 1}, "b": {"x": 2}, "c": [{"x": 1}, {"x": 2}]}')).toEqual([]);
  });
});

describe("locale files", () => {
  const files = import.meta.glob("./locales/**/*.json", {
    query: "?raw",
    import: "default",
    eager: true,
  }) as Record<string, string>;

  it("are found", () => {
    expect(Object.keys(files).length).toBeGreaterThanOrEqual(2);
  });

  it.each(Object.entries(files))("%s has no duplicate key", (file, text) => {
    expect(duplicateKeys(text).map((k) => `${file}: ${k}`)).toEqual([]);
  });
});
