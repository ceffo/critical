# ADR 0024: Deployment on fly.io: environments, release flow, image and data

**Status**: proposed, 2026-10-16. Step 8 and the defaults still listed as open under "Proposed
defaults awaiting the user" are proposals: each one becomes accepted when the user confirms it or
picks another option, and the answer is recorded here with its date. Amended 2026-10-16 by
[ADR 0025](0025-postgresql-storage.md) (see "Amendment (PostgreSQL on one fly cluster, 2026-10-16)"):
step 5 is answered (option (b), PostgreSQL first), steps 2, 4.2, 5, 6.1, 7 and 9 are rewritten for
volume-less apps on one fly PostgreSQL cluster, and the region, production data and PostgreSQL
defaults are answered. Amended again 2026-10-16 (see "Amendment (shared cluster tournesol-db,
2026-10-16)"): the cluster is the user's existing `tournesol-db`, shared, instead of a new
`critical-db`. Amended again 2026-10-16 (see "Amendment (restore point: a copy database,
2026-10-16)"): the pre-promotion restore point is a copy database on the same cluster, and production's
no-public-IP check has no bypass. Amended again 2026-10-17, **proposed, awaiting the user's OK** (see
"Amendment (staging versions from the conventional commits, 2026-10-17)"): every staging release gets a
version computed from the commits, baked into the image and tagged by CI once staging passed (steps 1.4, 3,
4, 10.2 and the "release name" consequence).

## Context

User, 2026-10-12: hosting moves from the Proxmox + Tailscale plan (never built) to fly.io.

User, 2026-10-16: _"I'll be able to host freely on fly.io, i still have 10 vm available so we can have
a staging environment, a 2-machines for prod, a pgsql instance and room to spare. devise the plan for
automatic deployment to staging area when merging to main and promotion to prod on version tagging.
or you can propose a better deploy scheme if you see."_ Same day, as a hard rule: nothing reachable
from the public internet may serve the catalogue (book text) or the app without access control.

Today nothing is deployed. CI runs `just validate`, the generated-code check and the e2e suite on
every PR. The only packaging is docker-compose: the API image plus nginx serving a host-built
`web/dist`, with `seeds/` and `.catalogue/` bind-mounted. Two facts shape the design:

- **The catalogue is private book data** ([ADR 0021](0021-spell-catalogue.md) steps 4 and 7). It
  lives in the private `critical-rulebooks` repo; the deployment fetches it with a read-only deploy
  key. Any database built from it holds catalogue rows, so a database copy is private too.
- **Storage is SQLite** ([ADR 0021](0021-spell-catalogue.md) step 6). On fly a volume belongs to one
  machine, so a SQLite database cannot be shared by two machines. The move to PostgreSQL is a
  separate design (the multi-user model), not started.
  _Amended 2026-10-16: the user's free allowance cannot hold a 1 GB volume per app, so storage moves
  to PostgreSQL before the first deployment ([ADR 0025](0025-postgresql-storage.md)); the multi-user
  model stays a separate design._

The requested shape (staging from main, production from version tags) is kept. The one change to it
is that production runs one machine until storage moves to PostgreSQL (step 5). _Amended 2026-10-16:
no change remains; production runs two machines from the first deployment (step 5)._

## Decision

```mermaid
flowchart LR
    subgraph gh["GitHub (public repo)"]
        PR["Pull request"] -->|"validate, generated, e2e, image check (fixture catalogue)"| CI["CI green"]
        CI -->|merge| MAIN["push to main"]
        MAIN -->|"CI green on the main commit"| VER{"version from the commits since the last v* tag"}
        VER -->|"X.Y.Z"| BUILD["release build: catalogue fetched with the deploy key, version baked in"]
        VER -->|"docs/test/chore only, no-release label"| NONE["no release"]
        TAG["tag vX.Y.Z, made by CI"]
        RUN["Promote run by hand from the tag"]
    end
    subgraph fly["fly.io (org personal, private registry)"]
        IMG[("image tagged with the commit sha")]
        STG["staging app: 1 machine, no volume, auto-stops"]
        PROD["production app: 2 machines, no volume, rolling deploy"]
        subgraph pgc["PostgreSQL cluster tournesol-db, shared (1 node, 1 GB volume, private)"]
            SDB[("critical_staging")]
            PDB[("critical_prod")]
        end
    end
    DUMP[("restore point: copy database critical_prod_pre_tag_time")]
    BUILD -->|push| IMG
    IMG -->|deploy + smoke check| STG
    STG --> SDB
    PROD --> PDB
    STG -->|"smoke check passed"| TAG
    TAG -.-> RUN
    RUN -->|"first: restore point"| DUMP
    RUN -->|"then: promote the same image, no rebuild"| PROD
    PROD -->|"smoke check, then GitHub Release"| REL["release vX.Y.Z"]
```

