# Deploy runbook (fly.io)

Operator commands for [ADR 0024](adr/0024-deployment.md). Configuration lives in `deploy/fly.staging.toml`
and `deploy/fly.prod.toml` (checked by `just check-fly`, and by `just validate` through the Go test in
`api/internal/flycheck`). The image is built by the release workflow and passed with `--image`; the
config never names it. Steps marked **(user)** need fly credentials or GitHub admin rights; an agent
never runs them. Nothing here is public: the apps have no public IP (ADR 0024 step 8).

Defaults (ADR 0024, "Proposed defaults"): apps `critical-staging` and `critical-prod`, region `yyz`
(Toronto: fly has no Montreal region; user, 2026-10-16), one machine and one 1 GB volume per app, `shared-cpu-1x` with 512 MB. App names are global on fly. To
rename an app or change the region, edit `app` / `primary_region` in the two toml files and the
`FLY_APP` values below; the Go test checks the app names against the file names.

```bash
fly=~/.fly/bin/fly   # flyctl on the operator's machine
```

## One-time setup per app (user)

Repeat for `staging` and `prod` (`<env>` = `staging` or `prod`, app = `critical-<env>`).

```bash
$fly auth login
$fly apps create critical-<env> --org personal
$fly volumes create data -a critical-<env> --region yyz --size 1 --snapshot-retention 7   # 30 for prod
$fly tokens create deploy -a critical-<env>        # app-scoped token: store it in GitHub, nowhere else
```

The volume must exist before the first deploy, and its name is `data` (the `[[mounts]]` source).

First deploy of an app, with a private Flycast address only (no public IP):

```bash
$fly deploy -c deploy/fly.<env>.toml -a critical-<env> --image <registry image ref> --no-public-ips
$fly ips allocate-v6 --private -a critical-<env>
$fly ips list -a critical-<env>      # must show no public (v4 / v6) address
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
2. A tag `vMAJOR.MINOR.PATCH` on a `main` commit that passed staging snapshots the production volume,
   deploys the **same image** to production, smoke-checks it and creates the GitHub Release.

```bash
git switch main && git pull
git tag v1.2.3 <sha that passed staging> && git push origin v1.2.3    # user: the deliberate act
```

By hand, the equivalent of a deploy is
`$fly deploy -c deploy/fly.<env>.toml -a critical-<env> --image <ref>` (immediate strategy, a few
seconds of downtime: one machine owns the volume).

## Rollback

Run the promotion again for the previous tag (re-run its workflow in GitHub Actions, user, or by hand):

```bash
$fly deploy -c deploy/fly.prod.toml -a critical-prod --image <image ref of the previous tag>
curl -s localhost:8080/api/health     # through fly proxy: the commit must be the previous one
```

**Across a migration** the older binary refuses to start on the newer schema (ADR 0024 step 6.2).
Redeploy the older tag **and** restore the pre-promotion snapshot (next section). Data written
since that snapshot is lost: say so to the user before restoring.

## Restore from a snapshot (user)

```bash
$fly volumes list -a critical-prod                     # note the id of the current volume
$fly volumes snapshots list <volume id>                # daily snapshots and the pre-promotion one
$fly volumes create data -a critical-prod --region yyz --snapshot-id <snapshot id> --snapshot-retention 30
$fly machine list -a critical-prod                     # note the machine id
$fly machine stop <machine id> -a critical-prod         # nothing writes the old volume from here
$fly machine clone <machine id> -a critical-prod --attach-volume <new volume id>:/data   # same image and config
$fly machine destroy <machine id> -a critical-prod --force   # the old machine
$fly machine list -a critical-prod                     # the clone runs; check it is the only machine
curl -s localhost:8080/api/health                      # through fly proxy
$fly volumes destroy <old volume id> -a critical-prod  # only after checking the app: irreversible
```

Keep the snapshot's data private: it holds catalogue rows (ADR 0024 step 7).

## A new catalogue version reaches production

The catalogue is in the image, fetched at build time, and applied at boot by the importer. So:
update `critical-rulebooks`, then run the main build again (merge anything, or re-run the release
workflow for the current `main` commit in GitHub Actions), wait for the staging smoke check, then tag
a version as above.
