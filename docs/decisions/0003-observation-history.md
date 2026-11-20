# ADR 0003: Separate, opt-in local observation history

Date: 2026-11-20
Status: Proposed; accepted on merge of issue #21.

## Context

M1 watchlists use schema 1 in `watchlist.db`, with a tested older-binary recovery path. M2 needs higher-volume observations, retention and eventually queries/exports. Adding tables to that schema would make older watchlist binaries refuse the entire database and complicate target recovery.

## Decision

- Create `history.db` alongside `watchlist.db`, with independent application ID `0x534b5948` (SKYH) and schema version 1. Reuse the pinned modernc SQLite driver; no new dependencies.
- Require `watch run --record`. Ordinary watch runs do not create or touch history. Direct plane/callsign/nearby/airline commands do not record.
- Initialize an empty history database in one immediate transaction. Validate existing identities/columns under a deferred read transaction. Refuse unrelated or unsupported schemas without rewriting them. No existing database is migrated by this change.
- Future changes to either database require an ADR, versioned transactional migration, an automatically created and verified pre-migration snapshot, and tested failure/restore/downgrade paths **before** that migration ships. Never silently upgrade on opening an unsupported schema. Separate files reduce the scope of migrations; they do not remove these requirements.
- Save successful lookup observations before display deduplication, including empty results. Store target, source, UTC local evaluation/retrieval times, provider snapshot time, state and known aircraft fields as a versioned JSON payload. Do not store provider errors, credentials or HTTP headers. A successful response is not evidence of authoritative flight status.
- Identify a response with SHA-256 of canonical target plus source, snapshot time and sorted aircraft payload. Ignore local evaluation/retrieval times when snapshot time exists; retain the first local encounter and its state. Without snapshot time, retrieval time participates in identity. Identical positions from different provider snapshots remain separate observations; distinct registration/hex targets stay separate. Aircraft sampling ages remain part of snapshot content.
- Within one transaction, delete rows strictly older than the encounter time minus retention, then insert the current response with conflict-ignore. Default retention is 720 hours (30 days), configurable per run. Invalid input or failed insert rolls back pruning too. Retention uses local encounter time, not provider snapshot time, and runs only on successful recorded lookups. Older compatible backups may reintroduce data until recording prunes it again.
- Keep SQLite's rollback journal and one connection per store; no write transaction spans provider requests or sleep. Bounded five-second lock waits and cancellation apply. Disk/permission/write errors stop the watch run with a diagnostic and failing exit rather than silently dropping history.
- Retention reuses SQLite pages; it does not guarantee a hard disk cap or shrink the file. Status exposes row count and allocated main-file bytes. Monotonic wall-clock progression is assumed; moving the system clock backwards can defer pruning. Native timestamps use signed Unix nanoseconds and reject out-of-range dates.
- History snapshots use SQLite `VACUUM INTO`, verification and file sync. Restore reads only validated rows into a fresh schema and publishes by a non-overwriting hard link. Extra source schema/triggers are not imported. Watchlist and history backups are independent, not a single cross-database transaction.

## Consequences and rollback

M1 binaries continue reading `watchlist.db` unchanged and ignore history. Downgrading preserves history on disk but cannot query it; keep a compatible newer binary and separate history snapshot. Git checkpoints do not back up either data file. A history restore does not restore saved targets, and watchlist restore does not restore history.

Recording is foreground-only and makes no additional provider requests. Observation storage does not provide provider historical data, continuous coverage, commercial flight status or a historical fleet dataset. Database contents retain the provider's separate data licensing obligations; the code's MIT license does not relicense collected observations. Do not commit collected histories into the source repository. Query/export/explicit offline pruning and migrations remain separate M2 work.