1. **One image per commit, built once.**
   1. A single container image serves everything: the Go binary serves `/api/*` and the built web
      app. Every non-`/api` path is a static file of the web build, or `index.html` when no such
      file exists (SPA fallback). Unknown `/api/*` paths keep the JSON 404. No nginx in production.
   2. The web directory is a configuration value; when it is unset (as in `just dev`), the server
      serves nothing outside `/api`.
   3. The image carries the campaign `seeds/` and the catalogue artifact, at the paths the server's
      configuration points to. The build takes the catalogue only as an explicit input and fails
      with a clear message when it is missing.
   4. `/api/health` reports the build identity: the commit the image was built from and the release
      version it was built as (`X.Y.Z`, step 3.1), both baked in at build time, each `unknown` when not
      set (as in `just dev`), next to `status`. _Amended 2026-10-17 (staging versions); was: the commit
      only._
   5. A main commit is built once; every later deployment of that commit (staging, production,
      rollback) runs that image, identified by its digest.

   Tests: `TestWeb_Step1_1_` (router: files, SPA fallback, JSON 404 under `/api`, no escape from the
   web dir, a dir without `index.html` refused), `TestWeb_Step1_2_NoWebDirServesNothingOutsideAPI`,
   `TestHealth_Step1_4_ReportsTheBuildCommit`, `TestHealth_Step1_4_ReportsTheBuildVersion`,
   `TestHealth_Step1_4_UnknownWithoutABuildIdentity`. Step 1.3 is checked by the CI `image` job
   (`scripts/image-smoke.sh`); the image and its inputs are described in `docs/backend.md`,
   "Production image".

2. **Environments.** _Amended 2026-10-16 ([ADR 0025](0025-postgresql-storage.md)); was: each app
   has its own volume holding its own SQLite database, and production keeps one machine._ Two fly
   apps in one fly org: **staging** and **production**. Neither app has a volume: their data lives in
   one fly legacy PostgreSQL cluster, the user's existing `tournesol-db`, which also holds the
   databases of the user's other app. _Amended 2026-10-16 (shared cluster); was: a new cluster
   `critical-db`, created for this app alone._ They share the image and the cluster, nothing else.
   1. Staging is disposable: its data may be wiped at any time. It runs one machine, which stops
      when idle and starts on the next request.
   2. Production runs two machines, always running (no auto-stop), updated by a rolling deploy.
   3. The cluster: one node, the smallest shared VM, one 1 GB volume, region `yyz`, no public IP.
      It holds two databases for this app, `critical_staging` and `critical_prod`, each owned by its
      own login user; a critical user can connect to its own database only, neither to the other's
      nor to any other database of the cluster. The other databases keep CONNECT for the login users
      they have when the provisioning runs and lose it for PUBLIC.
   4. Each app's DSN (its own user, its own database, the cluster's private address) is the fly
      secret `CRITICAL_DSN`; it is never in the fly config, the repo or a log.
   5. Nothing is paid without the user: if fly refuses a legacy cluster, or asks for payment or a
      card, the provisioning stops and reports.

