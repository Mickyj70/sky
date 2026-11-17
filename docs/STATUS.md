# Current project status

Updated: 2026-11-17. Current focus: **Architecture fixes before M1 polling, issue #7**. Remaining owner settings stay tracked separately in #4.

## Repository and progress

- Public repository: [Mickyj70/sky](https://github.com/Mickyj70/sky); source license: MIT.
- [Milestones M0–M5](https://github.com/Mickyj70/sky/milestones) track the delivery roadmap.
- [Issues](https://github.com/Mickyj70/sky/issues) track implementation, acceptance criteria, dependencies and rollback notes.
- Contributor tasks: help examples #10 merged in PR #13; Windows guidance #11 is under review in PR #15.
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
| Main branch protection | Active, verified 2026-11-17 | `protected: true`; detailed rules remain unreadable to the integration (HTTP 403) |
| Private vulnerability reporting | Enabled, verified 2026-11-17 | Owner-configured; private reporting route in SECURITY |
| Projects board / merge settings | Optional owner preferences | Merged branches auto-delete; all merge methods remain allowed; Projects access is unavailable to integration |
| Output contract and additional fixtures | Merged | [#5](https://github.com/Mickyj70/sky/issues/5); see CLI_CONTRACT.md |
| Persistent watchlists and recovery | Merged in PR #16 | #6; see WATCHLISTS.md and ADR 0002 |
| Architecture fixes | Implemented on `fix/architecture-redirects-and-read-locks`; review pending | Redirect policy and nonblocking read validation; see ARCHITECTURE.md |
| Polling scheduler and tracking output | Not started | #7–#9; depend on acceptance of watchlist storage |
| History, commercial flight data, releases | Not started | M2–M5 |

## Verification

- Issue #6: local race tests, vet and a CGO-disabled build passed; the real binary completed add/duplicate/list/backup/remove/restore/list across separate processes.
- CGO-disabled cross-compilation passed for macOS arm64/amd64 and Windows amd64; native Linux amd64 build and tests passed. Cross-compilation does not establish runtime validation on those other systems.
- Storage tests cover persistence, concurrent duplicate writers, failed/canceled writes, unrelated/newer schema refusal, WAL-aware snapshots, non-overwrite recovery and competing restores. Tests use temporary local databases and no provider calls.

- Issue #5 branch: local race tests, vet, build, Markdown links and desired protection JSON checks pass. New provider fixtures and subprocess/binary checks require no live API calls. Hosted checks run on [PR #14](https://github.com/Mickyj70/sky/pull/14).

- Local `go test -race ./...`, `go vet ./...`, and binary build passed after changing to the public module path.
- GitHub CI independently passed formatting, race tests, vet, build, and help smoke checks on both supported Go tracks.
- Prototype recovery was demonstrated in a separate detached worktree without altering main.
- Live provider smoke checks succeeded on 2026-11-15; provider calls are not required by CI and coverage is not guaranteed.

## Next work

1. Owner verifies the exact active PR/check requirements against `.github/branch-protection.json` to finish #4. Branch protection and private reporting are now active; no broader credential needs to be pasted into chat.
2. Review the redirect/read-lock architecture fixes after checks pass. Issue #6 merged in PR #16; issue #5 merged in PR #14.
3. Implement scheduler #7 after the architecture fixes are accepted, then watch output #8 and release validation #9.
4. Review Windows contributor documentation in PR #15 separately; #10 is complete.

## Open technical decisions

- SQLite driver: modernc.org/sqlite v1.45.0 (pure Go, Go 1.24 compatible), accepted with #6 and documented in ADR 0002.
- Commercial provider for scheduled flights: evaluate in M3; no paid provider is required for today's commands.
- Alpha JSON/exit-code contract: #5 merged; watchlist operations extend it through WATCHLISTS.md.

## Update protocol

Update this file in each progress PR with completed work, validation, next issue, and blockers. GitHub issues are the source of truth for active task status; BACKLOG retains the issue index and later work packages.
