# Checkpoints, releases, and recovery

Git stores committed source and documentation. It does not store ignored binaries, API keys, local configuration, or SQLite data. A remote on GitHub protects pushed commits against loss of this workspace; a local repository alone does not.

## Checkpoint policy

- Commit each coherent change and merge small PRs into main.
- Annotated `checkpoint/<description>-YYYY-MM-DD` tags identify verified intermediate states. If multiple checkpoints share a date, add a unique suffix.
- Use `vX.Y.Z` only for actual tested releases. The version targets in the roadmap are plans, not release tags.
- Never move or overwrite a shared checkpoint/release tag. Make a new one.
- Push main and tags after setup; a local tag is not an off-machine backup.
- A commit message/PR records validation. Each feature or migration PR includes its own rollback note.

Pushed starting points:

- `checkpoint/prototype-2026-11-15` at `b0ff308`: initial working Go CLI.
- `checkpoint/planning-2026-11-15` at `1fa4d1e`: roadmap and recovery foundation.

The public contributor setup checkpoint is `checkpoint/open-source-2026-11-15`, created after its setup PR passes CI and merges. `git tag --list 'checkpoint/*'` shows available local checkpoints; `git fetch origin --tags` retrieves pushed ones.

## Inspect an old version without changing current work

From the project root, first inspect your working tree. A separate worktree leaves current uncommitted files alone:

```sh
git status --short
git tag --list 'checkpoint/*'
git worktree add --detach ../sky-prototype checkpoint/prototype-2026-11-15
cd ../sky-prototype
go build -o bin/sky ./cmd/sky
./bin/sky --help
```

Build output is ignored. Use a different destination directory if `../sky-prototype` already exists. Inspect files before removing a recovery worktree; `git worktree remove ../sky-prototype` refuses dirty worktrees by default. Do not force removal.

## Undo a change on shared main

Preserve history with a revert PR rather than rewriting main:

```sh
git status --short
# Start only after committing or otherwise preserving unrelated changes.
git switch main
git pull --ff-only
git switch -c fix/revert-regression
git log --oneline -10
# Substitute the reviewed commit SHA; this creates a new inverse commit.
git revert <commit-sha>
go test ./...
go vet ./...
```

Open a PR for the revert, review it, and merge after checks pass. If subsequent changes depend on that commit, review the combined revert rather than blindly reverting multiple commits. Merge commits may need a mainline choice; prefer normal squash commits for simple rollback.

Do not use `git reset --hard` or force-push on shared main as the normal recovery procedure. Do not assume a tag captures uncommitted files.

## Recover the application binary

Until release binaries exist, build the desired tested tag in a separate worktree. Once releases exist, install the previous verified release and check its checksum. Keep the current binary until the replacement passes a smoke check.

An older binary may not understand a newer database. Code rollback and data rollback are separate operations.

## Watchlist data recovery

Issue #6 introduces schema-versioned SQLite watchlists. See [WATCHLISTS.md](WATCHLISTS.md) for paths, backup commands, restore and compatibility details.

1. Run `sky watch backup /path/to/new-backup.db`; use a new filename. The command uses SQLite's consistent snapshot mechanism, including committed WAL data, validates the result and syncs the copy.
2. Keep the backup outside the repository, with a separate durable copy where needed. Git tags do not contain your database or credentials.
3. Restore with `sky watch restore /path/to/backup.db --data-dir /path/to/new-directory`.
4. Verify with `sky watch list --data-dir /path/to/new-directory --json` before selecting it through `--data-dir` or `SKY_DATA_DIR`.
5. Keep the original watchlist. The command refuses an existing destination database/sidecar instead of overwriting it. Restoring an older snapshot loses changes made after that snapshot in the recovered copy.
6. Use a binary that supports the backup's schema. Unknown applications and unsupported versions are refused; no silent downgrade is attempted. Future migrations must add pre-migration backups and their own compatibility tests.

Do not copy only the main `.db` file from a running WAL database. Restore requires a hard-link-capable filesystem for atomic no-overwrite publication. A killed restore can leave a hidden `.sky-restore-*.db` staging file; it does not replace your active database. Inspect any leftover staging files before removing them.

## Candidate validation and schema-1 compatibility

The source candidate `v0.2.0-rc.1` uses schema 1. Its [release validator](RELEASE_VALIDATION.md) builds clean source snapshots, exercises real command processes and verifies two-way reads/writes and recovery with the earlier `checkpoint/watchlists-review-2026-11-17`. Use the [candidate notes](releases/v0.2.0-rc.1.md) for limits and install instructions once its tag is published.

This compatibility evidence applies to the recorded checkpoint and schema, not every past/future binary. It does not replace backups. Earlier checkpoints may lack `watch run`; preserve the current binary and check old command support before downgrading. Restore into a new directory and inspect the saved targets before switching.

## Release checklist

- [ ] Acceptance criteria met; changelog and STATUS updated.
- [ ] Formatting, tests/race checks, vet, and supported builds pass.
- [ ] API/JSON and migration compatibility reviewed.
- [ ] Backup/restore and downgrade procedure exercised for data changes.
- [ ] Version metadata matches the intended tag; binary smoke checks pass.
- [ ] Release notes name limitations and the prior compatible version.
- [ ] Annotated tested candidate/release tag pushed; publish verified checksums when binary assets are distributed (M4).
- [ ] Main and tags pushed; recovery point available off-machine.

## M2 history is independent

`watch backup/restore` handles saved targets only. Opt-in observations live in `history.db`; use `history backup/restore` for those snapshots. Back up both when both matter. Restoring either does not modify the other, and the snapshots are not one cross-database transaction. M1 binaries ignore history and keep reading unchanged schema-1 watchlists. Future history migrations require automatic verified pre-migration backups and transactional failure/recovery tests before release. See [HISTORY.md](HISTORY.md) and [ADR 0003](decisions/0003-observation-history.md).

History queries and exports do not replace SQLite backups. Before `history prune --apply`, create a uniquely named `history backup` and verify it. Preview/apply counts can differ if another process records/prunes between the two commands. To undo deliberate pruning, restore the compatible snapshot into a destination with no existing history; saved watchlist targets are independent. See HISTORY.md for exact page and pruning semantics.

The M2 validator now exercises both-way history snapshots/read-write compatibility against PR #23 commit `97d6db1`, post-prune restore, and independent restoration of history and watchlist databases. See RELEASE_VALIDATION.md for exact scope and report identity. These are unreleased compatible schema-1 checkpoints, not a guarantee for every binary or future schema.
