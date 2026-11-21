# Current project status

Updated: 2026-11-21. Current focus: **issue #24 — M2 recovery validation and source candidate**.

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
| Bounded polling engine with injectable clock/provider, deduplication, cooldowns and backoff | #7 / PR #18, merged 2026-11-19 |

Merged baseline: `748eaf3` (PR #25). M1 release validation is complete; source candidate `v0.2.0-rc.1` is published. Stable releases and prebuilt binaries remain future work.

## Current change

Branch: `release/0.3.0-rc.1`; issue #24.

- Clean-source candidate and two earlier watchlist/history baselines, with exact source/toolchain/fixture identity in a machine-readable report.
- Native two-way history snapshots, read/write/replay compatibility, export parity, pruning and post-prune recovery.
- Verify database independence, no-overwrite guards and refusal of unsupported/corrupt recovery data.
- Prepare source-only `v0.3.0-rc.1`; publish its immutable tag only after CI and validation. No stable release or prebuilt assets.
- Schemas stay at 1 and dependencies are unchanged. No migration is introduced.

See [history guide](HISTORY.md), [history decision](decisions/0003-observation-history.md), [watch contract](WATCH_RUN.md), and [architecture](ARCHITECTURE.md).

## What remains, in order

1. Review #24's release validation and source candidate preparation; finish tested tag publication.
2. Merge the release preparation and gather M2 candidate feedback before a stable 0.3.0 decision.
3. Scope M3 provider evaluation before integration. Any future schema migration requires a verified automatic pre-migration backup, transactional migration and failure/downgrade tests before shipping.
4. M3: select a commercial flight-status provider/budget and implement dated flights, airline code resolution and codeshares. Provider rights and coverage need verification first.
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
