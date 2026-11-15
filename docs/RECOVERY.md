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

## Data recovery (required before M1 ships)

Today the CLI has no persistent application database. Before introducing one:

1. Stop the tracking process before filesystem-level backups or restoration, or use SQLite's supported online backup mechanism.
2. Create a verified pre-migration backup with the schema/app versions recorded; do not copy only the main `.db` file from a running WAL database.
3. Run migrations transactionally where possible. Do not silently apply a destructive downgrade.
4. On failure, preserve the failed database for diagnosis, restore a compatible backup to a separate location, and validate it before replacing the active copy.
5. State that restoring an older backup loses observations collected after that backup. Export what can be preserved first.
6. Verify these steps against the actual chosen driver and schema before marking persistence complete.

Backups must live outside the Git repository and should have a separate durable copy if the data matters. Never commit credentials or database contents as a substitute for backups.

## Release checklist

- [ ] Acceptance criteria met; changelog and STATUS updated.
- [ ] Formatting, tests/race checks, vet, and supported builds pass.
- [ ] API/JSON and migration compatibility reviewed.
- [ ] Backup/restore and downgrade procedure exercised for data changes.
- [ ] Version metadata matches the intended tag; binary smoke checks pass.
- [ ] Release notes name limitations and the prior compatible version.
- [ ] Annotated release tag and checksum-verified assets published.
- [ ] Main and tags pushed; recovery point available off-machine.
