# Saved watchlists

Watchlist management is implemented by issue #6. These commands save targets locally and do not query a flight-data provider. Automatic polling and `watch run` belong to issues #7–#8 and are not available yet.

## Add, list and remove

```sh
sky watch add --registration G-ZBKE
sky watch add --hex 406d7b
sky watch list
sky watch list --json
sky watch remove --registration G-ZBKE
```

Replace `sky` with `./bin/sky` if running from a source checkout. On Windows, use the appropriate `sky.exe` path.

Add/remove require exactly one of `--registration` or `--hex`. Empty/invalid identifiers are rejected before opening the database. Registration is uppercased and hex is lowercased; leading/trailing whitespace is removed. Duplicate adds keep the original creation timestamp. Removing a target that is not saved is a successful no-op.

A registration and hex address for the same aircraft are separate saved targets at this stage. No online lookup or implicit association is performed. Broadcast callsigns are not currently supported as watchlist targets.

## Where data lives

The precedence and platform defaults are recorded in [ADR 0002](decisions/0002-watchlist-sqlite.md). Output includes the resolved database path. On macOS, the default is:

```text
~/Library/Application Support/sky/watchlist.db
```

Choose another directory per command:

```sh
sky watch --data-dir ./local-state add --registration G-ZBKE
sky watch --data-dir ./local-state list
```

Or set `SKY_DATA_DIR` for your session:

```sh
export SKY_DATA_DIR="$HOME/.local/share/sky-custom"
```

PowerShell equivalent:

```powershell
$env:SKY_DATA_DIR = Join-Path $HOME "sky-custom"
```

An explicit `--data-dir` wins over the environment variable. Relative paths resolve from the current working directory. Explicit blank directory flags are invalid. List on a fresh directory initializes an empty watchlist. API credentials are not stored in this database.

## Back up

Choose a filename that does not already exist:

```sh
sky watch backup "$HOME/sky-backups/watchlist-2026-11-17.db"
```

The parent directory is created if needed. SQLite makes a consistent snapshot, including committed WAL data if the active database uses WAL. Sky validates the copy and syncs the output file before reporting success. Existing files are never overwritten. Keep a separate durable copy outside the repository if the watchlist matters; local files alone do not protect against machine loss.

## Restore without replacing current data

Restore into a new directory, then inspect it:

```sh
sky watch restore "$HOME/sky-backups/watchlist-2026-11-17.db" \
  --data-dir "$HOME/sky-recovered"

sky watch list --data-dir "$HOME/sky-recovered" --json
```

After verifying the list, select that directory on subsequent commands, or set `SKY_DATA_DIR` to it. Keep the original directory until satisfied.

Restore refuses an existing `watchlist.db` or its `-wal`, `-shm`, or `-journal` sidecars. It validates the source, imports saved targets into a staged database in one transaction, and publishes without overwriting another file. The destination filesystem must support hard links; an unsupported filesystem fails safely. Do not modify the backup while recovery is running.

The backup schema must be supported by the current binary. Unknown/newer schemas are refused. Restoration preserves saved target timestamps but excludes changes made after the snapshot. This version stores watch targets only, not aircraft observation history. Future database versions will need their own migration/recovery compatibility plan.

## JSON and errors

List writes one JSON object; `targets` is always an array, including when empty:

```json
{
  "database": "/example/sky/watchlist.db",
  "targets": [
    {
      "kind": "registration",
      "identifier": "G-ZBKE",
      "created_at": "2026-11-17T12:00:00Z"
    }
  ]
}
```

Targets are ordered by kind then identifier. `created_at` is UTC RFC 3339, with optional fractional seconds.

- Add: `database`, `target`, and boolean `changed`.
- Remove: `database`, `kind`, `identifier`, and boolean `changed`.
- Backup: `backup`, the absolute output path.
- Restore: `database` and integer `count`.

Success/no-op returns exit 0. Invalid invocation/target returns 2. Storage, schema, backup/restore and output failures return 1. Preserved context cancellation/deadline errors use 130/124 as described in the [CLI contract](CLI_CONTRACT.md). Provider error codes do not apply because these commands make no provider calls. Diagnostics go to stderr; successful JSON goes to stdout. An output failure after a committed mutation does not undo that mutation; rerunning add/remove is safe because they are idempotent.
