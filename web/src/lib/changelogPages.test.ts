import { describe, expect, it, vi } from "vitest";
import { createChangelogPages } from "@/lib/changelogPages";
import type { ChangelogPage } from "@/types/changelog";

const v = (version: string) => ({ version, features: [], fixes: [] });

describe("changelog pages", () => {
  it("loads the next page with the cursor, appends it, and ends without a nextCursor", async () => {
    const fetchPage = vi
      .fn<(c?: string) => Promise<ChangelogPage>>()
      .mockResolvedValueOnce({ versions: [v("3.0.0")], nextCursor: "c1" })
      .mockResolvedValueOnce({ versions: [v("2.0.0")] });
    const p = createChangelogPages(fetchPage);
    await p.loadMore();
    await p.loadMore();
    await p.loadMore(); // complete: nothing more is asked
    expect(fetchPage.mock.calls).toEqual([[undefined], ["c1"]]);
    expect(p.getSnapshot().versions.map((x) => x.version)).toEqual([
      "3.0.0",
      "2.0.0",
    ]);
    expect(p.getSnapshot().done).toBe(true);
  });

  it("asks once while a request is in flight", async () => {
    let resolve!: (p: ChangelogPage) => void;
    const fetchPage = vi.fn(
      () => new Promise<ChangelogPage>((r) => (resolve = r)),
    );
    const p = createChangelogPages(fetchPage);
    const first = p.loadMore();
    void p.loadMore();
    void p.loadMore();
    expect(fetchPage).toHaveBeenCalledTimes(1);
    expect(p.getSnapshot().loading).toBe(true);
    resolve({ versions: [v("1.0.0")], nextCursor: "x" });
    await first;
    expect(p.getSnapshot().loading).toBe(false);
  });

  it("keeps what is loaded on a failure and retries the same page", async () => {
    const fetchPage = vi
      .fn<(c?: string) => Promise<ChangelogPage>>()
      .mockResolvedValueOnce({ versions: [v("2.0.0")], nextCursor: "c1" })
      .mockRejectedValueOnce(new Error("boom"))
      .mockResolvedValueOnce({ versions: [v("1.0.0")] });
    const p = createChangelogPages(fetchPage);
    await p.loadMore();
    await p.loadMore();
    expect(p.getSnapshot()).toMatchObject({ failed: true, loading: false });
    expect(p.getSnapshot().versions).toHaveLength(1);
    await p.loadMore();
    expect(fetchPage.mock.calls[2]).toEqual(["c1"]);
    expect(p.getSnapshot()).toMatchObject({ failed: false, done: true });
    expect(p.getSnapshot().versions).toHaveLength(2);
  });

  it("an empty answer is a complete, empty list, not an error", async () => {
    const p = createChangelogPages(() => Promise.resolve({ versions: [] }));
    await p.loadMore();
    expect(p.getSnapshot()).toMatchObject({
      versions: [],
      done: true,
      failed: false,
    });
  });
});
