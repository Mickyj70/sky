"""Smoke-test a native Sky binary with temporary offline data."""

import json
from pathlib import Path
import subprocess
import sys
import tempfile


def smoke(binary, root):
    def run(*args):
        return subprocess.run(
            [str(binary), *map(str, args)], check=True,
            capture_output=True, text=True, timeout=30,
        ).stdout

    def cli(group, directory, *args):
        return json.loads(run(group, "--data-dir", directory, "--json", *args))

    version = run("--version").strip()
    active, recovered = root / "active data", root / "recovered data"
    watch_backup, history_backup = root / "watch backup.db", root / "history backup.db"
    target = cli("watch", active, "add", "--registration", "G-TEST")["target"]
    cli("watch", active, "backup", watch_backup)
    cli("watch", recovered, "restore", watch_backup)
    cli("history", active, "backup", history_backup)
    cli("history", recovered, "restore", history_backup)
    if cli("watch", recovered, "list")["targets"] != [target]:
        raise RuntimeError("Restored watchlist differs from the saved target")
    if cli("history", recovered)["records"] != []:
        raise RuntimeError("Restored empty history returned records")
    print(f"{version} passed native SQLite recovery smoke checks")


if __name__ == "__main__":
    with tempfile.TemporaryDirectory(prefix="sky-native-") as directory:
        smoke(Path(sys.argv[1]).resolve(strict=True), Path(directory))
