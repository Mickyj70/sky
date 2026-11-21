# Local observation history

Sky records successful watch responses locally when recording is enabled. Collection runs only while the foreground process runs; it cannot fetch earlier flights or infer landed from missing coverage. Queries, exports and pruning use only this local database and never contact a provider.

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

## Offline history queries

```sh
sky history --registration G-ZBKE --since 24h
sky history --hex 406d7b --since 2026-11-01T00:00:00Z --until 2026-12-01T00:00:00Z --json
sky history --since 2026-11-01T00:00:00Z --limit 100 --offset 100 --json
```

Without a target filter, queries include all targets. Registration filters are uppercased and hex filters lowercased; they are mutually exclusive and do not resolve aliases. Bounds select the **first local encounter time**, inclusive `--since` and exclusive `--until`, not provider snapshot time. RFC3339 timestamps require a timezone and are normalized to UTC. `--since` also accepts a positive duration such as `24h`, evaluated once at command start; `--until` requires an absolute timestamp. There is no default time bound. Supported times must fit signed Unix nanoseconds (approximately 1678–2262).

Each page is ordered by encounter time ascending, then stable record key. Default `--limit` is 100, maximum 1000; `--offset` ranges from 0 to 1000000. Payloads in one page are capped at 16 MiB; lower the limit for unusually large observations. A single row larger than the cap requires separate investigation; increasing the limit cannot fix it. Empty results succeed. JSON includes `collection: "local"`, normalized `since`/`until` when supplied, `records: []`, `attribution: []`, `limit`, `offset`, `has_more` and optional `next_offset`. Each record contains a stable `key` and the stored `observation` envelope.

Each command reads one consistent SQLite transaction. Offset pages are deterministic on an unchanged database, but inserts or pruning between commands can shift offsets; stop recording and avoid pruning while exporting a multi-page dataset. Freeze relative bounds to the absolute UTC values returned by the first JSON page. At the maximum offset a page may indicate `has_more` without a next offset; narrow the target/time bounds. This is bounded pagination, not a single-command full-database dump.

## JSON and CSV export

```sh
sky history export --registration G-ZBKE --since 2026-11-01T00:00:00Z --format json > history-page.json
sky history export --registration G-ZBKE --since 2026-11-01T00:00:00Z --format csv > history-page.csv
```

Export accepts the same filters/limit/offset and exports **one page**, defaulting to JSON. `--json` conflicts with explicit CSV format. JSON has the same page envelope as a query. CSV has one row per snapshot, not one row per aircraft, with a header even for empty results:

`key, collection, target_kind, target_identifier, encounter_at_utc, snapshot_at_utc, retrieved_at_utc, state, position_age_seconds, source, source_license, attribution_url, aircraft_json, page_has_more, page_next_offset`

`aircraft_json` preserves the known aircraft fields and existing units: `alt_baro` is feet or `"ground"`, `gs` knots, `track` degrees, `lat`/`lon` decimal degrees, and `seen`/`seen_pos` seconds before the provider snapshot. Missing aircraft fields remain absent; real zeros are preserved. Missing snapshot time or position age is an empty CSV cell. Empty aircraft results retain `[]` (or a previously stored null array) inside the cell. CSV quoting preserves commas, quotes and newlines within fields.

Source text that resembles a spreadsheet formula, including after leading whitespace/control characters/BOM, is prefixed with an apostrophe in CSV. This deliberately changes unsafe text cells for spreadsheet use; JSON retains the original source text and is the lossless option for it. Generated numeric cells and aircraft JSON are not converted to spreadsheet formulas. Provider attribution is retained per source in JSON and per row in CSV; adsb.lol uses ODbL 1.0 and its data API documentation link. Unrecognized sources are marked `See provider terms`; no license is invented for them.

Query/export validates row payloads and indexed identities before emitting a page. Corrupt or unsupported data fails with no result document. An output-device failure can still leave a partial file; discard output from any failed command. These exports are not SQLite recovery snapshots; use `history backup` for recovery.

## Explicit offline pruning

```sh
sky history prune --before 2026-10-01T00:00:00Z --json
sky history backup ./history-before-prune.db
sky history prune --before 2026-10-01T00:00:00Z --apply --json
```

The first command previews only. `--apply` deletes all targets' encounters strictly before the cutoff. The cutoff must be an absolute RFC3339 timestamp with timezone. JSON reports `database`, UTC `before`, `applied`, `matched`, and `deleted`. In apply mode, count and deletion occur in one immediate transaction; failures roll back. Preview is a separate read transaction, so its count can differ from a later apply if another process writes/prunes in between. Saved targets are never pruned. Pages are reused rather than shrinking the file. A verified pre-prune snapshot is the recovery path for intentional deletion.

These commands use existing schema-1 tables/indexes. No migration or new dependency is introduced; the earlier recording binary can still manage this schema. Separate M2 release validation must verify cross-version history recovery before any stable compatibility promise.
