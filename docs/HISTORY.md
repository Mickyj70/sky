# Local observation recording

M2's first slice records successful watch responses locally. It is opt-in and collects only while the foreground process runs; it cannot fetch earlier flights or infer landed from missing coverage. Queries and CSV/JSON exports follow in separate work.

```sh
sky watch add --registration G-ZBKE
sky watch run --interval 5m --max-requests 10 --record
sky history status
```

Use `--json` for NDJSON watch events or a single JSON object from history maintenance commands. Recording adds `history_database` and `history_retention_seconds` to the `started` event; normal runs omit those fields. Lookup JSON and request accounting are unchanged. No extra provider requests are made.

## Data and deduplication

`history.db` lives beside `watchlist.db`. Both use `--data-dir`, then `SKY_DATA_DIR`, then the [platform data directory](WATCHLISTS.md). Use the same directory when recording and managing history. A `history status` or backup on a fresh directory initializes an empty history store; neither accesses a provider.

Each row preserves the first local encounter's target, source, provider snapshot time (when available), retrieval/evaluation times, freshness state and known aircraft fields. Successful empty responses are stored; failures remain live events and are not stored. UTC timestamps and existing aircraft units are retained. A stored state describes that encounter, not the aircraft's current status.

Repeating the same target/source/snapshot/content retains one row across restarts, even if retrieval time or evaluated freshness changes. Different snapshot timestamps retain separate rows even with unchanged aircraft position. When the provider has no snapshot time, retrieval time is part of identity. Registration and hex aliases stay distinct. Recording happens before terminal/NDJSON repeat suppression, so stored row count can exceed displayed observation count.

## Retention and disk use

```sh
sky watch run --interval 5m --max-requests 10 --record --history-retention 168h
sky history status --json
```

Retention defaults to `720h` (30 days). Use positive Go duration syntax, such as `168h`; `7d` is not valid. An explicit `--history-retention` requires `--record`. The selected retention applies to the entire history file, across all targets, at each successfully recorded lookup. It removes first local encounters strictly older than that encounter time minus retention. It does not run during outages, sleep or offline status checks. Concurrent runs with different retention settings share that file, so the shortest active policy can remove older rows.

Retention reuses freed database pages but does not shrink the file or impose a fixed byte limit. `history status` returns `database`, `count` and `bytes` (allocated main-file bytes). Provider cadence, number of targets, payload size and retention affect disk use. Clock changes can affect pruning. Failed recording stops further polling and returns a failing exit; retention and insertion roll back together. No write lock is held while waiting on the provider or interval.

## Backup and restore

Stop recording before recovery and take distinct snapshots of history and saved targets:

```sh
sky history backup ./history-before-change.db
sky watch backup ./watchlist-before-change.db
sky history restore ./history-before-change.db --data-dir ./recovered-sky
sky watch restore ./watchlist-before-change.db --data-dir ./recovered-sky
sky history status --data-dir ./recovered-sky
sky watch list --data-dir ./recovered-sky
```

Snapshot destinations must not exist. Each snapshot is consistent independently; the pair is not a single shared-time transaction. History restore refuses existing history files/sidecars and requires hard-link support. It may share a destination with a watchlist, and never modifies that watchlist. Watchlist restore likewise leaves history untouched. Do not copy a running SQLite main file as a backup.

No schema migration is introduced: watchlist remains schema 1 and history starts its own schema 1. Unknown schema identities are rejected. The M1 binary ignores history and remains compatible with saved targets; retain a newer binary to access history. Future migrations must include automatic pre-migration backups and failure/recovery tests before implementation ships; see [ADR 0003](decisions/0003-observation-history.md) and [recovery](RECOVERY.md).

## Licensing and limits

Collected adsb.lol data remains subject to its ODbL 1.0 terms and attribution obligations, separately from Sky's MIT code license. Keep source/provider identity with stored data. Review applicable data terms before sharing a database or derived export; storage is not a grant of redistribution rights. Never commit collected flight histories, personal databases or credentials to this repository. Tests use synthetic local fixtures.
