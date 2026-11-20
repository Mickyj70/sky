# Changelog

Only actual releases receive versioned sections. The CLI currently reports 0.1.0 as a prototype identifier; no public release has been published.

## Unreleased

### Added

- Go/Cobra CLI with registration/ICAO hex lookup, callsign lookup, nearby search, and regional airline callsign filtering.
- Table and JSON output including provider snapshot/retrieval timestamps and missing-reading handling.
- API key configuration, timeout/cancellation, rate-limit errors, and local HTTP tests.
- Roadmap, initial backlog, current status, architectural decision, contribution/recovery guidance, and GitHub CI/issue/PR templates.
- Local Git checkpoints for the working prototype and planning foundation.

- MIT code license, public contribution/security policies, fork workflow, Dependabot, and canonical GitHub module path.

### Changed

- Query-based injectable provider boundary, with endpoint construction isolated from CLI commands.
- Typed provider errors and Retry-After deadlines for future bounded polling.
- Detailed exit codes and an explicit alpha JSON contract (provider field names retained).
- Invalid/oversized provider responses fail explicitly; unknown readings remain absent.
- Offline HTTP fixtures, cancellation/error-boundary coverage, subprocess stream/exit checks, and real-binary entrypoint checks.
- Checked-in desired main-branch protection configuration; applying it remains blocked by repository administration permissions.

### Added

- Offline `watch add`, `list`, and `remove` with normalized unique targets in versioned SQLite storage.
- Platform data directories, `SKY_DATA_DIR`, and `watch --data-dir` override.
- Consistent snapshot backup and validated, atomic, non-overwriting restore into a new directory.
- Persistence/restart, duplicate/concurrent-write, failure, unsupported-schema, WAL backup and recovery tests.
- SQLite driver decision and watchlist JSON/recovery documentation.
- Internal polling scheduler with deduplicated targets, per-run request budgets, provider cooldowns, bounded backoff, and cancellation. CLI integration is planned for #8.

### Fixed

- Provider lookups reject redirects without changing caller-owned HTTP client configuration.
- Opening an existing watchlist validates it through a read transaction, so readers do not unnecessarily contend for the writer lock. Fresh initialization still rechecks schema under an immediate transaction.
- Added redirect and concurrent-reader/initialization regression tests plus an architecture/dependency guide.

### Added

- Bounded `watch run` with interval, required request allowance, per-request timeout and configurable position staleness.
- Observed/stale/missing/unknown states, meaningful-change suppression and NDJSON lifecycle/observation/error/progress events.
- Cancellation and budget accounting through in-flight stops; offline state/CLI/subprocess and interrupt tests.
- Current plan reconciled with merged PRs and remaining milestones; issue #8 completion tracked against the missing command implementation.
