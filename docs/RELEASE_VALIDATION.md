# M1 release validation

Issue #9 prepares the first source release candidate, `v0.2.0-rc.1`. This is not a stable release or a promise of tested native binaries on every platform. Platform packaging, checksums and broader runtime support remain M4.

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
