# Current project status

Updated: 2026-11-15. Current focus: **M0 — Project foundation**.

## Repository and progress

- Public repository: [Mickyj70/sky](https://github.com/Mickyj70/sky); source license: MIT.
- [Milestones M0–M5](https://github.com/Mickyj70/sky/milestones) track the delivery roadmap.
- [Issues](https://github.com/Mickyj70/sky/issues) track implementation, acceptance criteria, dependencies and rollback notes.
- [Beginner-friendly contributions](https://github.com/Mickyj70/sky/labels/good%20first%20issue): command help examples #10 and PowerShell contributor guidance #11.
- Progress uses status labels; a Projects board is optional and unavailable to the current integration.
- No stable release or prebuilt release binaries exist. `go install github.com/Mickyj70/sky/cmd/sky@main` installs development code.

## Baseline and recovery

- Prototype: `b0ff308`, tag `checkpoint/prototype-2026-11-15`.
- Planning foundation: `1fa4d1e`, tag `checkpoint/planning-2026-11-15`.
- Public contributor foundation: `612bd45` (canonical Go module, MIT license, contributor/security guidance and Dependabot).
- Public setup checkpoint: `checkpoint/open-source-2026-11-15` (created after the setup documentation is merged and checked).
- Existing prototype/planning tags are pushed to GitHub. See RECOVERY for safe code reverts and future database recovery.

## Progress

| Area | State | Evidence / remaining work |
| --- | --- | --- |
| Prototype | Done | Registration/hex/callsign lookup, nearby traffic, regional airline filtering, JSON/table output |
| Public source and licensing | Done | Public repo, canonical module path, MIT code license; provider data terms are separate |
| Contributor experience | Done | Fork/PR guidance, templates, code of conduct, security policy and two good first issues |
| Roadmap and backlog | Published | Six milestones; initial M0–M1 issues linked from BACKLOG |
| Automated checks | Hosted checks pass | [First successful CI run](https://github.com/Mickyj70/sky/actions/runs/43747116584) on Go 1.24.x and stable |
| Dependency updates | Configured | Dependabot for Go modules and GitHub Actions; generated PRs still require review |
| Main branch protection | Owner action needed | `protected: false`; integration cannot configure repository administration (HTTP 403) |
| Private vulnerability reporting | Owner action needed | Disabled; enabling it returned HTTP 403. SECURITY describes the current fallback |
| Projects board / merge settings | Owner action needed, optional | Projects and repository settings are outside current token permissions; labels work now |
| Output contract and additional fixtures | Ready | [#5](https://github.com/Mickyj70/sky/issues/5) |
| Watchlists and scheduler | Not started | #6–#9, dependent on contract hardening |
| History, commercial flight data, releases | Not started | M2–M5 |

## Verification

- Local `go test -race ./...`, `go vet ./...`, and binary build passed after changing to the public module path.
- GitHub CI independently passed formatting, race tests, vet, build, and help smoke checks on both supported Go tracks.
- Prototype recovery was demonstrated in a separate detached worktree without altering main.
- Live provider smoke checks succeeded on 2026-11-15; provider calls are not required by CI and coverage is not guaranteed.

## Next work

1. Owner completes the settings checklist in [#4](https://github.com/Mickyj70/sky/issues/4): protect main and enable private vulnerability reporting. No broader credential needs to be pasted into chat.
2. Implement [#5: CLI/provider contracts](https://github.com/Mickyj70/sky/issues/5) through a focused PR.
3. Design [#6: storage and persisted watchlists](https://github.com/Mickyj70/sky/issues/6), then work through scheduler #7, watch output #8, and release validation #9.
4. Contributors can independently pick up #10 or #11.

## Open technical decisions

- SQLite driver: select against platform and backup requirements in #6.
- Commercial provider for scheduled flights: evaluate in M3; no paid provider is required for today's commands.
- Stable JSON/exit-code contract: define in #5 before the scheduler and additional providers depend on it.

## Update protocol

Update this file in each progress PR with completed work, validation, next issue, and blockers. GitHub issues are the source of truth for active task status; BACKLOG retains the issue index and later work packages.
