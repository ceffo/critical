# Deploy runbook (fly.io)

Operator commands for [ADR 0024](adr/0024-deployment.md). Configuration lives in `deploy/fly.staging.toml`
and `deploy/fly.prod.toml` (checked by `just check-fly`, and by `just validate` through the Go test in
`api/internal/flycheck`). The image is built by the release workflow and passed with `--image`; the
config never names it. Steps marked **(user)** need fly credentials or GitHub admin rights; an agent
never runs them. Nothing here is public: the apps and the database have no public IP (ADR 0024 step 8).

Topology (ADR 0024 step 2, as amended): apps `critical-staging` (one machine, stops when idle) and
`critical-prod` (two machines, always running, rolling deploys), `shared-cpu-1x` with 512 MB, region
`yyz` (Toronto: fly has no Montreal region; user, 2026-10-16). The apps have **no volume**: their data
lives in the fly PostgreSQL cluster `tournesol-db`, shared with the user's other app, in the databases
`critical_staging` and `critical_prod`. Each app's DSN is its fly secret `CRITICAL_DSN`. App names are
global on fly; to rename an app or change the region, edit `app` / `primary_region` in the two toml
files; the Go test checks the app names against the file names.

```bash
fly=~/.fly/bin/fly   # flyctl on the operator's machine
```

## State and user steps left

First private deploy, 2026-10-16, image of commit `1648083` (one digest, both apps):

| App | Machines | Addresses | Checked |
|---|---|---|---|
| `critical-staging` | 1 (stops when idle) | `private_v6` only | `just fly-private`, `just fly-smoke` (health with the commit, database read); an action-log entry survived `fly machine restart` and a redeploy (`just fly-persistence`); both cold-cluster cases above |
| `critical-prod` | 2 (always on) | `private_v6` only | `just fly-private`, `just fly-smoke` on each machine; `just fly-db-counts tournesol-db critical_prod`: the 22 tables a fresh boot seeds, no play |

User steps left:

1. **Re-enter the character-sheet edits** the seeds do not hold, in the production app (`just fly-open
   critical-prod`, "Reaching a private app"): the crit table, the item added in the app, the dice favourite
   ("Production data").
2. **`RULEBOOKS_DEPLOY_KEY`** for the release build ("GitHub" below; until it is set, every main run
   of the release workflow ends green with a "Release skipped" notice naming it, runs no flyctl
   step and deploys nothing). The environments and both `FLY_API_TOKEN` secrets are set (2026-10-16). Then check the first
   real release run ("Release build"), none of which an agent can see:
   1. it is green, and its notice names `registry.fly.io/critical-staging:<sha>` and the rulebooks commit:
      the deploy key reached GitHub over ssh with the pinned host key, and the staging deploy token could
      `fly auth docker` and push;
   2. `$fly auth docker`, `docker pull registry.fly.io/critical-staging:<sha>`, then `docker inspect -f
      '{{json .Config.Labels}}' registry.fly.io/critical-staging:<sha>` shows
      `org.opencontainers.image.revision` and `critical.rulebooks.revision`;
   3. its log holds no catalogue JSON and no key material: only the clone, the build, the push lines and
      the notice with the reference and the rulebooks sha;
   4. the `deploy staging` job that follows is green (no public IP, smoke on the machine) and Deployments
      shows a `deploy-staging` deployment of the sha with the digest; the other post-merge checks are in "Staging deploy".
