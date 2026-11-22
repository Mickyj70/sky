# Install and verify a Sky binary

For a tagged prerelease, download the matching ZIP and SHA-256 sidecar from [GitHub Releases](https://github.com/Mickyj70/sky/releases). For a development build, download the matching artifact from a successful GitHub Actions run. The artifact contains the Sky ZIP and its `.zip.sha256` sidecar. Unpack the outer artifact first and keep both files together.

| System | Actions artifact |
| --- | --- |
| Linux x86-64 | `sky-package-linux-amd64` |
| Apple Silicon macOS | `sky-package-darwin-arm64` |
| Windows x86-64 | `sky-package-windows-amd64` |

The inner ZIP filename includes the CLI version, platform and short build revision. The full revision and Go toolchain are in `build.json`. CI artifacts expire after 30 days. Use the desired run and revision when choosing a build.

## Linux and macOS

Run these commands from the directory containing the ZIP and checksum. Replace the filename with the one you downloaded.

```sh
archive='sky-vVERSION-PLATFORM-REVISION.zip'
shasum -a 256 -c "$archive.sha256" # macOS
# On Linux use: sha256sum -c "$archive.sha256"
unzip "$archive" -d sky-install
chmod +x sky-install/sky
./sky-install/sky --version
```

Continue with extraction only after checksum verification succeeds. Keep the extracted directory with its notices intact. Add its absolute path to PATH if you want to run `sky` from other directories.

## Windows PowerShell

```powershell
$archive = 'sky-vVERSION-windows-amd64-REVISION.zip'
$expected = ((Get-Content -Raw "$archive.sha256").Trim() -split '\s+', 2)[0]
if ((Get-FileHash $archive -Algorithm SHA256).Hash -ine $expected) {
    throw 'Sky archive checksum mismatch'
}
Expand-Archive $archive -DestinationPath sky-install
.\sky-install\sky.exe --version
```

Install upgrades into a separate directory. Keep the previous binary and back up watchlist and history separately before switching your PATH. See the [recovery guide](RECOVERY.md).

## Shell completion

Use the installed executable to generate completion for your shell.

| Shell | Enable in the current session |
| --- | --- |
| Bash | `source <(sky completion bash)` |
| Zsh | `autoload -Uz compinit && compinit && source <(sky completion zsh)` |
| Fish | `sky completion fish \| source` |
| PowerShell | `sky completion powershell \| Out-String \| Invoke-Expression` |

## Rehearsed rollback

Use separate installation directories and explicit executable paths. The previous published version, `v0.3.0-rc.1`, can be built from its source tag. Stop any watch process before backing up or changing versions.

```sh
old=/absolute/path/to/prior/sky
new=/absolute/path/to/current/sky
"$old" watch --data-dir /absolute/path/to/active backup before-watch.db
"$old" history --data-dir /absolute/path/to/active backup before-history.db
"$new" watch --data-dir /absolute/path/to/active list --json
"$new" history --data-dir /absolute/path/to/active --json

# Restore to a fresh data directory if you need to roll back.
"$old" watch --data-dir /absolute/path/to/recovered restore before-watch.db
"$old" history --data-dir /absolute/path/to/recovered restore before-history.db
"$old" watch --data-dir /absolute/path/to/recovered list --json
"$old" history --data-dir /absolute/path/to/recovered --json
```

Verify both restored datasets before pointing `SKY_DATA_DIR` at the recovered directory. Keep the upgraded directory if you need data added after the snapshots.

### Acceptance evidence

On 2026-11-22, the published Linux amd64 ZIP for build `85c7974ee49f` passed SHA-256 verification and reported `0.5.0-rc.1` after extraction. The first registration lookup returned HTTP 503. A retry returned valid adsb.lol JSON with an empty aircraft array.

The previous binary was built from `v0.3.0-rc.1`. With temporary synthetic data, the new binary read the same saved target and history record, then added a target and pruned the history. The previous binary restored both pre-upgrade snapshots into a fresh directory. The restored targets and history matched the saved data. The watchlist response correctly reported the new database path.

Completion generation succeeded for all four shells. Bash syntax was checked with `bash -n`. Native archive and recovery coverage for macOS and Windows comes from the [tagged CI run](https://github.com/Mickyj70/sky/actions/runs/45056027376).
