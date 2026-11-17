# GitHub project workflow

Repository: [Mickyj70/sky](https://github.com/Mickyj70/sky), public under the MIT code license. The owner created the empty repository because the integration cannot create repositories; repository-level setup can use the existing destination.

GitHub hosts source, issues, checks, and releases. It does not run the tracking process continuously or back up local SQLite data.

## Progress tracking

- [Issues](https://github.com/Mickyj70/sky/issues) track scoped work and acceptance criteria.
- [Milestones](https://github.com/Mickyj70/sky/milestones) follow M0–M5 in ROADMAP.
- Use status labels: Backlog, Ready, In progress, and In review. Closed issues are Done.
- Use type and priority labels to find the kind of work and the current critical path.
- The integration cannot access GitHub Projects. A visual project board can be added when Projects access is available; milestones and labels remain usable now.
- Pick a `good first issue` for a small contribution. Keep one core implementation issue In progress initially.

## Changes and checks

Fork the repository and open focused PRs against main. The offline CI workflow uses read-only repository permissions and no provider secrets. Choose actual successful CI check names when configuring branch protection. For solo maintenance, require PRs/checks without requiring an impossible independent approval.

Link the issue in each PR, record acceptance evidence and rollback instructions, and update STATUS and CHANGELOG when appropriate. Prefer squash merges so a feature can be reverted as one coherent commit. Do not force-push shared main or move published checkpoint tags.

At a milestone boundary, verify recovery, create an annotated checkpoint, and push it. Use Releases only for actual downloadable versions once distribution is implemented. A milestone target is not automatically a release.

## Licensing and reporting

Code uses MIT; provider data terms remain separate. Follow CONTRIBUTING, CODE_OF_CONDUCT, and SECURITY. Keep keys, local data, and build artifacts out of Git. Use private vulnerability reporting rather than public exploit details.

## Setup verification

STATUS records the current hosted CI and branch protection results. A committed workflow definition alone does not prove that GitHub ran it. Future changes to repository settings must preserve fork contributions and offline checks.

## Verified owner settings

On 2026-11-17 the public branch metadata reports `main` as protected, and private vulnerability reporting reports `enabled: true`. Automatic deletion of merged branches is also enabled. The owner applied these settings; the integration did not enable them.

The integration still receives HTTP 403 when reading detailed branch-protection settings. The owner should verify the exact required checks/PR rules against `.github/branch-protection.json` before closing the remaining checklist in [#4](https://github.com/Mickyj70/sky/issues/4). All three merge methods currently remain available; squash-only merging and a Projects board are optional preferences.

## Applying the checked-in main protection configuration

The integration's 2026-11-16 apply request returned HTTP 403. Protection was subsequently enabled by the owner. For future configuration changes, an administrator can compare the active settings and deliberately apply the checked-in configuration:

```sh
gh api --method PUT repos/Mickyj70/sky/branches/main/protection \
  --input .github/branch-protection.json

gh api repos/Mickyj70/sky/branches/main --jq '{name, protected}'
```

The desired configuration requires a PR, both named Go CI checks, an up-to-date branch and resolved review conversations. It enforces rules for administrators, blocks force pushes/deletion, and requires linear history. Required independent approvals are zero so a solo maintainer is not locked out. Existing PRs can be updated and squash-merged when checks pass. The active detailed settings are not readable by this integration; the JSON file describes the desired configuration and does not automatically apply or verify it.