3. **Staging on main.** On a push to main, once the CI jobs for that commit are green:
   1. the release computes the **version** of the commit first, from the conventional-commit titles of
      all main commits since the last `v*` version (one whose run was replaced in the queue, or that made
      no release, included); the highest level wins: `feat` a minor; `fix`, `refactor`, `perf`, any other
      type and a title not in the conventional format a patch; `docs`, `test`, `chore` nothing. A breaking
      change (`!`, `BREAKING CHANGE`) is a minor, with a notice that a major may be due: a major is made by
      hand. When no commit of the range has a level outside the PRs labelled `no-release`, there is **no
      release**: nothing is built, deployed or tagged, and the run stays green with a notice saying why;
   2. otherwise the release build fetches the catalogue with the read-only deploy key, builds the image
      of step 1 with that version baked in (step 1.4) and pushes it to the fly registry, tagged with the
      commit sha and the version;
   3. that image is deployed to staging;
   4. a smoke check verifies that `/api/health` answers 200 and reports that commit and that version. A
      failed smoke check fails the run;
   5. only then CI creates the annotated tag `vX.Y.Z` on the commit, and the staging deployment record
      carries the version. A failed staging run takes no version: the next release computes it again,
      so that number may go to a different image, which, unlike the failed one, gets the tag.

   _Amended 2026-10-17 (staging versions); was: the image is pushed tagged with the commit sha only, and
   a version exists only when a human pushes a tag._ The release, from the version to the tag, is one
   serialized run: a newer main commit replaces a queued older one, a started one is never cancelled, and
   two runs never compute a version at once. A manual run with no version releases main's head even when
   the rules make no release (at least a patch); with a version, it redeploys that version's image.

