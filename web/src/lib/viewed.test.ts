import { describe, expect, it } from "vitest";
import { mayLeaveReadOnly, viewedIdOf, viewPath } from "./viewed";

describe("viewedIdOf", () => {
  it.each([
    ["/view/12", 12],
    ["/view/12/", 12],
    ["/view/1", 1],
    ["/", null],
    ["/characters", null],
    ["/view/", null],
    ["/view/0", null],
    ["/view/-3", null],
    ["/view/abc", null],
    ["/view/12/extra", null],
    ["/viewer/12", null],
  ])("%s names %s", (path, want) => {
    expect(viewedIdOf(path)).toBe(want);
  });

  it("reads back what viewPath builds", () => {
    expect(viewedIdOf(viewPath(345))).toBe(345);
  });
});

describe("mayLeaveReadOnly", () => {
  it.each([
    ["/api/hp", undefined, true],
    ["/api/hp", "GET", true],
    ["/api/hp", "HEAD", true],
    ["/api/hp/event", "POST", false],
    ["/api/items/3", "PUT", false],
    ["/api/items/3", "PATCH", false],
    ["/api/items/3", "DELETE", false],
    ["/api/characters/current", "PUT", false],
    ["/api/roll/dice", "POST", false],
    ["/api/auth/logout", "POST", true],
  ])("%s %s -> %s", (url, method, want) => {
    expect(mayLeaveReadOnly(url, method)).toBe(want);
  });
});
