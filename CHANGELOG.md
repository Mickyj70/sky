# Changelog

Version numbers identify source release candidates/releases; prototype checkpoints are listed in Git history. The first source candidate is `v0.2.0-rc.1`; no stable release or prebuilt release binaries are published by this change.

## Unreleased

Further changes will be recorded here after the first candidate.

## 0.2.0-rc.1 — 2026-11-20 (source candidate)

### Added

- Go/Cobra commands for registration/ICAO hex and callsign lookup, nearby traffic and regional airline-prefix activity.
- Table/JSON lookup output, UTC snapshot/retrieval times, typed provider errors, timeouts and cancellation.
- Persistent schema-1 SQLite watchlists with normalized unique targets, platform data paths, idempotent mutations and JSON output.
- Consistent SQLite snapshot backups and validated restore into a new directory without overwriting data.
- Quota-aware foreground scheduler with finite per-run allowances, deduplication, cooldowns, bounded backoff and injectable time/provider interfaces.
- `watch run` plain text/NDJSON events, observed/stale/missing/unknown states, meaningful-change suppression and accurate stop accounting.
- Open-source contributor/security policies, architecture/roadmap/recovery documentation, checkpoints, CI and dependency update configuration.
- Clean-source release validation with real-binary persistence/recovery, fixture polling/error/cancellation coverage and schema-1 baseline compatibility checks.

### Changed

- Canonical `internal/version` identity sets both CLI version and HTTP user agent to `0.2.0-rc.1`.
- Lookup and watch-stream JSON/exit contracts are documented separately.

### Fixed

- Provider redirects are rejected without mutating caller-owned client settings.
- Existing watchlists are validated through deferred reads; only fresh initialization reserves the writer lock.
- Missing/zero readings stay distinct; invalid or oversized provider responses fail explicitly.

### Compatibility and limits

- Requires Go 1.24+; normal builds are CGO-free. Race tests need the usual supported C toolchain.
- No schema migration from the earlier schema-1 watchlist checkpoint.
- Request allowances are per foreground run, not shared provider daily quotas.
- Source candidate only; prebuilt binaries, stored observation history and stable 1.0 guarantees remain future work.
