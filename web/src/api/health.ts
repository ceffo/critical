import { parseBuild, type Build } from "@/lib/build";

/** The running build, read once at start-up from `GET /api/health` (public); null when the API does not answer. */
export async function readBuild(): Promise<Build | null> {
  try {
    const res = await fetch("/api/health");
    if (res.ok) return parseBuild(await res.json());
  } catch {
    // no badge without an answer
  }
  return null;
}
