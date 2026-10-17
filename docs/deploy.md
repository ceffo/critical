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
to 60 s per new connection) instead of failing (`docs/backend.md`, "Database"). `fly ssh` does not start it:

```bash
$fly status -a tournesol-db                          # machine state; role "error" only means stopped
$fly machine start <machine id> -a tournesol-db      # before fly ssh / fly postgres connect
$fly ips list -a tournesol-db                        # private ingress (Flycast) only, no public v4/v6
```

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
current users keep their access.

**A user the other app gets later** (a `CREATE ROLE`, a `fly postgres attach` of a new app) has no
CONNECT on the locked databases, and a rerun does not add it (the databases are already locked). Grant
it by hand: `$fly postgres connect -a tournesol-db`, then `GRANT CONNECT ON DATABASE <db> TO <user>;`.
A database created later keeps PUBLIC CONNECT until the next run of the provisioning locks it.

**Passwords.** A user gets a generated password when it is created, sent to the server only as a
SCRAM-SHA-256 verifier, and its app's secret
`CRITICAL_DSN=postgres://critical_<env>:…@tournesol-db.flycast:5432/critical_<env>?sslmode=prefer` is
piped into `fly secrets import` right after (never printed): staged while the app has no machine (the
first `fly deploy` applies it), applied at once otherwise. An existing user's password and secret are
never touched by a rerun, so a rerun on a running app changes nothing it uses. `fly secrets list -a
critical-<env>` shows the name and a digest, never the value.

