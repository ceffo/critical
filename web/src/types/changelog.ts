/** One release in the changelog: its Features and Fixes are English lists, empty when it has none (ADR 0039 step 4). */
export interface ChangelogVersion {
  version: string;
  features: string[];
  fixes: string[];
}

/** One page of `GET /api/changelog`, newest version first; `nextCursor` is absent on the last page. */
export interface ChangelogPage {
  versions: ChangelogVersion[];
  nextCursor?: string;
}