3. **The public-access option** (ADR 0024 step 8.4): until it is picked the apps stay private.
4. **CREATEDB for `critical_prod`**, the one privilege the promotion's restore point needs (ADR 0024 step
   9.2, "Promotion" below), once the user has agreed to it: `just fly-db-provision tournesol-db prod` (a
   rerun: no password or secret changes; its checks accept the user's own restore points). Until then every
   promotion stops at the restore point and deploys nothing.
5. **The first promotion** (`just release <version>`, "Promotion"), whose post-run checks only a real run
   can make: listed at the end of "Promotion".

## The PostgreSQL cluster (user)

`tournesol-db`: postgres-flex 17, one node (`shared-cpu-1x`, 256 MB), one encrypted 1 GB volume
`pg_data`, region `yyz`, a private Flycast address only. It existed before this app; it was not
created for it. Why a shared cluster: `fly postgres create --name critical-db` was refused on
2026-10-16 with _"To create more than 1GB in volumes please add a payment method"_ (ADR 0024,
"Amendment (shared cluster tournesol-db, 2026-10-16)").

**What it costs** (as observed on 2026-10-16, org `personal`): the org has no payment method; fly
lets it hold one 1 GB volume and refuses a second (the message above), so this cluster's volume is the
only one and the apps must stay volume-less. Creating apps, secrets and private IPs asked for nothing.
Whether the machines' running time stays free is fly's billing and was not checked here; adding a
payment method is the user's decision only.

**It stops when idle.** The cluster scales to zero (configured before this app used it): its machine
stops with no connection and starts on the next one through `tournesol-db.flycast`, so the first
request after an idle period waits a few seconds for PostgreSQL. Idle means no open connection for an
hour (`FLY_SCALE_TO_ZERO=1h` on its machine): the app's pool closes connections unused for 5 minutes, so
quiet apps let it stop, and a server booting or a request arriving while it starts waits for it (up
to 60 s per new connection) instead of failing (`docs/backend.md`, "Database"). Checked on staging, 2026-10-16: with the cluster
stopped, a booting server got `EOF` twice, the connection started the cluster (event source `proxy` in
`fly machine status`) and the server listened 5 s after its start; with the cluster stopped under a
running server holding pooled connections, the next database read answered 200 in 2.6 s. A DSN that
cannot work (wrong password, unknown host, `sslmode=require`) fails at once instead of waiting.
`fly ssh` does not start it:

```bash
$fly status -a tournesol-db                          # machine state; role "error" only means stopped
$fly machine start <machine id> -a tournesol-db      # before fly ssh / fly postgres connect
$fly ips list -a tournesol-db                        # private ingress (Flycast) only, no public v4/v6
```

Residual risk, accepted: **one 500 is possible when a connection is reset abruptly under load** (the
cluster killed, not stopped, while a statement runs on a pooled connection). A clean stop (FATAL
57P01/57P02) or an EOF reaches the pool as a bad connection, and the pool retries on a new one; an
abrupt reset (TCP RST) reaches it as a network error on the statement, which lib/pq returns as it is
and the request fails once. It is not converted into `driver.ErrBadConn` on purpose: database/sql then
re-runs the statement on a new connection, and an autocommit statement the server may already have
executed would run twice. The 5-minute idle close keeps connections from being reused across the
cluster's idle stop, so this needs the cluster to die during traffic.

To create a cluster of this app's own instead (once its volume is freed, or with a payment method):
`$fly postgres create --name critical-db --org personal --region yyz --initial-cluster-size 1
--vm-size shared-cpu-1x --vm-memory 256 --volume-size 1`, its output redirected to a private file
(mode 600): on success it prints the superuser password and a connection string. Then rerun the
provisioning below with that cluster's name.

### Databases, users and the apps' secrets

```bash
just fly-db-provision tournesol-db staging prod      # scripts/fly-db-provision.sh
```

For each environment it creates (or keeps) the login user `critical_<env>`, owner of the database
`critical_<env>`. CONNECT on that database belongs to its user only. Every other database of the
cluster that accepts connections (`template1` included) keeps CONNECT for the login users it has at
run time and loses it for PUBLIC, so a critical user reaches only its own database and the other app's
current users keep their access. `critical_prod` also has the `CREATEDB` attribute (`critical_staging`
has not): the promotion's restore points are copy databases it creates ("Promotion"); the checks below
accept CONNECT on its own `critical_prod_pre_*` copies.

**A user the other app gets later** (a `CREATE ROLE`, a `fly postgres attach` of a new app) has no
CONNECT on the locked databases, and a rerun does not add it (the databases are already locked). Grant
it by hand: `$fly postgres connect -a tournesol-db`, then `GRANT CONNECT ON DATABASE <db> TO <user>;`.
A database created later keeps PUBLIC CONNECT until the next run of the provisioning locks it.

**Passwords.** A user gets a generated password when it is created, sent to the server only as a
SCRAM-SHA-256 verifier, and its app's secret
`CRITICAL_DSN=postgres://critical_<env>:…@tournesol-db.flycast:5432/critical_<env>?sslmode=disable` is
piped into `fly secrets import` right after (never printed): staged while the app has no machine (the
first `fly deploy` applies it), applied at once otherwise. An existing user's password and secret are
never touched by a rerun, so a rerun on a running app changes nothing it uses. `fly secrets list -a
critical-<env>` shows the name and a digest, never the value. `sslmode=disable` (user, 2026-10-16): the
cluster has no TLS (PostgreSQL answers `N` to a TLS request; the Flycast `pg_tls` handler answers `S`,
then closes), and the app's driver, unlike `psql` with `prefer`, does not fall back to plain, so every
connection of the first staging deploy ended in `EOF`. The traffic stays inside fly's WireGuard
private network, which encrypts it; `fly postgres attach` writes `sslmode=disable` too. A secret set
with `sslmode=prefer` is replaced with `--rotate`.

**Checks**, every run: each critical user holds CONNECT on its own database and on no other (privileges
read as the cluster's operator). When the run set a password it also connects through `fly proxy` to
`tournesol-db.flycast`: own database usable (may create in its schema), every other connection refused
(`permission denied for database`). Then it sets the volume's snapshot retention.

**Rotation** (a leaked password): `just fly-db-provision --rotate
tournesol-db <env>`. It sets a new password, then applies the new secret, so **the app is briefly out**:
from the password change until fly has restarted each of its machines with the new secret, new
database connections fail (connections already open keep working). Before the app's first deploy it has
no machine and the rotation costs nothing. `--password-dir ~/.config/critical` also writes the new
password to `~/.config/critical/critical_<env>.password` (mode 600) for the operator; delete it after
use. If a run stops between the password change and the secret, rerun it with `--rotate`.

### Reaching a database

```bash
$fly postgres connect -a tournesol-db -d critical_prod     # psql as the cluster's operator (superuser)
just fly-db-proxy tournesol-db                             # prints the psql connection string (free local port), Ctrl-C stops it
```

The second form (`scripts/fly-open.sh --db`: `fly proxy` to `tournesol-db.flycast:5432` on a free local
port, never the compose PostgreSQL's 5432) needs the user's password, which is only in the app's secret. To look at the data,
use `fly postgres connect` (`SET ROLE critical_prod;` to act as that user); never rotate a running
app's password just to log in: the rotation is an outage. Production data is
private (ADR 0024 step 7): never paste query output into an issue, a PR or a log.

### Snapshots

fly takes a daily snapshot of the cluster volume, kept 30 days (ADR 0024 step 9.1, set by the
provisioning). A snapshot holds every database of the cluster, the other app's too: restoring one rolls
them all back, so `critical_prod` is restored from a promotion's restore point first (step 9.3,
"Rollback").

```bash
$fly volumes list -a tournesol-db                    # retention, and the volume id
$fly volumes snapshots list <volume id>
```

## One-time setup per app (user)

Repeat for `staging` and `prod` (`<env>` = `staging` or `prod`, app = `critical-<env>`). Both apps exist
(created 2026-10-16).

```bash
$fly auth login
$fly apps create critical-<env> --org personal
$fly tokens create deploy -a critical-<env>        # app-scoped token: store it in GitHub, nowhere else
```

No volume: never `fly volumes create` on an app (the free allowance has none left, and the apps are
stateless, ADR 0024 step 5.1). `CRITICAL_DSN` must be set (provisioning above) before the first deploy:
without it the server falls back to its local development default (`localhost`) and fails its first
connection.

First deploy of an app, with a private Flycast address only (no public IP). fly creates two machines
for an app without volumes; staging wants one:

```bash
just release-image critical-staging critical-prod  # the clean HEAD, real catalogue, pushed to both apps' registries
$fly deploy -c deploy/fly.prod.toml -a critical-prod --image registry.fly.io/critical-prod:<sha> --no-public-ips          # two machines
$fly deploy -c deploy/fly.staging.toml -a critical-staging --image registry.fly.io/critical-staging:<sha> --no-public-ips --ha=false   # one machine
$fly ips allocate-v6 --private -a critical-<env>
just fly-private critical-<env>                    # fails on any public address
just fly-smoke critical-<env> <sha>                # every machine: health with the commit, a database read, the web app
```

A first deploy reports a health-check timeout until the app has its private address and can reach the
database; the machine keeps running. Later deploys come from the workflows; fly allocates no public IP
on a redeploy of an app that already has an address (seen on staging's redeploy, 2026-10-16), and the
workflows pass `--no-public-ips` anyway. If `fly ips list` shows a public IP, release it with
`$fly ips release <address> -a critical-<env>` unless a public-access option has been chosen (ADR 0024
step 8.4).

### GitHub (user)

`scripts/github-environments.sh` (admin rights on the repository, fly credentials) creates the
environments `staging` and `production`, restricts `production` to tags `v*`, and sets each
`FLY_API_TOKEN` from a new deploy token of its app, piped, never printed (done 2026-10-16; `staging` is also restricted to the branch `main`; a rerun makes
new tokens, `--no-tokens` keeps the secrets). `RULEBOOKS_DEPLOY_KEY` stays a user step:

| Environment | Secret | Content |
|---|---|---|
| `staging` | `FLY_API_TOKEN` | deploy token of `critical-staging` |
| `staging` | `RULEBOOKS_DEPLOY_KEY` | private half of the read-only deploy key (below), used by the release build |
| `production` | `FLY_API_TOKEN` | deploy token of `critical-prod` |

Restrict the `production` environment to deployment branches and tags matching `v*` (ADR 0024 step
10.2). Optionally add a required reviewer (default: none, pushing the tag is the act).

The rulebooks deploy key is read-only and belongs to the private `critical-rulebooks` repo:

```bash
ssh-keygen -t ed25519 -N '' -C critical-release-build -f ./rulebooks_deploy_key
gh repo deploy-key add ./rulebooks_deploy_key.pub --repo <owner>/critical-rulebooks   # read-only is the default
gh secret set RULEBOOKS_DEPLOY_KEY --env staging --repo <owner>/critical < ./rulebooks_deploy_key
shred -u ./rulebooks_deploy_key ./rulebooks_deploy_key.pub
```

Never commit a token or key; the repository holds none.

## Reaching a private app

```bash
just fly-open critical-staging              # or critical-prod: prints the URL to open, Ctrl-C stops it
curl -s http://127.0.0.1:<port>/api/health  # {"status":"ok","commit":"<sha>"}, <port> from the line above
$fly ssh console -a critical-<env>          # a shell on the machine
$fly logs -a critical-<env>
```

`just fly-open <app> [port]` (`scripts/fly-open.sh`) runs `fly proxy` from a free local port to the
app's `.flycast` address on port 80, bound to 127.0.0.1, waits until `/api/health` answers (a stopped
machine or database takes a few seconds), prints `http://127.0.0.1:<port>/` and the commit, and keeps
the proxy until Ctrl-C. Never use a fixed 8080 (or 5173, 5432): `just dev` holds those, so the proxy
fails to bind or the browser reaches the local stack instead of fly. A port given explicitly is
refused when taken. Rerun it any time; each run gets its own port.

Through the `.flycast` address, fly's proxy starts a stopped machine (staging stops when idle) and
balances between machines; the address answers on the service ports (80, and 443 with TLS), not on 8080.
`$fly proxy <free port>:8080 -a critical-<env>` without the address goes straight to one running
machine's port 8080 and starts nothing. A request can be pinned to one machine with the header
`fly-force-instance-id: <machine id>` (what `just fly-smoke` does for each machine).

For a device that must reach the app without `fly proxy`, join the org's WireGuard network
(`$fly wireguard create`, user) and use the app's `.flycast` address. A public IP, basic auth and a
custom domain come only once the user picks a public-access option (ADR 0024 step 8.4); this runbook
gets those steps then.

## How a release flows

ADR 0024 steps 3 and 4, driven by GitHub Actions:

1. A merge to `main` with green CI builds the image with the real catalogue and pushes it tagged with
   the commit sha ("Release build" below, `.github/workflows/release.yml`); the same run then deploys that
   image to staging and smoke-checks it ("Staging deploy" below, `.github/workflows/deploy-staging.yml`).
2. A tag `vMAJOR.MINOR.PATCH` on a `main` commit that passed staging takes a restore point of
   `critical_prod` (no restore point, no deploy), deploys the **same image** to production, smoke-checks
   it and creates the GitHub Release ("Promotion" below, `.github/workflows/promote.yml`).

```bash
git switch main && git pull
just release 1.2.3        # user: the deliberate act; tags main's head v1.2.3 and pushes the tag
```

### Release build

`.github/workflows/release.yml` (ADR 0024 steps 3.1, 7 and 10.3-10.4), job `release image`:

- **When.** It runs on `workflow_run` of the `CI` workflow, and its job only when that run concluded
  `success` for a `push` to `main` of this repository: a red or cancelled CI run, a pull request (a fork's
  branch named `main` included) and a fork never build. It is not a job of `ci.yml` because CI cancels a
  running `main` build when a newer commit lands; release jobs are serialized instead (the job's
  `concurrency: release-main`, never cancelled once started: a newer green commit replaces only a queued
  one). The group is on the job, not the workflow: every CI completion on a branch named `main` (a red
  one, a fork's) starts a run of this workflow, and only a job that passes the `if` joins the group, so a
  skipped run never cancels a pending green release. GitHub runs a
  `workflow_run` workflow as it is on `main`, so a pull request cannot change what runs with the secrets,
  and the workflow only starts once this file is on `main`.
- **Secrets**, from the GitHub environment `staging`: `FLY_API_TOKEN` and `RULEBOOKS_DEPLOY_KEY`. When one
  is missing (`scripts/release-secrets.sh`) the run ends green with a `Release skipped` notice naming it,
  and nothing is built: that is what every main run shows until the user sets the key ("GitHub" below).
- **Catalogue** (`scripts/release-catalogue.sh`): an ssh-agent gets the deploy key on stdin, then one
  shallow, blob-less clone of `critical-rulebooks` at its default branch, with github.com's host key
  pinned; only `dist/catalogue.json` is read, into the runner's temp directory, outside the checkout. The
  agent and the clone are removed when the script ends, the catalogue when the build step ends. It is
  never printed, never an artifact, never in an actions cache (the workflow has none) or an exported build
  cache.
- **Image** (`scripts/release-image.sh`, the script `just release-image` runs too): one plain `docker
  build` of the clean checkout with the catalogue as the `catalogue` build context and `COMMIT`, labelled
  `org.opencontainers.image.revision=<commit>` and `critical.rulebooks.revision=<rulebooks commit>`, then
  `fly auth docker` and a push to **`registry.fly.io/critical-staging:<full commit sha>`** (the app name is
  read from `deploy/fly.staging.toml`). The staging app's repository is the one image repository of the
  release flow: the staging deploy token can push only there. Production gets the same digest from it at
  promotion (ADR 0024, consequences, "Promotion and registry layout"), never by a rebuild. The run's
  notice names the pushed reference and the rulebooks commit; the push line in the log gives the digest.
  **The digest is the image's identity, not the tag**: a re-run for a new catalogue pushes a new digest
  under the same `<sha>` tag, so the staging deploy records the digest it smoke-checked and the promotion
  deploys and compares that digest (`registry.fly.io/critical-staging@sha256:…`), never the tag (ADR 0024
  step 1.5).
- **No shell tracing** anywhere (`set -x` is absent), and none of the scripts prints a secret.
  `just validate` runs `scripts/test-release-scripts.sh`, which exercises the three scripts offline (docker
  and fly stubbed, a local rulebooks repo and a generated key): the missing-secret no-op, stdout of the
  fetch being only the sha, the labels and the full-sha tag, a dirty tree refused. Its rulebooks remote is
  a `file://` repository, so ssh itself (the agent's key reaching GitHub, the pinned host key) is only
  proven by the first real run (user steps left).

### Staging deploy

The composite action `.github/actions/staging-deploy` (ADR 0024 steps 3.2-3.3, 8.3, 10.1 and 10.5) runs
`scripts/staging-deploy.sh <sha>` and records the result. Two jobs `deploy staging` use it, both with
`environment: staging`: one in `release.yml`, one in `deploy-staging.yml` (manual). It is not a reusable
workflow: a called workflow's job does not see the secrets of its own environment (the first real run
read an empty token), and a calling job cannot have an environment. The job passes
`secrets.FLY_API_TOKEN` to the action as an input.

- **When.** `release.yml`'s job `deploy staging` (`needs` the image job) runs only when the image was
  pushed: with a secret missing the image job builds nothing and the deploy job is skipped, so main stays
  green. A manual run redeploys an image that is already in the registry (below). Either way the job runs
  on `main` only, is serialized by `concurrency: deploy-staging` and is never cancelled once started (a
  newer commit replaces only a queued one).
- **Secret.** `FLY_API_TOKEN` of the `staging` environment, read only by the action's secret check and
  deploy step. Once the image is pushed, an empty token **fails** the run (`Staging deploy` error): it is a
  misconfiguration, and a green skip would leave staging on the old image. A manual run fails the same way
  (someone asked for a deploy that cannot happen); only the release build keeps main green on missing secrets.
- **Steps** (`scripts/staging-deploy.sh`, reusing `scripts/fly-private-check.sh` and `scripts/fly-smoke.sh`):
  1. **No public IP**: the job fails before deploying if the app has an address other than a Flycast one.
     The check is skipped, with a notice, only when the repository (or environment) variable
     `PUBLIC_ACCESS_OPTION` is set, which the user does when picking an option (ADR 0024 step 8.4). Until
     then the apps stay private.
  2. `fly deploy -c deploy/fly.staging.toml --image registry.fly.io/critical-staging:<sha>`; the machine is
     started if it was stopped.
  3. The smoke check through `fly proxy` (the app is private): on every machine, `/api/health` is 200 with
     `status` ok and `commit == <sha>`, `/api/character` is 200 (a read through the database) and `/` is
     the web app. A cold start of the machine or of the shared database is waited for (up to 90 s per
     machine).
  4. The machines must run exactly one image **digest**; it is printed and recorded.
- **Record.** The job creates a GitHub deployment of the environment `staging` with
  `scripts/record-deployment.sh`: task `deploy-staging`, ref the full sha, payload `{"commit", "digest"}`,
  and one status, `success` (description: the digest) or `failure`. GitHub also creates a deployment of
  its own for every job in the environment (task `deploy`, ref `main`'s head) and marks earlier
  deployments of the environment `inactive` when a newer one succeeds, so **the latest status of a
  `deploy-staging` deployment becomes `inactive` within minutes: never read "the latest status"**. How the
  promotion must read it ("staging ran this sha with this digest"):
  1. list `GET /repos/{repo}/deployments?environment=staging&task=deploy-staging&sha=<sha>` (the task
     filter drops GitHub's own deployments, the sha is the full commit);
  2. for each, list `GET /repos/{repo}/deployments/{id}/statuses` and keep the ones whose **status
     history contains `success`** (`inactive` only means superseded; a `failure` deployment has no
     `success`);
  3. take the newest of those by `created_at` (a rerun for a new catalogue records a new digest under the
     same sha) and its `payload.digest`; none means staging did not run and pass that sha: refuse.
  The promotion compares that digest with the one it deploys (`registry.fly.io/critical-staging@sha256:…`).
- **Config of the image's commit.** The fly config (`deploy/fly.staging.toml`) is checked out at the
  image's own sha (that file only), the scripts and the workflow come from `main`: a dispatched older image
  is deployed with the config that matches its Dockerfile, and a sha that predates the file fails at once.
- **A failed run.** The log of `Deploy and smoke-check` says which check failed: `fly-private-check` (a
  public address, nothing deployed), `fly-smoke: FAIL (...)` with the machine and the status or body
  (`HTTP 000` is a machine or proxy that never answered; a wrong `commit` is the old image still serving or
  a wrong tag), or a digest mismatch. After a failed deploy or smoke it prints the app's recent releases
  (`fly releases --image`): redeploy the previous one's image with the command in "Rollback", or fix and
  rerun. The deployment is recorded as `failure`, so the promotion refuses that sha.
- **Redeploy a given sha.** Actions > Deploy staging > Run workflow on `main`, with the full commit sha
  of an image already pushed to `registry.fly.io/critical-staging:<sha>` (the release build's notice names
  it). By hand: `$fly deploy -c deploy/fly.staging.toml --image registry.fly.io/critical-staging:<sha>`
  then `just fly-smoke critical-staging <sha>`.
- **Offline test.** `scripts/test-staging-deploy.sh` (part of `just validate`) runs both scripts against
  stubs of fly and gh: a public IP deploys nothing, the deploy and smoke arguments, the rollback listing,
  the digest output, the recorded deployment, and the workflows' gating. The real proxy, deploy and
  GitHub API calls are proven only by the first real run.
- **Environment rule.** `scripts/github-environments.sh` restricts `staging` to deployments from the
  branch `main` (and `production` to tags `v*`), so a job from another branch, even one that edits this
  workflow, gets neither the environment nor its `FLY_API_TOKEN`; the `github.ref` check in the workflow is
  only the readable half. Set 2026-10-17 (`--no-tokens`).
- **Post-merge checks (user, the first real runs; none can be faked offline).**
  1. The release run is green; the deploy job proxies to a stopped staging machine and the cold
     `tournesol-db` within 90 s, and prints the digest.
  2. Deployments shows a `deploy-staging` entry with ref = the sha and the digest in its payload; after the
     job its latest status is probably `inactive` while the history keeps `success` (the reading rule above).
  3. Actions > Deploy staging > Run workflow on `main` with an existing sha redeploys it and the smoke is
     green.
  4. A dispatch from a branch other than `main` is refused by the environment rule (and the job is skipped).
  5. Two quick merges: the older queued deploy is replaced by the newer, and a deploy already running is
     never cancelled.

### Promotion

`.github/workflows/promote.yml` (ADR 0024 steps 4, 8.3, 9.2 and 10), job `promote to production`, runs
`scripts/promote.sh <tag>`.

- **When.** On the push of a tag `v*`, and by hand (Actions > Promote > Run workflow, "Use workflow from" a
  **tag**: the rollback). The job runs only for a tag ref, in the environment `production`, which accepts
  deployments from `v*` tags only, so a run from a branch gets neither the environment nor its token. It
  runs the tag's own scripts and fly config (the ones of its image). Serialized by `concurrency:
  deploy-production`, never cancelled once started.
- **Starting it.** `just release <version>` (`scripts/release-tag.sh`) tags the head of `main` and pushes
  the tag. It refuses, before tagging: a version that is not `MAJOR.MINOR.PATCH` (`1.2.3` or `v1.2.3`), one
  not greater than the highest `v*` tag (version order: `1.10.0` > `1.9.0`), a branch other than `main`, a
  dirty tree, a `main` other than `origin/main`'s head after a fetch, and a commit staging has not passed
  (it reads the staging record with `gh`, so it waits for the staging deploy of the merge). Pushing a tag
  by hand (`git tag -a v1.2.3 -m ... <sha> && git push origin v1.2.3`) starts the same workflow; its guards
  refuse the same commits.
- **Secret.** `FLY_API_TOKEN` of `production` (the deploy token of `critical-prod`). An empty one **fails**
  the run before anything else (`Promotion` error): a tag push is a deliberate act (step 10.4).
- **Steps**, each stopping the promotion when it fails:
  1. **Guards** (`scripts/release-guards.sh all <tag>`): the tag is `vMAJOR.MINOR.PATCH`; its commit is
     on `origin/main`; staging ran and passed that commit, read as "Record" above says, which gives the
     **digest**; and `registry.fly.io/critical-staging@<digest>` is in the registry (`docker manifest
     inspect` after `fly auth docker` with the production token). Nothing is written before they pass.
  2. **No public IP** on `critical-prod` (`scripts/fly-private-check.sh`). Mandatory: unlike staging, no
     variable skips it; the apps stay private until a public-access option with access control is built
     (ADR 0024 step 8).
  3. **Restore point** (`scripts/restore-point.sh`, steps 4.2 and 9.2): `fly console -a critical-prod
     --image <that image>` starts an ephemeral machine of the production app from the release image (which
     ships the PostgreSQL 17 client; the machine has the app's `CRITICAL_DSN` and does not start the
     server), uploads the script and runs it; the machine is destroyed after. The script `pg_dump`s
     `critical_prod` and `pg_restore`s it into a new database on the same cluster,
     **`critical_prod_pre_<tag with _>_<UTC yyyymmddhhmmss>`** (e.g. `critical_prod_pre_v1_2_3_20261017153000`:
     the data as it was before that release), owned by `critical_prod`, CONNECT revoked from PUBLIC. It is
     verified (the same tables and recorded migrations as `critical_prod`, rows in it, PUBLIC locked out),
     then the copies beyond the **newest 3** are dropped. It refuses first: a name that exists, the user
     without `CREATEDB` (user step 4 above), and a projected use of `critical_prod` plus its kept copies plus
     the new one over **300 MB** (the 1 GB volume is shared with the other app). A copy that fails its
     verification is dropped. The log shows names, counts and sizes only; `pg_restore`'s own error text
     (which can quote rows) stays in the ephemeral machine, only the object it names is printed. The step
     passes only on the script's last line `restore-point: OK <name>`.
  4. `fly deploy -c deploy/fly.prod.toml -a critical-prod --image registry.fly.io/critical-staging@<digest>
     --no-public-ips`: rolling, one machine at a time.
  5. **Smoke** (`scripts/fly-smoke.sh critical-prod <commit>`, every machine: health with the commit, a
     database read, the web app), then every machine runs exactly that digest.
  6. **GitHub Release** of the tag with generated notes, naming the image digest and the restore point.
     It exists already on a rerun or a rollback (an older tag): kept.
  After a failure past the restore point it prints the app's recent releases: roll back as in "Rollback".
- **Why a copy database** (user, 2026-10-16, "1a"): it stays inside fly and the cluster, private like
  production itself (ADR 0024 step 7): never a GitHub artifact, a cache or a bucket. fly's daily snapshots of
  the cluster volume cover the loss of the machine (step 9.1).
- **Registry.** The production deploy pulls the image from the staging app's repository by digest (ADR
  0024, "Promotion and registry layout"). Whether the production deploy token may read it is only proven by
  the first real run: if not, guard 1 fails on the image with a clear message, nothing is written, and the
  copy of the image into `critical-prod`'s repository is the follow-up.
- **Offline test.** `scripts/test-promotion.sh` (part of `just validate`): each guard on fixture
  repositories and a stubbed GitHub API (the history-not-latest reading, the newest success, a string
  payload), the order of the promotion's steps and what each failure prevents (fly stubbed), every refusal
  of `just release`, and the workflow parsed. The restore point's SQL runs for real in the CI `image` job
  (`scripts/image-smoke.sh`: copies, retention, the refusals) against a throwaway PostgreSQL 17.
- **Post-run checks (user, the first real promotion; none can be faked offline).**
  1. The run is green; its log shows the guards' commit and digest, `fly-private-check: OK`, the restore
     point lines (`fly console` created and destroyed its machine; no server log line in between), the
     rolling deploy, `fly-smoke: OK` with both machines, and the release.
  2. `$fly postgres connect -a tournesol-db`, `\l critical_prod*` lists the copy, owned by `critical_prod`,
     no `=c/` (PUBLIC) in its access privileges.
  3. `$fly machine list -a critical-prod` shows the two app machines only (the console machine is gone).
  4. The GitHub Release exists, with the digest.
  5. Actions > Promote > Run workflow from `main` is refused (the job is skipped, the environment refuses
     the branch).

By hand, the equivalent of a deploy is
`$fly deploy -c deploy/fly.<env>.toml -a critical-<env> --image <ref>`: a rolling deploy, one machine
at a time, so production keeps serving from the other (ADR 0024 step 5.2). A release's migrations are
additive, so the previous release keeps working on the new schema while its machines are replaced.

## Rollback

Run the promotion again for the previous tag: Actions > Promote > Run workflow, "Use workflow from" that
tag (or "Re-run all jobs" on its run). It redeploys that tag's image by its staging digest, never rebuilt,
after a restore point of the current data. By hand (operator, production token):

```bash
git fetch origin --tags && git switch --detach v1.2.2
fly auth docker && scripts/promote.sh v1.2.2
```

**Across a migration** the older binary refuses to start on the newer schema (ADR 0024 step 6.2): restore
`critical_prod` from the restore point taken **before the release being undone** (`critical_prod_pre_<that
tag>_<time>`), then promote the older tag (step 9.3). Everything written since that restore point is lost:
say so to the user first, and get their yes.

1. Remove the app's machines, so no server boots on the restored database before the older release (a
   newer one would migrate it again): `$fly scale count 0 -a critical-prod --yes`.
2. As the cluster's operator (`$fly postgres connect -a tournesol-db`, connected to `postgres`), set the
   current database aside and copy the restore point into its place (the restore point stays):

   ```sql
   \l critical_prod_pre_*
   ALTER DATABASE critical_prod ALLOW_CONNECTIONS false;
   SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'critical_prod';
   ALTER DATABASE critical_prod RENAME TO critical_prod_failed_<yyyymmdd>;
   CREATE DATABASE critical_prod TEMPLATE "critical_prod_pre_<tag>_<time>" OWNER critical_prod;
   REVOKE ALL ON DATABASE critical_prod FROM PUBLIC;
   GRANT CONNECT, TEMPORARY ON DATABASE critical_prod TO critical_prod;
   ```

3. Promote the older tag (above). Its deploy recreates the two machines from `deploy/fly.prod.toml`; if it
   reports no machine, `$fly scale count 2 -a critical-prod --yes` and run it again.
4. Once production checks out, `DROP DATABASE critical_prod_failed_<yyyymmdd>;` (it takes room on the shared
   volume; it accepts no connection meanwhile).

A restore point is also the way back from bad data without a rollback (same steps, without changing the
release: promote the current tag again in step 3).

## Restore from a cluster snapshot (user, last resort)

Only when no usable dump exists: a snapshot rolls back **every** database of `tournesol-db`, the other
app's included, to the snapshot's day. Say so to the user first.

```bash
$fly volumes list -a tournesol-db                    # note the id of the current volume
$fly volumes snapshots list <volume id>              # pick the snapshot
$fly postgres create --name <new cluster name> --org personal --region yyz --initial-cluster-size 1 \
    --vm-size shared-cpu-1x --vm-memory 256 --snapshot-id <snapshot id>   # output to a private file
```

A second volume needs a payment method (see "What it costs"), so this step is the user's decision.
Point the apps at the new cluster by rerunning the provisioning with its name, check them
(`just fly-smoke`), and destroy the old cluster only after checking, with the other app's owner.
Keep a snapshot's data private: it holds catalogue rows (ADR 0024 step 7).

## Production data

Production starts like staging: its first boot imports the catalogue and the campaign `seeds/` (the
character, proficiencies, bard skills, weapons, items, spells). Nothing of the operator's local play is
copied ([ADR 0024](adr/0024-deployment.md), "Amendment (production starts from the seeds,
2026-10-16)"). Character-sheet edits made in the app that the seeds do not hold are re-entered by hand
in the production app, opened with `just fly-open critical-prod` (user steps left, below).

To see what a local database holds beyond what a fresh boot seeds, without printing any of it, compare
it with a freshly seeded throwaway database (names and counts only):

```bash
scripts/db-compare.sh "<local dsn>" "<fresh dsn>"   # per table: row counts, "same" or the differing columns
```

## A new catalogue version reaches production

The catalogue is in the image, fetched at build time, and applied at boot by the importer. So:
update `critical-rulebooks`, then run the main build again (merge anything, or re-run the release
workflow's last run in GitHub Actions: it rebuilds that `main` commit with the rulebooks' current default
branch and pushes the same tag again, now a new digest with the new `critical.rulebooks.revision`), wait
for the staging smoke check, then tag a version as above.
