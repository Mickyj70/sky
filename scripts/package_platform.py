"""Package and verify one native Sky build in the current checkout."""
import argparse
import hashlib
import json
import re
from pathlib import Path
import subprocess
import tempfile
import zipfile

from smoke_platform import smoke


def command(*args):
    return subprocess.check_output(args, text=True).strip()


def package(binary, output):
    version = command(str(binary), "--version").removeprefix("sky version ")
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?", version):
        raise ValueError("Unexpected binary version")
    revision = command("git", "rev-parse", "HEAD")
    target = "-".join(command("go", "env", "GOOS", "GOARCH").splitlines())
    output.mkdir(parents=True, exist_ok=True)
    archive = output / f"sky-v{version}-{target}-{revision[:12]}.zip"
    modules = command("go", "list", "-deps", "-f",
                      "{{with .Module}}{{if not .Main}}{{.Path}} {{.Dir}}{{end}}{{end}}",
                      "./cmd/sky")
    goroot = Path(command("go", "env", "GOROOT"))
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as bundle:
        bundle.write(binary, binary.name)
        for name in ("LICENSE", "THIRD_PARTY_NOTICES.md"):
            bundle.write(name)
        for name in ("LICENSE", "PATENTS"):
            bundle.write(goroot / name, f"licenses/Go/{name}")
        for entry in sorted(set(modules.splitlines()) - {""}):
            module, directory = entry.split(" ", 1)
            notices = [path for path in Path(directory).iterdir()
                       if path.is_file() and path.name.upper().startswith(
                           ("LICENSE", "COPYING", "NOTICE", "PATENTS", "COPYRIGHT"))]
            if not notices:
                raise RuntimeError(f"No license notice for {module}")
            for path in sorted(notices):
                bundle.write(path, f"licenses/{module}/{path.name}")
        bundle.writestr("build.json", json.dumps({
            "version": version, "revision": revision, "target": target,
            "go": command("go", "version"),
        }, indent=2) + "\n")

    checksum = archive.with_suffix(".zip.sha256")
    checksum.write_text(f"{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n",
                        encoding="ascii", newline="\n")
    expected, name = checksum.read_text(encoding="ascii").strip().split("  ", 1)
    if name != archive.name or expected != hashlib.sha256(archive.read_bytes()).hexdigest():
        raise RuntimeError("Archive checksum verification failed")
    with tempfile.TemporaryDirectory(prefix="sky package ") as directory:
        root = Path(directory)
        with zipfile.ZipFile(archive) as bundle:
            bundle.extractall(root)
        extracted = root / binary.name
        extracted.chmod(binary.stat().st_mode)
        if command(str(extracted), "--version") != f"sky version {version}":
            raise RuntimeError("Packaged binary version mismatch")
        smoke(extracted, root / "smoke data")
    print(f"Verified {archive.name}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    package(args.binary.resolve(strict=True), args.output)
