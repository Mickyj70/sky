# Clean-install acceptance test

Use the published Apple Silicon ZIP and matching checksum from the [1.0 release candidate](https://github.com/Mickyj70/sky/releases/tag/v1.0.0-rc.1). Follow the [binary installation guide](BINARY_INSTALL.md), then run the commands below from the extracted directory on macOS. Start with a fresh shell and confirm the version printed is `1.0.0-rc.1`.

```sh
./sky --version
work=$(mktemp -d)
export SKY_DATA_DIR="$work/active"
./sky nearby --lat 9.0068 --lon 7.2632 --radius-km 100 --airborne
```

If Abuja returns no observations, record the result and use Heathrow to finish the workflow.

```sh
./sky nearby --lat 51.47 --lon -0.45 --radius-km 100 --airborne
```

Choose a registration from the current table and replace the placeholder below. If only an ICAO hex is available, use `--hex` consistently in the watch, history and export commands.

```sh
registration='REPLACE-WITH-OBSERVED-REGISTRATION'
./sky watch add --registration "$registration"
./sky watch list --json
./sky watch run --record --interval 10s --max-requests 3 --json > "$work/watch.ndjson"
cat "$work/watch.ndjson"
./sky history --registration "$registration" --json > "$work/history.json"
./sky history export --registration "$registration" --format json > "$work/export.json"
./sky history export --registration "$registration" --format csv > "$work/history.csv"
cmp "$work/history.json" "$work/export.json"
./sky watch backup "$work/watch.db"
./sky history backup "$work/history.db"
./sky watch --data-dir "$work/restored" restore "$work/watch.db"
./sky history --data-dir "$work/restored" restore "$work/history.db"
./sky watch --data-dir "$work/restored" list --json
./sky history --data-dir "$work/restored" --registration "$registration" --json > "$work/restored-history.json"
cmp "$work/history.json" "$work/restored-history.json"
```

Pass when the watch stops at three requests, at least one recorded snapshot contains an aircraft, the original and restored watchlists contain the same target and creation time, and the JSON comparisons succeed. Compare CSV keys and aircraft payloads with the JSON export. Watchlist database paths will differ. Record the platform, version, lookup area, timestamp, observation count, any provider error and the restore result. An empty snapshot cannot satisfy the live observation check.

## Named-place checks on the development branch

Build the updated checkout first with `go build -o bin/sky ./cmd/sky`. These additions are newer than the published candidate.

```sh
./bin/sky nearby Abuja
./bin/sky nearby --state Lagos
./bin/sky history Abuja --day today --json
./bin/sky history Abuja --since 2h --json
./bin/sky history export Abuja --day 2026-11-22 --format csv
```

Named places use an airport-centered radius, default 100 km. The supported presets are Abuja/FCT, Lagos and Heathrow. Extend the radius with `--radius-km`. A date selects midnight through the next midnight in the place’s timezone. Use `--since` and `--until` for a custom interval. History selects whole recorded watch snapshots with at least one matching position. It cannot retrieve unrecorded traffic or reconstruct every flight through a state that day. Check continuation fields for additional pages.
