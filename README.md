# Sky

[![CI](https://github.com/Mickyj70/sky/actions/workflows/ci.yml/badge.svg)](https://github.com/Mickyj70/sky/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A Go CLI for observed aircraft, nearby traffic, and regional airline activity using [adsb.lol](https://www.adsb.lol/).

## Install the development version

Requires Go 1.24 or newer:

```sh
go install github.com/Mickyj70/sky/cmd/sky@main
```

Add your Go binary directory (`go env GOPATH` plus `/bin`, unless `GOBIN` is set) to PATH. This installs the current development branch; no stable release or prebuilt binaries are available yet.

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

## Scope

Implemented: aircraft and callsign lookup, nearby search, regional operator filtering, JSON output, timeout and rate-limit handling, interrupt cancellation.

Next: persisted watchlists and quota-aware polling; SQLite observation history; a separate flight-status provider for dated flight numbers, schedules, routes, delays, and ETAs. No hosted backend is required for the current commands.

## Development

```sh
go test ./...
go vet ./...
```

Tests use local HTTP servers and require no provider credentials or live network.

Data source: [adsb.lol API](https://www.adsb.lol/docs/open-data/api), published under ODbL 1.0. Review the provider's license and current access limits before redistributing data.

## Project direction and progress

- [Roadmap](docs/ROADMAP.md): milestones, deliverables, dependencies, and completion criteria.
- [Current status](docs/STATUS.md): what works, current focus, open decisions, and next steps.
- [Initial backlog](docs/BACKLOG.md): issue-ready tasks for GitHub.
- [GitHub setup](docs/GITHUB.md): repository, milestones, board, and required checks.
- [Recovery guide](docs/RECOVERY.md): checkpoints, safe reverts, and future database recovery.
- [Contributing](CONTRIBUTING.md): branch/PR workflow and verification.
- [Changelog](CHANGELOG.md) and [architecture decisions](docs/decisions/0001-local-go-cli.md).

The current implementation is a prototype. Track work in [issues](https://github.com/Mickyj70/sky/issues) and [milestones](https://github.com/Mickyj70/sky/milestones). No stable release has been published.

## Contributing

Contributions are welcome. Start with [good first issues](https://github.com/Mickyj70/sky/labels/good%20first%20issue), read [CONTRIBUTING.md](CONTRIBUTING.md), and open a focused pull request from your fork. Discuss larger changes in an issue before implementing them. Contributors do not need API credentials to run the offline test suite.

Please follow our [Code of Conduct](CODE_OF_CONDUCT.md). Report vulnerabilities using the [security policy](SECURITY.md).

## License

Sky source code is licensed under [MIT](LICENSE). Flight data has separate provider terms, including adsb.lol's ODbL 1.0; MIT does not relicense upstream data.
