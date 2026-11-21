# Release validation

Issue #9 established validation for the M1 source candidate, `v0.2.0-rc.1`. The current runner additionally validates the M2 candidate, `v0.3.0-rc.1`, as described below. This is not a stable release or a promise of tested native binaries on every platform. Platform packaging, checksums and broader runtime support remain M4.

## Reproduce the checks

Requires a Git checkout with checkpoint tags, Go 1.24+, Python 3.9+, and the C toolchain needed for Go race tests. Normal Sky builds remain CGO-free. Keep the working checkout and your real data separate from validation.

```sh
git fetch origin --tags
python3 scripts/validate_release.py \
  --candidate-ref HEAD \
  --report /tmp/sky-release-validation.json
```

Use `--go /absolute/path/to/go` if the normal `go` command is unavailable. On Windows, choose a suitable report path instead of `/tmp`. The runner validates **committed source**, not uncommitted working changes. It resolves and records the candidate/baseline commit IDs before archiving them into temporary directories. It never checks out or resets your working files.

After the candidate tag exists, use `--candidate-ref v0.2.0-rc.1`; the runner also verifies that the tag name matches the canonical source version. A different compatible checkpoint can be selected with `--baseline-ref`, and its exact identity is recorded.

## Evidence produced

The JSON report records timestamp, candidate/baseline commits, canonical version, Go version, native platform/architecture, completed checks and limitations. CI runs the validator from a full checkout on Go 1.24.x and preserves the report as the `release-validation` artifact for 30 days. Normal Go 1.24.x and stable CI checks also run independently.

The clean candidate archive must pass:

1. The full race suite: provider fixtures, quota/cooldown behavior, unknown/stale/missing state transitions, output failure and interrupt handling.
2. `go vet`, a build without CGO, exact `--version` identity and command entrypoint checks.
3. Real binary add → duplicate → restart/list → backup → remove → restore → list. Each operation runs in a separate process.
4. Non-overwriting backup/restore refusal, unchanged active data after rejection, and an empty-watchlist guard before polling.
5. Schema-1 downgrade/re-upgrade reads/writes and two-way snapshot recovery with the baseline checkpoint.

Native live polling uses the local HTTP source injected by the Go subprocess tests. The production binary has no arbitrary API URL environment override. The validator makes no public flight-data API calls; initial Go module downloads may need internet access.

## Compatibility scope

Baseline: `checkpoint/watchlists-review-2026-11-17` (`8ad2424445295c79940599d63cdae984f896a82c`), the earlier schema-1 watchlist implementation. It is an unreleased source checkpoint, not a previous stable release.

Both binaries must read the same saved target values/timestamps. The baseline must write data the candidate can reopen and restore a candidate backup; the candidate must restore a baseline snapshot. The validator does this only on temporary data.

This proves compatibility with that checkpoint and schema 1. It does not prove compatibility with every future/older schema or restore observations that never existed in a snapshot. Other native platforms need their own runtime evidence; cross-compilation alone is insufficient.

## Candidate publication

1. Set the canonical identity in `internal/version/version.go`; both CLI version and provider user agent consume it.
2. Commit candidate code, notes and documentation; validate that commit from clean archives.
3. Open the release-preparation PR and require green Go 1.24.x, stable, and M1 release-validation jobs.
4. Create the annotated candidate tag only on the tested commit and push it. Never move an existing candidate tag; fixes receive `rc.2`, `rc.3`, etc.
5. A candidate may be tagged from its reviewed release-preparation branch before main merges. It remains explicitly pre-release; the tag identifies the exact tested source.
6. Keep notes at `docs/releases/v0.2.0-rc.1.md`. No prebuilt binary assets are part of this candidate. Stable `v0.2.0` requires a separate final release decision after candidate feedback and remaining acceptance checks.

## Safe upgrade and recovery

Before replacing a binary that manages watchlists, create a uniquely named backup with that binary. Keep the current binary and data directory until the candidate passes your local smoke check.

Build a previous checkpoint in a separate worktree if rollback is needed. Use a backup supported by that binary's schema and restore into a **new** data directory; verify the list before selecting it. The previous checkpoint manages schema-1 targets but does not implement live `watch run`. Reverting to it restores those earlier capabilities, not the newer command.

No database migration is introduced by the release-preparation change. See RECOVERY and WATCHLISTS for the full procedure. Neither Git tags nor a release asset back up your local database.

## M2 candidate and history acceptance

Current release preparation targets `v0.3.0-rc.1`; the M1 source tag remains unchanged. Run the current committed validator against matching current source. The scripts themselves must match the candidate archive; use the scripts from a tagged checkout to reproduce an earlier candidate rather than mixing newer tooling with old source.

```sh
git fetch origin --tags
python3 scripts/validate_release.py --candidate-ref HEAD --report /tmp/sky-validation.json
```

The runner now resolves **three** exact revisions before export: candidate, watchlist baseline (`--baseline-ref`, unchanged), and history baseline (`--history-baseline-ref`, default `97d6db1776c3ebbd3712dff770ebfaf23e40a5f2`, PR #23). The JSON report adds history baseline identity, validator SHA-256 hashes, synthetic fixture-writer hash and completion timestamp. It contains no collected history, credentials or personal database contents. CI's **M1 and M2 release validation** job preserves it as the same 30-day `release-validation` artifact. Go 1.24.x and stable checks remain separate.

Clean-archive tests/vet and production binary builds run first. The runner then copies the archived `scripts/history_fixture.go.txt` into a new temporary command directory in each candidate/history-baseline source archive and compiles **separate synthetic writers**. This harness uses each revision's real `history.Record` API; it is not linked into either production CLI binary, adds no public API endpoint override, and makes no provider requests. The production source files are not modified. The fixture source/hash is preserved so this distinction is reviewable.

Additional native checks must pass:

1. Candidate writes → earlier binary reads; earlier API writes → candidate reads; replay across revisions is a no-op.
2. One-record pages reconstruct the exact saved observations; target normalization and inclusive/exclusive UTC bounds agree. JSON/CSV preserve target, times, state, aircraft fields/units/missing values, source attribution and continuation metadata.
3. Each binary restores the other's verified history snapshot without changing record keys/payloads. Backup/restore overwrite refusals leave active observations intact.
4. Prune preview changes nothing; apply deletes strictly older encounters and persists its counts; a pre-prune snapshot recovers all observations into a new destination.
5. History restore preserves saved targets, and watchlist restore preserves history.
6. Unsupported/unrelated history identities are refused by both binaries without changing database bytes; corrupt backup rows are never published as restored history.

All data is synthetic and temporary. SQLite's backup API is used for corrupt-copy fixtures; no running SQLite main file is copied as a backup. Fixture HTTP polling, retention failures, output failures and cancellation remain covered by the Go suite. There is no in-place schema migration to test in this candidate; any future migration still needs automatic verified pre-migration backup and transactional failure/downgrade evidence before shipping.

For publication, require the query/export dependency (#25) merged, green Go 1.24/stable and combined release validation, and a clean validation report for the exact candidate commit. Then create the annotated immutable tag; record it in the release PR. M2 notes live in `docs/releases/v0.3.0-rc.1.md`. This source candidate has no prebuilt assets and is not a stable release. A fix after tagging needs a new candidate number.
