# ADR 0002: Local SQLite watchlists and non-overwriting recovery

Date: 2026-11-17
Status: Proposed; accepted on merge of the issue #6 implementation.

## Context

Watchlists must survive process exits, work without provider credentials, and support recovery before observation history is added. Sky targets local macOS/Linux/Windows development and currently requires Go 1.24+. A caller may save registration and hex identifiers without an online identity lookup.

## Decision

- Use `database/sql` with the pure Go `modernc.org/sqlite` driver, pinned to v1.45.0. Its module declares Go 1.24.0. The latest driver at evaluation time, v1.62.0, requires Go 1.26.0; v1.45.0 was the newest stable compatible version found from official module metadata.
- Avoid a CGO requirement for normal builds. Race testing still needs the usual Go race/C toolchain. SQLite through a CGO driver would have a smaller dependency surface in some configurations, but would complicate straightforward cross-compilation and first-time contributor setup.
- Keep schema version 1 in `PRAGMA user_version` and use application ID `0x534b5931` (SKY1). Initialize the table and metadata in one immediate transaction. Refuse unrelated databases and unsupported schema versions; do not silently downgrade them.
- Store `kind`, canonical `identifier`, and a UTC creation timestamp in `watch_targets`. The composite primary key enforces uniqueness. Registration is uppercased; hex is lowercased. Duplicate adds and missing removals are successful no-ops.
- A registration and hex for the same physical aircraft remain separate targets until provider-assisted identity resolution is implemented. This issue performs no provider requests.
- Use SQLite's default rollback journal for this small watchlist. Existing WAL-mode databases are supported by backup; revisit WAL as a default with observation-history requirements.
- Configure a five-second busy timeout, immediate write transactions, one connection per Store, foreign keys, and disabled trusted schema. Keep secrets/configuration out of the database.
- Back up through `VACUUM INTO`, verify application/schema/data integrity, and sync the output file. Never copy just the main file of a running WAL database. Existing backup paths are refused.
- Restore only into a destination without an existing database or database sidecar. Read the backup in a read-only transaction, validate its identity and targets, import into a fresh staged schema, then publish with a non-overwriting hard link. This preserves creation timestamps and prevents backup triggers/extra objects from being imported as executable schema.
- Hard-link publication requires a supporting filesystem. Common local macOS/Linux filesystems and Windows NTFS support this; unsupported filesystems return an error instead of falling back to an overwrite. A failed normal restore removes its staging file. A forced process termination can leave a hidden staging file, but does not replace existing data.

## Paths

Configuration currently uses flags/environment only; no separate configuration file is introduced or needs backup. Credentials remain outside the watchlist database.

Precedence is `watch --data-dir PATH`, then `SKY_DATA_DIR`, then:

| Platform | Default |
| --- | --- |
| macOS | `~/Library/Application Support/sky` |
| Linux / other Unix | `$XDG_DATA_HOME/sky` when absolute; otherwise `~/.local/share/sky` |
| Windows | `%LOCALAPPDATA%\Sky`; fallback `%APPDATA%\Sky`, then the home directory's `AppData\Local\Sky` |

The database is `watchlist.db`. Relative explicit/environment paths are resolved from the current working directory. New directories/files use Unix modes 0700/0600; existing directory modes are not changed and Windows ACL behavior is platform-dependent.

## Consequences

Normal builds remain free of CGO, at the cost of a larger dependency graph and binary. Dependency updates that raise the Go requirement need an explicit compatibility decision. Packaged releases must retain dependency license notices.

Recovery selects a new data directory rather than replacing the active watchlist. The user verifies the recovered list and deliberately selects it with `--data-dir` or `SKY_DATA_DIR`. Restoring a snapshot does not recover changes made after that snapshot. Git commits and tags do not back up local databases.

There is no schema upgrade from a prior persistent Sky release yet. Future schema changes need transactional migrations, a pre-migration backup, and compatible restore/downgrade tests before release.
