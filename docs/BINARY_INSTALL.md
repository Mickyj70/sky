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
