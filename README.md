# Sky

[![CI](https://github.com/Mickyj70/sky/actions/workflows/ci.yml/badge.svg)](https://github.com/Mickyj70/sky/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A Go CLI for observed aircraft, nearby traffic, and regional airline activity using [adsb.lol](https://www.adsb.lol/).

## Source release candidate

The published M1 source candidate is **v0.2.0-rc.1**. M2 source candidate **v0.3.0-rc.1** is published with [history/recovery notes](docs/releases/v0.3.0-rc.1.md) and native cross-version validation. See [candidate notes](docs/releases/v0.2.0-rc.1.md) and [validation/recovery evidence](docs/RELEASE_VALIDATION.md). It is not a stable release and has no prebuilt binary assets.

Install the source candidate with Go 1.24+:

```sh
go install github.com/Mickyj70/sky/cmd/sky@v0.2.0-rc.1
```

The M1 candidate does not include the M2 history commands below. Both current main and the M2 source candidate include recording/query/export.

Native CI builds are available after successful packaging runs. See [binary installation and checksum verification](docs/BINARY_INSTALL.md).

## Install the binary prerelease

Download the ZIP and matching SHA-256 sidecar for Linux amd64, Apple Silicon macOS, or Windows amd64. [Sky v0.5.0-rc.1 downloads](https://github.com/Mickyj70/sky/releases/tag/v0.5.0-rc.1)

- Linux: run `sha256sum -c <sidecar>`. On macOS use `shasum -a 256 -c <sidecar>`. Replace `<sidecar>` with the downloaded checksum filename. After a successful check, extract the ZIP to a new directory and run `chmod +x sky` inside it.
- Windows: run `Get-FileHash <zip> -Algorithm SHA256` and compare its hash to the first field in the sidecar. After they match, extract the ZIP to a new directory.

Inside the extracted directory, run the following on Linux or macOS. In PowerShell, replace `./sky` with `.\sky.exe`.

```sh
./sky --version
./sky plane G-ZBKE --json
```

Expect version `0.5.0-rc.1`. A successful lookup can return an empty `aircraft` array when the provider has no matching observation. See the [installation guide](docs/BINARY_INSTALL.md) for completion and rollback instructions.

## Install the development version

Requires Go 1.24 or newer:

```sh
go install github.com/Mickyj70/sky/cmd/sky@main
```

Add your Go binary directory (`go env GOPATH` plus `/bin`, unless `GOBIN` is set) to PATH. This installs current main. Use the binary prerelease above if you prefer a prebuilt executable.

## Build from source

Requires Go 1.24 or newer.

```sh
git clone https://github.com/Mickyj70/sky.git
cd sky
go build -o bin/sky ./cmd/sky
./bin/sky --help
```

## Commands

```sh
# Aircraft registration, or an explicit ICAO hex address
sky plane G-VBEL
sky plane --hex 4073ce --json

# Broadcast callsign; this is not a scheduled flight-number lookup
sky callsign BAW123

# Coordinates are required; radius is in kilometres (default 100, max 463)
sky nearby --lat 6.58 --lon 3.32 --radius-km 100
sky nearby --lat 51.47 --lon -0.45 --radius-km 100 --airborne --json

# Observed operator activity within an area, using a three-letter ICAO prefix
sky airline BAW --lat 51.47 --lon -0.45 --radius-km 100 --airborne --json

# Optional provider API key; no key is embedded in the binary
export SKY_API_KEY='your-key'
sky plane G-VBEL --timeout 10s
```

If running the binary directly after building, replace `sky` above with `./bin/sky`, or put it on your PATH.

Each invocation makes one request. There is no automatic polling or retry loop. HTTP 429 errors surface the provider's Retry-After header. Consult the provider's current quota before automating requests; a once-per-minute query uses 1,440 requests/day.

## Reading results

- Table output includes registration, broadcast callsign, aircraft type, barometric altitude (feet), ground speed (knots), ground track (degrees), coordinates, message age, and position age.
- `ground` is the provider's explicit ground altitude value. `--airborne` includes numeric barometric altitude observations and excludes unknown/ground altitude; it is not authoritative flight status.
- A dash means unavailable. Zero is preserved as a valid reading.
- Message and position ages are seconds before the provider snapshot, not the local fetch time. Both snapshot and retrieval timestamps are included.
- JSON wraps normalized provider fields in `source`, `retrieved_at`, `snapshot_at`, and `aircraft`. Altitude may be numeric or `"ground"`; missing readings remain absent. An empty result has `"aircraft": []`.
- No observation does not establish whether an aircraft is landed, inactive, or outside receiver coverage.
- Airline filtering uses broadcast callsign prefixes. It is a regional view of observed operator activity, not an authoritative airline fleet or codeshare lookup.

See the [CLI/JSON/error contract](docs/CLI_CONTRACT.md) for field types, exit codes, provider failures, and alpha compatibility.

## Saved watchlists

```sh
sky watch add --registration G-ZBKE
sky watch add --hex 406d7b
sky watch list --json
sky watch remove --registration G-ZBKE
```

Targets persist locally without provider calls. See [watchlists and safe backup/restore](docs/WATCHLISTS.md) for data locations, overrides and recovery. The live command uses a required finite request budget:

```sh
sky watch run --interval 5m --max-requests 100
sky watch run --interval 5m --max-requests 10 --json
```

See [live watch output](docs/WATCH_RUN.md) for observation states, NDJSON events and per-run budget limits. Tracking stops when the process exits.

## Scope

Implemented: aircraft and callsign lookup, nearby search, regional operator filtering, JSON output, timeout and rate-limit handling, interrupt cancellation.

Implemented additionally: persistent watchlists, SQLite snapshot backup and non-overwriting restore.

Current development adds opt-in SQLite snapshot recording (see [history](docs/HISTORY.md)); offline history queries, pruning and CSV/JSON export have merged. M3 [provider evaluation](docs/PROVIDER_EVALUATION.md) is evaluation-only at a $0 budget; a provider-neutral offline model comes next before any commercial integration. No hosted backend is required for the current commands.

## Development

```sh
go test ./...
go vet ./...
```

Tests use local HTTP servers and require no provider credentials or live network.

Data source: [adsb.lol API](https://www.adsb.lol/docs/open-data/api), published under ODbL 1.0. Review the provider's license and current access limits before redistributing data.

## Project direction and progress

- [Architecture and Go packages](docs/ARCHITECTURE.md): components, dependencies, and planned tracking boundary.
- [Roadmap](docs/ROADMAP.md): milestones, deliverables, dependencies, and completion criteria.
- [Current status](docs/STATUS.md): what works, current focus, open decisions, and next steps.
- [Initial backlog](docs/BACKLOG.md): issue-ready tasks for GitHub.
- [GitHub setup](docs/GITHUB.md): repository, milestones, board, and required checks.
- [Release validation](docs/RELEASE_VALIDATION.md): clean-source checks, schema compatibility, and candidate procedure.
- [Recovery guide](docs/RECOVERY.md): checkpoints, safe reverts, and future database recovery.
- [Contributing](CONTRIBUTING.md): branch/PR workflow and verification.
- [Changelog](CHANGELOG.md) and [architecture decisions](docs/decisions/0001-local-go-cli.md).

The current implementation is pre-release software. Track work in [issues](https://github.com/Mickyj70/sky/issues) and [milestones](https://github.com/Mickyj70/sky/milestones). No stable release has been published.

## Contributing

Contributions are welcome. Start with [good first issues](https://github.com/Mickyj70/sky/labels/good%20first%20issue), read [CONTRIBUTING.md](CONTRIBUTING.md), and open a focused pull request from your fork. Discuss larger changes in an issue before implementing them. Contributors do not need API credentials to run the offline test suite.

Please follow our [Code of Conduct](CODE_OF_CONDUCT.md). Report vulnerabilities using the [security policy](SECURITY.md).

## License

Sky source code is licensed under [MIT](LICENSE). Flight data has separate provider terms, including adsb.lol's ODbL 1.0; MIT does not relicense upstream data.

## Record observations locally (development)

```sh
sky watch run --interval 5m --max-requests 10 --record
sky history status
sky history backup ./history-snapshot.db
```

Requires saved targets. Recording is opt-in, stores successful snapshots in a separate `history.db`, and defaults to 30-day retention applied during successful recording. It does not add provider requests. The current history query/export change (#22) adds the commands below. See [HISTORY.md](docs/HISTORY.md) for replay identity, retention limits, licensing and independent backup/restore.

### Query and export local history (development)

History recording/query/export has merged. Update and rebuild the development branch:

```sh
sky history --registration G-ZBKE --since 24h --json
sky history export --registration G-ZBKE --format csv --limit 100 > history-page.csv
sky history prune --before 2026-10-01T00:00:00Z --json
```

Queries and exports are bounded pages of locally collected snapshots, with no provider calls. Check `has_more`/`next_offset` (or CSV page columns) before treating an export as complete. Pruning previews by default; back up first and add `--apply` to delete. See [history command contract](docs/HISTORY.md) for UTC bounds, units, pagination, spreadsheet text handling and recovery.

See the [acceptance walkthrough](docs/ACCEPTANCE_TEST.md) for the clean-install test and named-place examples.