**Checks**, every run: each critical user holds CONNECT on its own database and on no other (privileges
read as the cluster's operator). When the run set a password it also connects through `fly proxy` to
`tournesol-db.flycast`: own database usable (may create in its schema), every other connection refused
(`permission denied for database`). Then it sets the volume's snapshot retention.

**Rotation** (a leaked password, or the one-off seed below): `just fly-db-provision --rotate
tournesol-db <env>`. It sets a new password, then applies the new secret, so **the app is briefly out**:
from the password change until fly has restarted each of its machines with the new secret, new
database connections fail (connections already open keep working). Before the app's first deploy it has
no machine and the rotation costs nothing. `--password-dir ~/.config/critical` also writes the new
password to `~/.config/critical/critical_<env>.password` (mode 600) for the operator; delete it after
use. If a run stops between the password change and the secret, rerun it with `--rotate`.

### Reaching a database

```bash
$fly postgres connect -a tournesol-db -d critical_prod     # psql as the cluster's operator (superuser)
$fly proxy 15432:5432 tournesol-db.flycast -a tournesol-db # then psql "host=127.0.0.1 port=15432 user=critical_prod dbname=critical_prod"
```

The second form needs the user's password, which is only in the app's secret. To look at the data,
use `fly postgres connect` (`SET ROLE critical_prod;` to act as that user); never rotate a running
app's password just to log in: the rotation is an outage. Production data is
private (ADR 0024 step 7): never paste query output into an issue, a PR or a log.

### Snapshots

fly takes a daily snapshot of the cluster volume, kept 30 days (ADR 0024 step 9.1, set by the
provisioning). A snapshot holds every database of the cluster, the other app's too: restoring one rolls
them all back, so a restore of `critical_prod` uses a `pg_dump` first (step 9.3).

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
$fly deploy -c deploy/fly.prod.toml -a critical-prod --image <registry image ref> --no-public-ips          # two machines
$fly deploy -c deploy/fly.staging.toml -a critical-staging --image <registry image ref> --no-public-ips --ha=false   # one machine
$fly ips allocate-v6 --private -a critical-<env>
just fly-private critical-<env>                    # fails on any public address
$fly machine list -a critical-<env>                # prod: 2 machines, staging: 1
```

Later deploys come from the workflows. If `fly ips list` shows a public IP, release it with
`$fly ips release <address> -a critical-<env>` unless a public-access option has been chosen (ADR 0024
step 8.4).

### GitHub (user)

In the repository settings, create the environments `staging` and `production`:

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
$fly proxy 8080:8080 -a critical-<env>      # then http://localhost:8080 (leave it running)
curl -s localhost:8080/api/health           # {"status":"ok","commit":"<sha>"}
$fly ssh console -a critical-<env>          # a shell on the machine
$fly logs -a critical-<env>
```

For a device that must reach the app without `fly proxy`, join the org's WireGuard network
(`$fly wireguard create`, user) and use the app's `.flycast` address. A public IP, basic auth and a
custom domain come only once the user picks a public-access option (ADR 0024 step 8.4); this runbook
gets those steps then.

## How a release flows

ADR 0024 steps 3 and 4, driven by GitHub Actions (the workflows are separate items):

1. A merge to `main` with green CI builds the image with the real catalogue, tags it with the commit
   sha, deploys it to staging and smoke-checks `/api/health` for that commit.
2. A tag `vMAJOR.MINOR.PATCH` on a `main` commit that passed staging takes a `pg_dump` of
   `critical_prod` (no dump, no deploy), deploys the **same image** to production, smoke-checks it and
   creates the GitHub Release.

```bash
git switch main && git pull
git tag v1.2.3 <sha that passed staging> && git push origin v1.2.3    # user: the deliberate act
```

By hand, the equivalent of a deploy is
`$fly deploy -c deploy/fly.<env>.toml -a critical-<env> --image <ref>`: a rolling deploy, one machine
at a time, so production keeps serving from the other (ADR 0024 step 5.2). A release's migrations are
additive, so the previous release keeps working on the new schema while its machines are replaced.

## Rollback

Run the promotion again for the previous tag (re-run its workflow in GitHub Actions, user, or by hand):

```bash
$fly deploy -c deploy/fly.prod.toml -a critical-prod --image <image ref of the previous tag>
just fly-smoke critical-prod <commit of the previous tag>
```

**Across a migration** the older binary refuses to start on the newer schema (ADR 0024 step 6.2).
Redeploy the older tag **and** restore `critical_prod` from the pre-promotion dump (step 9.3). Data
written since that dump is lost: say so to the user before restoring. The dump and restore commands
come with the backup tooling (`just db-backup`, ADR 0024 step 9.4).

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

## Seed production with the SQLite data (once, user)

Production starts with the data of the developer's SQLite `.dev/critical.db`, copied by the SQLite copy
tool ([ADR 0025](adr/0025-postgresql-storage.md) step 9, `docs/backend.md` "Moving a SQLite database
into PostgreSQL"). Do it before the first deploy of `critical-prod`: the server seeds an empty database
at its first boot, and the tool refuses a database that holds any row. Staging is never seeded; it
starts empty.

```bash
# On the operator's machine, in a checkout of the release being deployed. Stop every server still
# writing the SQLite file first: what it writes after the copy is not in production.
just fly-db-provision --rotate --password-dir ~/.config/critical tournesol-db prod   # before the first deploy: no machine, no outage
$fly proxy 15432:5432 tournesol-db.flycast -a tournesol-db   # the private cluster; leave it running
PROD_DSN="postgres://critical_prod:$(cat ~/.config/critical/critical_prod.password)@localhost:15432/critical_prod?sslmode=disable"
just db-import-sqlite .dev/critical.db "$PROD_DSN"     # prints the rows of each table; counts equal or nothing is kept
```

The tool reads the SQLite file read-only, works on a copy in a private temporary directory, which it
deletes at the end, and writes only to the target. The data never goes anywhere but the operator's
machine and the cluster (ADR 0024 step 7). If it stops with an error, nothing was written: fix the cause
and run it again. Then deploy `critical-prod`; its first boot finds the baseline applied and the seeds
and catalogue already there, and writes nothing.

Then `shred -u ~/.config/critical/critical_prod.password`. The tool is deleted from the repository once production is seeded (ADR 0025 step 9.5).

## A new catalogue version reaches production

The catalogue is in the image, fetched at build time, and applied at boot by the importer. So:
update `critical-rulebooks`, then run the main build again (merge anything, or re-run the release
workflow for the current `main` commit in GitHub Actions), wait for the staging smoke check, then tag
a version as above.
