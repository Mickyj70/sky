import hashlib
import json
import sys
import zipfile
from pathlib import Path

root = Path(sys.argv[1])
revision = sys.argv[2]
tag = sys.argv[3]
version = tag.removeprefix("v")
for platform in ("linux-amd64", "darwin-arm64", "windows-amd64"):
    name = f"sky-{tag}-{platform}-{revision[:12]}.zip"
    archive = root / f"sky-package-{platform}" / name
    sidecar = archive.with_suffix(".zip.sha256")
    expected = f"{hashlib.sha256(archive.read_bytes()).hexdigest()}  {name}"
    if sidecar.read_text().strip() != expected:
        raise SystemExit(f"Incorrect checksum for {name}")
    with zipfile.ZipFile(archive) as bundle:
        metadata = json.loads(bundle.read("build.json"))
        for key, value in {"version": version, "target": platform, "revision": revision}.items():
            if metadata[key] != value:
                raise SystemExit(f"Incorrect {key} for {name}")
        expected_files = {"LICENSE", "THIRD_PARTY_NOTICES.md", "sky.exe" if platform.startswith("windows") else "sky"}
        if not expected_files.issubset(bundle.namelist()) or bundle.testzip() is not None:
            raise SystemExit(f"Invalid archive {name}")
    print(f"Verified {name}")
