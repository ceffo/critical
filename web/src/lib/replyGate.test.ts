import { describe, expect, it } from "vitest";
import { replyGate } from "./replyGate";

describe("replyGate", () => {
  it("keeps only the newest request's reply", () => {
    const g = replyGate();
    const first = g.open();
    expect(first()).toBe(true);
    const second = g.open();
    expect(first()).toBe(false);
    expect(second()).toBe(true);
  });
  it("drops every pending reply once closed", () => {
    const g = replyGate();
    const pending = g.open();
    g.close();
    expect(pending()).toBe(false);
    expect(g.open()()).toBe(true);
  });
});
