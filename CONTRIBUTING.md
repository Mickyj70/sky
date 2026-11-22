# Development and progress workflow

Start with [current status](docs/STATUS.md), then the [roadmap](docs/ROADMAP.md). New features should have an issue and acceptance criteria before implementation. The issue seed is [BACKLOG.md](docs/BACKLOG.md).

## First contribution

Fork [Mickyj70/sky](https://github.com/Mickyj70/sky), clone your fork, and add the upstream repository:

```sh
git clone https://github.com/YOUR_USERNAME/sky.git
cd sky
git remote add upstream https://github.com/Mickyj70/sky.git
git fetch upstream
git switch -c docs/my-improvement upstream/main
```

Replace `YOUR_USERNAME` with your GitHub account. Use an existing unassigned issue or open an issue for larger changes. A maintainer can help scope work; picking an issue does not guarantee assignment or a review deadline. Documentation and test improvements are welcome alongside features.

Install Go 1.24+ and run the local checks below. No API key is needed for tests. Push your branch to your fork and open a PR against `Mickyj70/sky:main`. Direct write access to the upstream repository is not required.

## One change at a time

1. Choose a Ready issue in the current milestone; mark it In progress.
2. Branch from updated main: `feat/watchlist-storage`, `fix/provider-timeout`, or `docs/recovery`.
3. Make small, coherent commits. Keep credentials, local databases, and binaries out of Git.
4. Add tests for important behavior or regressions; use local fixtures instead of live provider calls in CI.
5. Open a PR with the issue link, behavior change, verification evidence, and rollback instructions. Update STATUS and CHANGELOG as appropriate.
6. Mark In review, pass required CI, then review and squash-merge. Close the issue only after acceptance criteria are met.
7. Update milestone progress; create an annotated checkpoint at a tested milestone boundary.

Initial planning and the prototype were committed directly to establish the baseline. Subsequent implementation should use the PR workflow after GitHub setup. Main should stay buildable. Never rewrite shared main history to undo a feature; use a revert PR.

## Local checks

Requires Go 1.24+. CI runs the minimum version plus current stable Go on Linux. Cross-platform runtime support will be verified before the distribution milestone promises it.

```sh
gofmt -l cmd internal
go test -race ./...
go vet ./...
go build -o bin/sky ./cmd/sky
```

`gofmt -l` must print no paths. Live smoke checks are optional, consume provider quota, and should record the date and command without recording secrets. An API outage must not fail the offline test suite.

## Windows setup with PowerShell

Install Go 1.24 or newer and Git, then follow the fork and clone instructions above. Run these commands from the repository root.

### Build and run

```powershell
go version
go build -o bin/sky.exe ./cmd/sky
.\bin\sky.exe --help
```

### Run offline checks

```powershell
gofmt -l cmd internal
go test ./...
go vet ./...
```

`gofmt -l` must print no paths. These tests use local fixtures and require no provider API key or live flight-data requests.

Race detection requires CGO and a compatible C compiler installed and available on PATH:

```powershell
go test -race ./...
```

If the required toolchain is unavailable, run the ordinary tests above and report that race detection was not run locally. GitHub CI runs race checks on Linux.

### Optional provider credentials

Public lookups can run without a key within the provider's current limits. If you have a provider key, set it for the current PowerShell session:

```powershell
$env:SKY_API_KEY = "your-key"
```

Remove it when finished:

```powershell
Remove-Item Env:SKY_API_KEY -ErrorAction SilentlyContinue
```

Never commit credentials or paste them into issues, pull requests, or logs.

### Platform verification

Current GitHub CI runs on Linux. These instructions do not establish verified Windows runtime support. Contributors testing on Windows should record their Windows version, Go version, and check results in their pull request.

## Decisions and compatibility

Record significant choices under `docs/decisions/` with context, decision, consequences, and status. Examples: storage driver, provider contracts, JSON compatibility, paid provider selection, or a hosted backend.

During alpha development, document breaking changes before merging. Before 1.0, define the stable command/JSON contract and deprecation policy. Data migrations require the recovery plan in [RECOVERY.md](docs/RECOVERY.md).

## Licensing and community

By submitting a contribution, you agree to license it under this project's MIT license and confirm you have the right to contribute it. No CLA or DCO sign-off is required. Keep third-party license notices intact. Provider data licenses remain separate; use synthetic/minimal test fixtures rather than committing collected aircraft histories.

Follow the [Code of Conduct](CODE_OF_CONDUCT.md). Report security concerns privately as described in [SECURITY.md](SECURITY.md). Never put an API key in an issue, PR, log, or fixture.

Fork pull requests run offline checks with read-only repository permissions and do not receive maintainer secrets. Workflows must not execute untrusted PR code with privileged credentials. Maintainers may need to approve a first-time contributor's workflow run in GitHub.
