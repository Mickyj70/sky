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

## Remaining owner settings

The integration returned HTTP 403 for repository administration. Main is currently unprotected and private vulnerability reporting is disabled; these are tracked in [#4](https://github.com/Mickyj70/sky/issues/4).

1. In repository Settings, add a main-branch rule requiring PRs and successful `Go 1.24.x checks` / `Go stable checks`. Block force pushes and branch deletion; avoid requiring a separate approver while there is only one maintainer.
2. In Security settings, enable private vulnerability reporting. Until then, SECURITY's fallback applies.
3. Optionally allow squash merges only and auto-delete merged branches; a Projects board can also be added by the owner.

Successful CI is verified at [run 43747116584](https://github.com/Mickyj70/sky/actions/runs/43747116584). These checks are not yet enforced as branch requirements.

## Applying the checked-in main protection configuration

The apply request was retried on 2026-11-16 and still returned HTTP 403. The current integration cannot perform this action; main remains unprotected. An owner using their own administrator-authorized GitHub CLI can run from a checkout containing this change:

```sh
gh api --method PUT repos/Mickyj70/sky/branches/main/protection \
  --input .github/branch-protection.json

gh api repos/Mickyj70/sky/branches/main --jq '{name, protected}'
```

The desired configuration requires a PR, both named Go CI checks, an up-to-date branch and resolved review conversations. It enforces rules for administrators, blocks force pushes/deletion, and requires linear history. Required independent approvals are zero so a solo maintainer is not locked out. Existing PRs can be updated and squash-merged when checks pass. The JSON file is a configuration proposal until the API/UI confirms protection is active; committing it does not enable protection.
