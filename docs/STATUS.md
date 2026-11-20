# Current project status

Updated: 2026-11-20. Current focus: **issue #8 — live watch output and observation states**.

## What is complete

| Work | Evidence |
| --- | --- |
| Public MIT-licensed Go repository, contribution workflow, CI and rollback checkpoints | Core foundation issue #4 closed; branch protection and private reporting verified active 2026-11-20 |
| Live plane/callsign/nearby/regional airline lookups, JSON tables, typed errors and cancellation | #5 / PR #14; help examples #10 / PR #13 |
| Saved SQLite watchlists, platform paths, idempotent mutations, snapshot backup and safe restore | #6 / PR #16 |
| Redirect rejection and read validation without unnecessary writer-lock contention | PR #17 |
| Bounded polling engine with injectable clock/provider, deduplication, cooldowns and backoff | #7 / PR #18, merged 2026-11-19 |

Merged baseline at the start of this task: `efb2ad8`.

## Current change

Branch: `feat/8-watch-observation-output`.

- Connect the merged scheduler to `watch run` with a required per-run request budget, interval, staleness threshold and per-request timeout.
- Evaluate observed/stale/missing/unknown position states; never infer landed from missing coverage.
- Suppress repeated observations/errors while keeping request progress visible.
- Emit plain text or NDJSON lifecycle/observation/error/progress events.
- Stop cleanly on exhaustion, cancellation, terminal provider failure or output failure.
- Keep saved-target schema version 1 and existing live-lookup JSON behavior unchanged.

Issue #8 was closed as completed even though main lacked `watch run`. It was reopened on 2026-11-20 while implementing the missing acceptance criteria. Completion will be linked to the implementation PR rather than inferred from an earlier issue state.

See [current roadmap](ROADMAP.md), [live watch contract](WATCH_RUN.md), [storage/recovery guide](WATCHLISTS.md), and [architecture](ARCHITECTURE.md).

## What remains, in order

1. Review/merge issue #8 and verify its live-command behavior locally.
2. Complete issue #9: fresh-checkout walkthrough, backup/restore/downgrade evidence, quota/error scenarios, version/changelog alignment and a tested 0.2.0 release candidate.
3. M2: locally collected observation history, dedupe, retention, queries and CSV/JSON export; add migration/backup compatibility before schema changes.
4. M3: select a commercial flight-status provider/budget and implement dated flights, airline code resolution and codeshares. Provider rights and coverage need verification first.
5. M4: tested platform binaries/checksums, build-time versioning, install/upgrade instructions and optional interactive terminal UI.
6. M5: stable 1.0 contracts, compatibility/deprecation policy, operational guidance and demonstrated recovery.

Parallel work: Windows PowerShell contributor instructions are still open in issue #11 / PR #15. This does not block the watch implementation.

## Operational limits

- The watch request budget is per foreground run, not a shared account/IP daily quota. Restarts, other processes and unrelated commands can spend additional requests.
- Targets are loaded once per run. Editing the saved list requires restarting to reload it.
- Position states describe data freshness at each poll. They do not establish scheduled flight status.
- There is no persistent aircraft observation history, background daemon, external alerting or stable binary release yet.
- Current direct dependencies are Cobra v1.10.2 and modernc.org/sqlite v1.45.0; Go 1.24+ remains required. No new dependency is needed for live output.

## Verification and checkpoints

The implementation PR records current local/hosted check evidence. Prior milestones have race tests, vet, CGO-disabled build and local HTTP/SQLite fixtures. Cross-build evidence does not claim native runtime support on every target.

Annotated `checkpoint/*` tags preserve tested source states. They do not contain local watchlists or credentials. SQLite backups and restore compatibility remain separate from Git rollback; see RECOVERY.md. Do not overwrite shared checkpoints or rewrite shared main history.

## Progress maintenance

GitHub issues/PRs are the acceptance source of truth; update this document with each substantial change. A merged scheduler does not by itself complete the CLI/output work. Target version numbers in the roadmap are plans until an actual release is validated and published.
