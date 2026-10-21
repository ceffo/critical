import { parseBuild, type Build } from "@/lib/build";

/** A fresh read of `GET /api/health` (public); null when the API does not answer. */
export async function fetchBuild(): Promise<Build | null> {
  try {
    const res = await fetch("/api/health", { cache: "no-store" });
    if (res.ok) return parseBuild(await res.json());
  } catch {
    // no answer, no badge
  }
  return null;
}

let loaded: Promise<Build | null> | null = null;

/**
 * The build this page loaded with, read once from `GET /api/health` and kept for the page's life
 * (every screen's badge shows the same one; a later deploy is only seen by `fetchBuild`). A failed
 * read is tried again by the next caller.
 */
export function readBuild(): Promise<Build | null> {
  loaded ??= fetchBuild().then((b) => {
    if (!b) loaded = null;
    return b;
  });
  return loaded;
}
