# Current project status

Updated: 2026-11-21. Current focus: **M4 native platform verification (#31)**.

## What is complete

| Work | Evidence |
| --- | --- |
| Public MIT-licensed Go repository, contribution workflow, CI and rollback checkpoints | Core foundation issue #4 closed; branch protection and private reporting verified active 2026-11-20 |
| Live plane/callsign/nearby/regional airline lookups, JSON tables, typed errors and cancellation | #5 / PR #14; help examples #10 / PR #13 |
| Saved SQLite watchlists, platform paths, idempotent mutations, snapshot backup and safe restore | #6 / PR #16 |
| Redirect rejection and read validation without unnecessary writer-lock contention | PR #17 |
| Live watch command, observation states, text/NDJSON events and clean shutdown | #8 / PR #19, merged 2026-11-20 |
| M1 clean-source and schema-1 recovery validation, source candidate | #9 / PR #20, merged 2026-11-20; `v0.2.0-rc.1` |
| Opt-in snapshot recording, durable dedupe/retention and independent history recovery | #21 / PR #23, merged 2026-11-20 |
| Offline queries, paginated JSON/CSV export and preview/apply pruning | #22 / PR #25, merged 2026-11-21 |
| M2 native cross-version recovery validation and source candidate | #24 / PR #26, merged 2026-11-21; `v0.3.0-rc.1` |
| Bounded polling engine with injectable clock/provider, deduplication, cooldowns and backoff | #7 / PR #18, merged 2026-11-19 |

Merged baseline: `fdc4132` (PR #29). M1 release validation is complete; source candidate `v0.2.0-rc.1` is published. Stable releases and prebuilt binaries remain future work.

## Current change

PR #30 merged the offline dated-flight model. Commercial adoption remains deferred under the $0 budget.

Issue #31 adds native Go 1.24 tests and CGO-free binary smoke checks on Linux amd64, macOS arm64 and Windows amd64. The binary checks exercise SQLite recovery through temporary paths with spaces.

See [history guide](HISTORY.md), [history decision](decisions/0003-observation-history.md), [watch contract](WATCH_RUN.md), and [architecture](ARCHITECTURE.md).

## What remains, in order

1. Review and merge the native CI change for #31. Versioned binaries and checksums follow.
3. Continue M2 candidate feedback; stable 0.3.0 is a separate final-release decision.
4. Commercial M3 adapter stays deferred until an explicit budget change, verified rights/plan and authorized coverage samples.
5. M4: tested platform binaries/checksums, build-time versioning, install/upgrade instructions and optional interactive terminal UI.
6. M5: stable 1.0 contracts, compatibility/deprecation policy, operational guidance and demonstrated recovery.

Parallel work: Windows PowerShell contributor instructions are still open in issue #11 / PR #15. This does not block the watch implementation.

## Operational limits

- The watch request budget is per foreground run, not a shared account/IP daily quota. Restarts, other processes and unrelated commands can spend additional requests.
- Targets are loaded once per run. Editing the saved list requires restarting to reload it.
- Position states describe data freshness at each poll. They do not establish scheduled flight status.
- History recording is locally collected and opt-in; offline queries/exports have merged. Multi-page exports require an unchanged dataset for stable offsets. There is no daemon, external alerting or stable binary release.
- Current direct dependencies are Cobra v1.10.2 and modernc.org/sqlite v1.45.0; Go 1.24+ remains required. No new Go dependency is needed for history query/export.

## Verification and checkpoints

The implementation PR records current local/hosted check evidence. Prior milestones have race tests, vet, CGO-disabled build and local HTTP/SQLite fixtures. Cross-build evidence does not claim native runtime support on every target.

Annotated `checkpoint/*` tags preserve tested source states. They do not contain local watchlists or credentials. SQLite backups and restore compatibility remain separate from Git rollback; see RECOVERY.md. Do not overwrite shared checkpoints or rewrite shared main history.

## Progress maintenance

GitHub issues/PRs are the acceptance source of truth; update this document with each substantial change. A merged scheduler does not by itself complete the CLI/output work. Target version numbers in the roadmap are plans until an actual release is validated and published.
