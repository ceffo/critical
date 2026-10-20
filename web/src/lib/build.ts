/** The environment `/api/health` reports (ADR 0024 step 1.4); anything else is read as unknown. */
export type BuildEnv = "dev" | "staging" | "production" | "unknown";

/** What `GET /api/health` says about the running build. */
export interface Build {
  version: string;
  env: BuildEnv;
}

/** The build the server reports, from the health answer; a missing or odd field reads as unknown. */
export function parseBuild(body: unknown): Build {
  const b = (body ?? {}) as { version?: unknown; env?: unknown };
  const env: BuildEnv =
    b.env === "dev" || b.env === "staging" || b.env === "production"
      ? b.env
      : "unknown";
  return {
    version: typeof b.version === "string" ? b.version : "unknown",
    env,
  };
}

/** The badge a build shows: its version and the pill before it (production and unknown: none), or null when there is nothing to show. */
export function badgeOf(
  build: Build,
): { version: string; pill: "dev" | "staging" | null } | null {
  const pill =
    build.env === "dev" || build.env === "staging" ? build.env : null;
  if (build.version === "unknown" && !pill) return null;
  return { version: build.version, pill };
}
