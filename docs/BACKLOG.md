# Initial issue backlog

M0–M1 tickets now exist on GitHub; their linked issues are the source of truth for acceptance criteria and status. The sections below preserve the original plan. Effort is relative: S/M/L.

| Local ID | GitHub issue |
| --- | --- |
| M0-01 | [#4 — Public setup and remaining owner settings](https://github.com/Mickyj70/sky/issues/4) |
| M0-02 | [#5 — CLI/provider contracts](https://github.com/Mickyj70/sky/issues/5) |
| M0-03 | [#10 — Command help examples; good first issue](https://github.com/Mickyj70/sky/issues/10) |
| M0-04 | [#11 — PowerShell contributor guide; good first issue](https://github.com/Mickyj70/sky/issues/11) |
| M1-01 | [#6 — Storage and watchlists](https://github.com/Mickyj70/sky/issues/6) |
| M1-02 | [#7 — Polling scheduler](https://github.com/Mickyj70/sky/issues/7) |
| M1-03 | [#8 — Watch output and observation state](https://github.com/Mickyj70/sky/issues/8) |
| M1-04 | [#9 — Saved tracking release validation](https://github.com/Mickyj70/sky/issues/9) |

## M0 — Foundation

### M0-01 — Publish repository and activate progress tracking (S)

- Create the chosen GitHub repo and push `main` plus both checkpoint tags.
- Create M0–M5 milestones and labels: `type:feature`, `type:bug`, `type:docs`, `type:chore`, `priority:p0`, `priority:p1`, `priority:p2`, `blocked`, `needs-decision`.
- Use status labels for Backlog, Ready, In progress, and In review. Create a project board with those columns plus Done when Projects permissions are available.
- Create the M0–M1 issues in this document and assign milestones.
- Confirm CI passes remotely; configure PR checks and block force pushes where supported by account policy.
- Done when the remote URL, milestone/progress links, and first green run are recorded in STATUS.
- Rollback: remove a mistaken remote configuration locally; keep commits/tags. Do not delete the repository as a routine rollback.

### M0-02 — Stabilize existing CLI/provider contracts (M)

- Document JSON field types, empty results, timestamps, units, and exit codes; decide whether names remain provider-specific during the alpha.
- Add local HTTP fixtures for all query paths, invalid/missing schema, oversized responses, cancellation, numeric/ground/unknown altitude, and empty observations.
- Add end-to-end subprocess coverage for exit behavior and clean stdout JSON/stderr errors.
- Define error classes before a polling scheduler depends on rate-limit versus transient-error detection.
- Done when contract examples and tests agree, and baseline commands still work.
- Rollback: revert the PR; no data migration.

## M1 — Saved tracking

### M1-01 — Choose storage and persist watchlists (M)

- Write an ADR selecting a SQLite driver with supported OS/architecture and CGO tradeoffs.
- Define config/data directory paths, optional overrides, typed identifiers, uniqueness rules, and a versioned initial schema.
- Implement add/list/remove and a backup/restore path; preserve secrets outside the database.
- Test restart persistence, duplicates, path errors, transactions, and backup recovery using temporary directories.
- Done when the watchlist commands work offline and persistence is proven.
- Depends on M0-02. Rollback: use previous binary and restore a compatible pre-change backup if required.

### M1-02 — Build quota-aware polling scheduler (L)

- Introduce injectable clock and provider interfaces; deduplicate saved targets.
- Estimate requests/cycle, require a bounded request budget, and expose remaining allowance.
- Respect Retry-After seconds/date, bounded backoff, cancellation, and provider failures.
- Test with fake time; never use live API calls in CI.
- Done when tests prove upper request bounds and correct cooldown/stop behavior.
- Depends on M1-01. Rollback: revert scheduler PR; saved targets remain available.

### M1-03 — Add watch output and observation state (M)

- Implement `watch run` plain text and JSON events on top of the scheduler.
- Define freshness thresholds, repeat-event suppression, and unknown/stale/observed transitions.
- Never infer landed solely from missing observations.
- Done when stale/empty/outage fixtures produce documented events and Ctrl-C exits cleanly.
- Depends on M1-02. Rollback: revert output PR; no watchlist schema change without a separate migration.

### M1-04 — Validate and release saved tracking (S)

- Walk through add/list/run/remove using the README and record validation.
- Exercise backup/restore and downgrade instructions; review quota behavior and changelog.
- Complete M1 acceptance criteria and tag the tested 0.2.0 release candidate.
- Depends on M1-03. Rollback: select the prior tested binary/checkpoint and compatible data backup.

## Later milestones (split into smaller issues when ready)

| ID | Milestone | Work package | Acceptance evidence |
| --- | --- | --- | --- |
| M2-01 | M2 | Observation storage, dedupe, retention | Replay, disk retention, migration and recovery tests |
| M2-02 | M2 | History queries and CSV/JSON export | Offline fixture queries and export comparison |
| M3-01 | M3 | Commercial provider evaluation and budget decision | Recorded feature/coverage/cost/rights comparison with official sources |
| M3-02 | M3 | Dated flights and airline code resolution | Date reuse, codeshare and missing-field fixtures |
| M4-01 | M4 | Versioned platform builds and checksums | Build matrix and install smoke evidence |
| M4-02 | M4 | Optional interactive watch screen | Shared scheduler tests and terminal usability checks |
| M5-01 | M5 | Compatibility and 1.0 release review | All roadmap acceptance and recovery evidence linked |

## Prioritization

Use priority p0 for broken core workflows, data loss, leaked credentials, or uncontrolled spending. Use p1 for the current milestone's critical path; p2 for nonblocking improvements. Keep at most one implementation issue in progress initially. Reassess scope before pulling later work forward.
