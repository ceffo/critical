# CI runner

The heavy and privileged jobs run on self-hosted GitHub Actions runners (`runs-on: [self-hosted, critical]`), VM
instances on the owner's Proxmox host; the small jobs run on GitHub-hosted `ubuntu-latest`. Standard GitHub-hosted
runners are free in a public repository ([GitHub billing docs, "Actions billing"](https://docs.github.com/en/billing/concepts/product-billing/github-actions)),
so the hosted jobs cost nothing. The Actions budget did run out on 2026-11-11 (the heavy CodeQL and CI load that
day), which is why everything moved here; with only the small jobs hosted that stops mattering. If a hosted job
ever queues or fails for billing, check Settings → Billing first. [agent-workflow.md](agent-workflow.md)
("CI and PRs") describes the jobs themselves.

| Where | Jobs | Why |
|---|---|---|
| GitHub-hosted `ubuntu-latest` | `changes`, `validate`, `generated`, `pr-title`, Dependabot auto-merge | Small, parallel-friendly, no queue (about 2 s start, against minutes behind the runners); a fork's code never runs on the VM anyway |
| Homelab runners | `e2e`, `dev-smoke`, `image`, release, promote, major, main catch-up | Chromium's system packages are preinstalled (no per-run apt), docker is there, and release/promote hold the environment secrets and deploy key |

Queueing was the whole PR-latency regression when every job shared one runner (median wait 4 min, p90 15-20 min,
CRI-333); keeping the cheap jobs off it, and running several runner instances, brings it back.

```mermaid
flowchart LR
    GH["GitHub (ceffo/critical)"] -->|"small jobs"| H["ubuntu-latest (hosted)"]
    GH -->|"long poll, outbound only"| R["N runner services on the ci-runner VM"]
    R --> J["job: e2e, dev-smoke, image, release..."]
    J --> D["docker on the VM"]
```

## The machine

| | |
|---|---|
| Host | Proxmox `centauri` (`root@centauri.home.lan`), VM id 106, name `ci-runner` |
| Size | 4 vCPU, 8 GB RAM, 4 GB swapfile, 40 GB disk (`local-lvm`), starts on boot |
| OS | Ubuntu 24.04 LTS cloud image, user `ci` (key login, passwordless sudo) |
| Runner | `~/actions-runner`, systemd service `actions.runner.ceffo-critical.centauri-ci`, label `critical` |

The e2e suite takes about 5 minutes here (budget in [ADR 0048](adr/0048-e2e-runs-on-ui-and-api-paths.md)).
One runner service runs one job at a time; GitHub gives a queued job to any idle runner with the label, so
`centauri-ci`, `centauri-ci-2` and `centauri-ci-3` (below) run three jobs at once on the VM. Jobs on the VM are
safe side by side: e2e and dev-smoke use free ports and throwaway compose projects, the image job tags and removes
`critical:ci-<run id>-<attempt>`, and the Playwright browser download is serialized with `flock` on
`~/.cache/ms-playwright.lock`. Each runner has its own `_work` directory; Go and npm caches under `$HOME` are
safe to share. If e2e slows down with three jobs at once, give the VM more cores (`qm set 106 --cores 8 --memory 12288`, reboot).

## Fork pull requests never run here

The repository is public, so anyone can open a pull request, and a job runs the PR's code on this machine.
Two guards, both required:

1. Repository setting **Settings → Actions → General → Fork pull request workflows from outside
   collaborators → Require approval for all outside collaborators**.
2. The jobs started by `pull_request` (`ci.yml`, `pr-title.yml`) carry
   `github.event.pull_request.head.repo.full_name == github.repository`; a fork's run skips them. Release,
   promote, major and the catch-up start only from this repository's own events (`push`, `workflow_run`,
   `workflow_dispatch`, `schedule`); the Dependabot job (`pull_request_target`) checks out nothing from the PR.

The runner is persistent: a job leaves files and Docker images behind, and the release jobs run on it with
the staging and production environment secrets. Keep the VM single-purpose and never register it on another
repository.

## Rebuilding it

1. **VM** (on `centauri`): download `noble-server-cloudimg-amd64.img` to `/var/lib/vz/template/iso/`;
   `qm create 106 --name ci-runner --memory 8192 --cores 4 --cpu host --net0 virtio,bridge=vmbr0
   --scsihw virtio-scsi-pci --agent 1 --onboot 1 --ostype l26`; `qm importdisk 106 noble.img local-lvm`;
   attach it as `scsi0` (discard, ssd), add the cloud-init drive (`--ide2 local-lvm:cloudinit --ciuser ci
   --sshkeys <public key file> --ipconfig0 ip=dhcp`), boot order `scsi0`, `qm resize 106 scsi0 40G`,
   `qm start 106`. Give the VM a DHCP reservation (nothing depends on its address). Add a 4 GB `/swapfile`
   (and its `fstab` line).
2. **Tools** (as `ci`, `sudo` for the installs):
   - apt: `ca-certificates curl git jq build-essential postgresql-client xvfb docker.io docker-buildx docker-compose-v2 unzip
     gh shellcheck python3`; `sudo usermod -aG docker ci`.
   - Go: the version in `api/go.mod`, from `https://go.dev/dl/`, unpacked in `/usr/local/go`.
   - Node: the current LTS (NodeSource `setup_lts.x`, then `apt install nodejs`).
   - just: the release in `.github/actions/setup-just/action.yml` (binary into `/usr/local/bin`).
   - Playwright: in a scratch directory, `sudo npx playwright@<version in web/package-lock.json> install-deps
     chromium`, then `npx playwright@<version> install chromium` as `ci` (browsers land in
     `~/.cache/ms-playwright`). When Playwright's version moves, the e2e job downloads the new browser by
     itself; only a change of its system packages needs `install-deps` again.
     (`scripts/ci-install-chromium-deps.sh` is the retrying installer the hosted runners needed; CI no longer calls it.)
3. **Runner**: download the latest `actions-runner-linux-x64` release from
   `https://github.com/actions/runner/releases` into `~/actions-runner`, then in the repository's
   **Settings → Actions → Runners → New self-hosted runner** copy the registration token (it expires in an
   hour; never put it in a file, a commit or an issue) and run
   `./config.sh --unattended --url https://github.com/ceffo/critical --token <token> --name centauri-ci
   --labels critical --replace`. Put `PATH=/usr/local/go/bin:/home/ci/go/bin:` + the default path in
   `~/actions-runner/.env` (the service does not read the shell profile), then `sudo ./svc.sh install ci`
   and `sudo ./svc.sh start`.
4. **Check**: the runner shows Idle in the repository's runner list; reboot the VM and see it come back;
   open a pull request and see CI run on it. To check the machine alone, copy the repository to it and run
   `just validate`, `just e2e` and `just dev-smoke`.

### More runner instances (parallel jobs)

Needs the user: a registration token per runner (Settings → Actions → Runners → New self-hosted runner; it expires
in an hour; never put it in a file, a commit or an issue). On the VM, as `ci`, for `n` in 2 and 3, with a fresh
token each:

```bash
n=2
mkdir ~/actions-runner-$n && tar -C ~/actions-runner-$n -xzf ~/actions-runner/actions-runner-linux-x64-*.tar.gz
# or download the same release tarball again if the archive is gone
cp ~/actions-runner/.env ~/actions-runner-$n/.env
cd ~/actions-runner-$n
./config.sh --unattended --url https://github.com/ceffo/critical --token <token> \
  --name centauri-ci-$n --labels critical --replace
sudo ./svc.sh install ci && sudo ./svc.sh start
```

Check: the repository's runner list shows every `centauri-ci*` Idle; reboot the VM and they all come back.
Remove one like the first runner (its own directory, `./config.sh remove`).

Removing the runner: **Settings → Actions → Runners → Remove** (or `./config.sh remove --token <token>`), then
destroy the VM. To move a job to GitHub-hosted runners, switch its `runs-on` line to `ubuntu-latest` (for e2e, also restore the Chromium apt steps of the e2e job (git history of `ci.yml`)).