4. **Production from a version tag.** Running the promotion from a version tag `vMAJOR.MINOR.PATCH`
   promotes that commit to production. _Amended 2026-10-17 (staging versions); was: pushing the tag
   promotes. CI makes the tags (step 3.5) and the promotion runs only by hand, from the tag: no tag push
   starts it, until promotion on demand exists._
   1. the promotion refuses a tag whose commit is not on main, or whose image did not pass the
      staging smoke check of step 3;
   2. it takes the restore point of step 9.2, a verified copy of `critical_prod`, and waits for it
      to complete; no restore point, no deploy. _Amended 2026-10-16 ([ADR 0025](0025-postgresql-storage.md));
      was: an on-demand snapshot of the production volume. Amended again 2026-10-16 (restore point); was:
      a `pg_dump` kept where step 9.2 would say._ It is private data (step 7);
   3. it deploys **the same image** (same digest as staging, no rebuild) to production;
   4. it runs the smoke check of step 3.3 against production;
   5. it creates a GitHub Release for the tag.

   Rollback is running the promotion again for an older tag (step 6 says when that is not enough).
   A promotion whose image is older than the production schema (`critical_prod` records a migration
   the tag's commit does not hold) is refused before the restore point is written: that rollback is
   the runbook's restore of step 9.3, then the promotion of the older tag. Production deployments never
   run concurrently and are never cancelled midway.

5. **Storage: PostgreSQL first, two production machines** (_accepted 2026-10-16, option (b); was
   the proposed default (a), SQLite on a volume with one machine per environment_). User,
   2026-10-16: _"can't request 1gb"_; _"if we need a volume, we can't go with fly.io. unless it's
   just a legacy postgres db"_. Options as proposed:
   - (a) ship on SQLite now, one machine per environment, PostgreSQL with the storage move:
     **not taken** (it needs a 1 GB volume per app);
   - **(b) PostgreSQL first: taken.** No deployment until the storage port of
     [ADR 0025](0025-postgresql-storage.md) is done;
   - (c) LiteFS replication of SQLite: rejected (pre-1.0, its cloud backup service is gone, and
     write forwarding adds moving parts to a single-user tool).

   1. The apps are stateless: no volume, no file the app writes outside the database.
   2. Production runs two machines from the first deployment and is deployed with fly's rolling
      strategy, one machine at a time, so a deploy has no planned downtime.
   3. Staging runs one machine with auto-stop and auto-start.

   Check: the fly config test (no `[[mounts]]`, no DSN in `[env]`, two production machines with
   the rolling strategy and no auto-stop, staging auto-stop).

6. **Migrations run at boot**, as today, before the server listens.
   1. An older binary does not undo a newer schema. Rolling back across a migration means
      redeploying the older tag **and** restoring `critical_prod` from the pre-promotion dump of
      step 4.2 (step 9.3). Data written since that dump is lost; the runbook says so before the
      restore. _Amended 2026-10-16 ([ADR 0025](0025-postgresql-storage.md)); was: restoring the
      pre-promotion volume snapshot._
   2. The server refuses to start when the database records a migration that its binary does not
      embed, so a rollback across a migration without the restore fails its smoke check instead of
      running on a schema it does not know. Tests:
      `TestMigrate_Step6_2_RefusesMigrationsTheBinaryDoesNotEmbed`,
      `TestMigrate_Step6_2_AcceptsADatabaseItsBinaryMigrated`,
      `TestBuild_AFailingStepRefusesTheStartAndClosesTheStore` (composition: the migrate step's refusal
      stops the start and closes the store).

7. **Private book data stays private.** The catalogue, and any database copy (it holds catalogue
   rows), never enters the public repo, a PR build, a workflow artifact, a shared build cache or a
   log. It lives only in the private fly registry image and in the fly PostgreSQL cluster with its
   volume snapshots (and on the operator's machine for a downloaded dump, step 9.4). _Amended
   2026-10-16 ([ADR 0025](0025-postgresql-storage.md)); was: on the fly volumes._
   1. PR builds and the CI image check use the fixture catalogue `api/testdata/catalogue.json`.
   2. The release build never echoes the catalogue, the deploy key or a token (no shell tracing), and
      does not export the layer that holds the catalogue to a build cache that other workflows can
      read.

8. **Access control: private by default.**
   1. **Hard rule** (user, 2026-10-16): nothing reachable from the public internet serves the
      catalogue or the app without access control.
   2. **Default, reversible** (proposed, awaiting the user): both apps have **no public IP**. The first
      deploy uses `--no-public-ips` and allocates a private Flycast IPv6 only. Operators reach an app
      with `fly proxy` or fly WireGuard.
   3. Every deploy job (staging and production) checks the app's IPs before deploying and fails if
      the app has a public IP while no public-access option is configured. _Amended 2026-10-16 (user):
      for production the check is mandatory, with no bypass, until a public-access option with access
      control is built; only staging's honours the `PUBLIC_ACCESS_OPTION` variable._
   4. Options for public access, the choice awaiting the user:
      - **(a) Shared-secret HTTP basic auth** in the Go server, then public IPs. Recommended for use at
        the table from any phone.
      - **(b) Stay private**: players' devices join through WireGuard or Tailscale. Recommended if only
        the user's own devices need the app.
      - **(c) Real login**, which belongs to the multi-user storage design.

9. **Backups.** _Amended 2026-10-16 ([ADR 0025](0025-postgresql-storage.md)); was: daily
   snapshots of both app volumes (proposed retention 30 days production, 7 days staging), the
   pre-promotion volume snapshot, restore by attaching a volume made from a snapshot, a SQLite copy
   for download, Litestream rejected for now._
   1. The cluster's volume has fly's daily snapshots, with a set retention (30 days, answered
      2026-10-16; the runbook records the value set).
      A snapshot holds both databases.
   2. _Answered 2026-10-16 (user, "1a"); was: open, where a pre-promotion `pg_dump` is kept._ Every
      promotion first takes a **restore point**: a copy database of `critical_prod` on the same cluster,
      named after the release being promoted and the time, `critical_prod_pre_<tag>_<UTC time>` (e.g.
      `critical_prod_pre_v1_2_3_20261017153000`), made by `pg_dump` and `pg_restore` and verified (the
      same tables and recorded migrations as `critical_prod`, rows in it). It never leaves fly: not a
      GitHub artifact, a cache or a bucket (step 7). fly's daily snapshots of step 9.1 cover the loss of
      the cluster machine.
      1. The newest **3** restore points are kept; older ones are dropped by the promotion that adds one.
      2. The promotion refuses to take one when `critical_prod`, the copies it keeps and the new one
         would use more than 300 MB (the 1 GB volume is shared with the other app). The budget counts
         critical's databases only: `critical_prod` cannot size the other app's databases nor see the
         volume's free space, so the operator watches the volume's use (the runbook's "Promotion").
      3. A copy is locked like every database of the cluster: owned by `critical_prod`, CONNECT revoked
         from PUBLIC. `critical_prod` has the `CREATEDB` attribute for this, and only this (it creates,
         owns and drops its copies); `critical_staging` has not.
      4. It is made in an ephemeral `fly console` machine of the production app, started from the release
         image (which ships the PostgreSQL client of the cluster's major version) without starting the
         server, so the app's own secret is the only credential used and the running app is untouched.
   3. Restore: `critical_prod` is restored from a restore point (or a `pg_dump` taken by step 9.4)
      without touching `critical_staging`: the current database is set aside and the restore point is
      copied into its place, with no server running on it meanwhile (the runbook's "Rollback").
      Restoring a cluster volume snapshot rolls back every database of the cluster; it is the fallback
      when no usable restore point exists.
   4. `just db-backup <env>` downloads a `pg_dump` of that environment's database into a gitignored
      folder on the operator's machine. It is private data (step 7).

10. **Secrets and permissions.**
    1. Each app has its own app-scoped fly deploy token, stored in the GitHub environment of the same
       name: `staging` and `production`.
    2. The `production` environment only accepts deployments from `v*` tags. A tag runs the promotion
       workflow of its own commit, whose guards it could have edited out, so only a repository admin
       may move or delete a `v*` tag, and only an admin and the release job (the GitHub Actions app,
       through the workflow token) may create one (two tag rulesets, _added 2026-10-16; amended
       2026-10-17 (staging versions), was: only an admin may create one_). CI is the only maker of
       version tags.
    3. The rulebooks deploy key is a read-only key on `critical-rulebooks`, held only by the job that
       builds the release image.
    4. When a staging secret is missing, the main-branch build and deploy jobs skip with a notice and
       never fail main. A promotion with a missing secret fails: running it is a deliberate act
       (_amended 2026-10-17 (staging versions); was: pushing a tag is a deliberate act_).
    5. Smoke checks reach the app through `fly proxy`, so they work while the app is private.

## Proposed defaults awaiting the user

Applied until the user answers; each answer is recorded here with its date and flips the matching
part of this ADR to accepted. Answered rows keep their original default and say what was chosen.

| Question | Default | Options |
|---|---|---|
| Access for a public URL (step 8) | private, no public IP | (a) basic auth + public IPs, (b) stay private, devices join WireGuard / Tailscale, (c) real login |
| PostgreSQL and the second production machine (step 5) | SQLite, one machine per environment | **Answered 2026-10-16: PostgreSQL first**, no deploy until then; two production machines from the first deploy ([ADR 0025](0025-postgresql-storage.md)) |
| Production data | fresh database | **Answered 2026-10-16: fresh database** (_amended 2026-10-16; was: seeded once with the user's data, the local `.dev/critical.db` copied into `critical_prod` by the copy tool_): production's first boot seeds it from `seeds/` and the catalogue, like staging; the user re-enters the few character-sheet edits made in the app. See "Amendment (production starts from the seeds, 2026-10-16)" |
| Production approval | none: running the promotion from the tag is the act (_amended 2026-10-17; was: pushing the tag is the act_) | a required reviewer on the `production` environment |
| Names, region, size | `critical-staging` / `critical-prod`, region `cdg`, shared-cpu-1x 256-512 MB, 1 GB volumes, staging auto-stops | **Region answered 2026-10-16: `yyz`**; no app volume, the shared cluster `tournesol-db` of step 2.3 holds the data. App names and the app VM size stay the defaults; app names are global on fly and set in one place |
| Where the pre-promotion dump is kept (step 9.2) | open: no promotion runs until it is answered | **Answered 2026-10-16 ("1a"): a copy database on the same cluster** (a restore point, newest 3 kept), not a dump file; see "Amendment (restore point: a copy database, 2026-10-16)" |
| Snapshot retention of the cluster volume (step 9.1) | 30 days | **Answered 2026-10-16: 30 days** (the default) |
| A migration during a rolling production deploy (step 5.2) | a release's migrations are additive, so the previous release keeps working on the new schema while its machines are replaced | **Answered 2026-10-16: additive migrations** (the default); the `immediate` strategy is not used |
| Custom domain | none | a domain, only once public access is chosen |

## Consequences

- **What each step's test looks like.** Steps 1.1, 1.2, 1.4 and 6.2 are Go tests (router, health
  handler, migration runner). Steps 1.3 and 7.1 are checked by the CI image job, which builds the
  image with the fixture and smoke-checks it. Steps 3, 4, 8.3 and 10 are workflow behaviour: their
  check is a failing run on the broken case (a smoke check against the wrong commit, a missing
  restore point, a public IP, a missing secret), exercised once by hand and noted in the PR that adds
  the workflow. Steps 2.1, 2.2 and 5 are fly configuration, checked by a parse and required-keys
  test of the fly config files. _Amended 2026-10-16: steps 2.3 to 2.5 and 9.1 are the cluster's
  configuration, made with flyctl and not in the repo; their evidence (`fly status`, `fly ips list`,
  a refused cross-database connection, `fly secrets list` showing names only) is noted in the PR that
  provisions it, and the runbook holds the commands._
- **The release flow is driven by GitHub Actions**, with the fly CLI. The deploy runbook
  (`docs/deploy.md`, written with the fly configuration) holds the commands: one-time setup, release,
  rollback, restore, catalogue update.
- **A new catalogue version reaches production** by being in the image: the release build fetches
  the catalogue at build time, so it takes a main build (any merge, or re-running the build for the
  current main commit) and then a tag. The importer ([ADR 0021](0021-spell-catalogue.md) step 5)
  applies it at boot.
- **Promotion and registry layout.** _Settled 2026-10-17 by a test with a production deploy token;
  was: "if the production token cannot read staging's repository, the promotion copies the image"._
  fly's API lets an app's deploy token use only that app's images (the registry lets it read the
  staging repository, but `fly console` and `fly deploy` refuse another app's image), so the promotion
  copies the image by digest into the production app's repository, with the production token, before
  the restore point; it never rebuilds. The digest check of step 1.5 holds: the copy has the digest
  staging smoke-checked.
- **The release name is in the image.** _Amended 2026-10-17 (staging versions); was: "the release name
  is not in the image: the image is built before the tag exists; `/api/health` reports the commit, and the
  GitHub Release maps the tag to it"._ The version is computed before the build and baked in, so each
  environment reports its version in `/api/health`; the tag follows once staging passed. A failed staging
  run's image reports a version that never becomes a tag.
- **Downtime and machine count.** _Amended 2026-10-16; was: one production machine, no redundancy,
  until the storage move._ Production runs two machines on a one-node cluster: a machine failure
  leaves the other serving, a cluster failure stops staging and production until fly restarts the
  node, or a restore (step 9.3). Accepted for a single-user tool on a free allowance.
- **Staging auto-stop.** A stopped staging machine starts on the next request through fly's proxy;
  the deploy starts it anyway, so the smoke check right after a deploy reaches it.
- **Production starts from the seeds** (_amended 2026-10-16, twice; was: production is seeded once
  with the user's data by the copy tool of [ADR 0025](0025-postgresql-storage.md) step 9_): like
  staging, its first boot imports the catalogue and the seeds. Nothing of the local play is carried
  over; the user re-enters by hand, in the production app, the character-sheet edits made in the app
  (see the amendment below).
- **Not covered**: the multi-user model and login, a custom domain, preview environments
  per PR (a PR build never sees the catalogue, so a preview would only show the fixture), and
  dependency update automation.
- **docker-compose** remains for running the production image locally, rebuilt on the single image
  of step 1.

## Amendment (PostgreSQL on one fly cluster, 2026-10-16)

User, 2026-10-16, on the fly free allowance: _"can't request 1gb"_; _"if we need a volume, we can't go
with fly.io. unless it's just a legacy postgres db"_; _"I need free hosting for this app."_ The same
triage decided the storage port ([ADR 0025](0025-postgresql-storage.md)) and the fly topology below.
What changed in this ADR, each change marked in place with its date and the text it replaces:

- **Step 2**: the apps have no volume. One new fly legacy PostgreSQL cluster, `critical-db` (one node,
  smallest shared VM, 1 GB volume, region `yyz`, private), holds the databases `critical_staging` and
  `critical_prod`, one login user each, neither able to reach the other's; each app's DSN is the fly
  secret `CRITICAL_DSN`. Staging runs one auto-stopping machine, production two.
- **Step 4.2**: a `pg_dump` of `critical_prod` before every promotion replaces the volume snapshot;
  no dump, no deploy.
- **Step 5**: answered, option (b), PostgreSQL first; production runs two machines from the first
  deploy with a rolling deploy.
- **Step 6.1**: a rollback across a migration restores `critical_prod` from the pre-promotion dump.
- **Step 7**: the private data lives in the cluster and its snapshots instead of the app volumes.
- **Step 9**: daily snapshots of the cluster volume; the pre-promotion dump; restore from a dump;
  `just db-backup <env>` downloads a `pg_dump`; the Litestream line is removed (nothing to replicate
  on the app side).
- **Defaults**: PostgreSQL timing, production data (seeded once with the user's data) and region
  (`yyz`) answered; three new open questions (where the pre-promotion dump is kept, the cluster
  snapshot retention, a migration during a rolling deploy).
- **Context and Consequences**: the SQLite and one-machine statements are marked as superseded.

## Amendment (shared cluster tournesol-db, 2026-10-16)

`fly postgres create --name critical-db` (one node, 256 MB, 1 GB volume, `yyz`) was refused on
2026-10-16 with _"To create more than 1GB in volumes please add a payment method"_: the org's free
allowance holds one 1 GB volume, and the user's existing cluster `tournesol-db` already has it (an
earlier try failed on fly's Consul step, then passed). A new machine running PostgreSQL by hand
needs the same volume. User, 2026-10-16: _"ok so we reuse the existing db running. tournesol-db"_,
then, after the options were laid out (share it, or free its volume for a cluster of our own),
_"1 for now"_.

- **Step 2 and 2.3**: the cluster is `tournesol-db` (postgres-flex 17, one node, shared-cpu-1x
  256 MB, 1 GB volume, `yyz`, private), shared with the user's other app. Its other databases
  (`template1` included) keep CONNECT for the login users they have when the provisioning runs and
  lose it for PUBLIC, so a critical user reaches only its own database; the other app's users of that
  day keep their access. A user the other app gets later needs an explicit `GRANT CONNECT` (the
  runbook says how): the price of sharing the cluster.
- **Step 9.1**: the daily snapshots of the cluster volume, retention 30 days, also hold the other
  app's databases; restoring a snapshot rolls them back too, one more reason step 9.3 restores from a
  dump first.
- **Consequences**: the cluster stops when idle (scale to zero, as configured before this app used
  it) and starts on the next connection through its `.flycast` address, so the first request after an
  idle period waits for PostgreSQL to start.
- **Reversible**: if the other app's data is no longer needed, its volume can be freed and a cluster
  of this app's own created as first planned; the DSN secrets are the only thing to change.

## Amendment (production starts from the seeds, 2026-10-16)

User, 2026-10-16, before the first production deploy: _"we'll need to seed only non gameplay data
(data not affected by playing). the runs i've been doing is for testing"_, then, offered (A) a fresh
boot and re-entering the sheet edits by hand, (B) adding them to `seeds/`, (C) a partial copy tool:
_"we go with A"_.

- The local play (action log, its snapshots and corrections, buffs, round, spell slots, attack form)
  was testing and is not production data.
- The data play does not change is what the first boot seeds from `seeds/` and the catalogue: the
  character, proficiencies, bard skills, weapons, items, spells. A comparison of the user's local
  database with a fresh boot (`scripts/db-compare.sh`, names and counts only) found three edits made
  in the app that the seeds do not hold: the crit table, one added item and one dice favourite. The
  user re-enters them in the production app after its first deploy (docs/deploy.md, "user steps
  left").
- So production gets no copy: the copy tool of [ADR 0025](0025-postgresql-storage.md) step 9 is no
  longer needed for it (ADR 0025 step 9.3), and it is retired with the first production deploy
  (step 9.5).

## Amendment (restore point: a copy database, 2026-10-16)

The promotion needed a place for its pre-promotion `pg_dump` (step 9.2, open). Offered: the cluster's own
volume, a private store, or a dump taken by the operator before tagging. User, 2026-10-16: _"1a"_, a copy
database on the same fly cluster, named after the release, keeping the last few; not a GitHub artifact,
not a bucket; fly's daily snapshots of the cluster cover the loss of the machine. Same day: the apps stay
private until access control is decided, so production's no-public-IP check is mandatory.

- **Step 4.2**: the promotion takes a restore point, a verified copy database (step 9.2); no restore
  point, no deploy.
- **Step 8.3**: production's check has no bypass; staging's still honours `PUBLIC_ACCESS_OPTION`.
- **Step 9.2**: answered, the restore point `critical_prod_pre_<tag>_<UTC time>`, newest 3 kept, a 300 MB
  budget for `critical_prod` and its copies, made from an ephemeral console machine of the release image.
- **Step 9.3**: restore by copying the restore point into the place of `critical_prod`.
- **Privilege**: `critical_prod` gets `CREATEDB` (one attribute, prod only). It lets that user create
  databases on the shared cluster, never read or change the other app's (CONNECT stays locked, and a
  database it creates gets PUBLIC's CONNECT revoked at once); the size budget and the retention bound how
  much of the shared volume it takes. A separate role it can `SET ROLE` to would hold the same power for
  anyone with the app's secret, and a separate credential would have to be a second fly app's secret; both
  were judged not worth it for one user.
- **Tests**: `scripts/test-promotion.sh` (guards, step order, refusals, offline) and the restore point run
  for real against a throwaway PostgreSQL 17 by `scripts/image-smoke.sh` (the CI `image` job).

## Amendment (staging versions from the conventional commits, 2026-10-17)

**Proposed, awaiting the user's OK before merge.** User, 2026-10-17: _"have staging releases infer the
version number automatically from conventional commit to bump minor or patch version (major is manual).
then promotion to prod just promote latest staging version. this way we have no risk of generating a bad
tag and we know easily the different version on each environment. useful if we decide not to promote for a
while"_. Decided in the same triage: the range is all main commits since the last version, highest bump
wins; `feat` minor; `fix`, `refactor`, `perf` and non-conventional titles patch; a breaking change a minor
plus a notice; `docs`, `test`, `chore` and the `no-release` PR label make no release (no build, no deploy,
no tag), with no "force release" label; the version is baked in and reported by `/api/health`; the tag is
created only after the staging smoke check; the manual staging run with no input releases main's head, with
a version redeploys it; `just release` goes away and CI is the only maker of version tags.

- **Step 1.4**: `/api/health` reports `version` next to `commit`, `unknown` when not set.
- **Step 3**: the version first (3.1), the build with it (3.2), the deploy and smoke check with the version
  (3.3, 3.4), the tag last (3.5); one serialized run from the version to the tag; the manual run.
- **Step 4**: the promotion runs only by hand (`workflow_dispatch`) from a version tag; it has no tag-push
  trigger, so no tag starts it, CI's or an admin's (a tag CI creates would start no workflow anyway: GitHub
  starts none for events made with a workflow's `GITHUB_TOKEN`, except `workflow_dispatch` and
  `repository_dispatch`). One commit can carry two versions (a manual release of a tagged head, a new
  catalogue): the guard of step 4.1 takes the staging deployment recorded as the tag's version, so each
  version promotes its own image. Promoting the newest staging version on demand and making a major are
  later changes.
- **Step 10.2**: the tag rules split in two rulesets: moving and deleting `v*` tags stays admin-only;
  creating one is open to admins and the GitHub Actions app. Any workflow token of the repository with
  `contents: write` can therefore create a `v*` tag; such a tag starts no workflow, the release refuses to
  release while the highest `v*` tag is not before its commit ("a newer release exists", also what stops an
  older run re-run after a newer release), and the operator promotes only versions the release made
  (runbook, "GitHub").
- **Consequences**: the release name is in the image.
- **Tests**: `scripts/test-release-version.sh` (the rules as a table, the range and labels on a fixture
  repository, the tag's API calls), `scripts/test-release-scripts.sh` and `scripts/test-staging-deploy.sh`
  (the version through the build, the deploy, the smoke and the record; the job's steps), the health tests of
  step 1.4, and the CI `image` job (`scripts/image-smoke.sh` checks the baked version).
