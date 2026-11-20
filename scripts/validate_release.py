#!/usr/bin/env python3
"""Validate committed source and schema-1 recovery without live provider calls."""
import argparse
import datetime
import io
import json
import os
from pathlib import Path, PurePosixPath
import platform
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile


def run(args, cwd, env=None, expected=0):
    result = subprocess.run(
        [str(arg) for arg in args], cwd=cwd, env=env,
        text=True, capture_output=True, timeout=300,
    )
    if result.returncode != expected:
        raise RuntimeError(
            f"{args[0]} exited {result.returncode}, expected {expected}\n"
            f"{result.stdout}\n{result.stderr}"
        )
    return result


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def archive(repo, commit, destination):
    data = subprocess.check_output(
        ["git", "archive", "--format=tar", commit], cwd=repo, timeout=60,
    )
    destination.mkdir()
    # Portable across Python 3.9+: reject links/special members and traversal.
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:") as source:
        for member in source:
            path = PurePosixPath(member.name)
            require(not path.is_absolute() and ".." not in path.parts,
                    "Unsafe archive member")
            target = destination.joinpath(*path.parts)
            require(target.resolve().is_relative_to(destination.resolve()),
                    "Archive member escapes its destination")
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
            elif member.isfile():
                target.parent.mkdir(parents=True, exist_ok=True)
                with source.extractfile(member) as incoming, target.open("wb") as outgoing:
                    shutil.copyfileobj(incoming, outgoing)
                target.chmod(member.mode & 0o777)
            else:
                raise RuntimeError("Release archive contains an unsupported link/special file")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", default=shutil.which("go"))
    parser.add_argument("--candidate-ref", default="HEAD")
    parser.add_argument("--baseline-ref", default="checkpoint/watchlists-review-2026-11-17")
    parser.add_argument("--report", type=Path)
    args = parser.parse_args()
    require(args.go is not None, "Go 1.24+ is required; pass --go /path/to/go")
    repo = Path(__file__).resolve().parents[1]
    compiler = str(Path(args.go).resolve())
    go_version = run([compiler, "version"], repo).stdout.strip()
    match = re.search(r"go version go(\d+)\.(\d+)", go_version)
    require(match is not None and tuple(map(int, match.groups())) >= (1, 24),
            "The selected executable must be Go 1.24 or newer")
    # Source archives intentionally have no repository metadata. Do not let Go
    # discover an unrelated/inaccessible parent .git directory when building.
    os.environ["GOFLAGS"] = (os.environ.get("GOFLAGS", "") + " -buildvcs=false").strip()
    commits = {}
    for name, ref in (("candidate", args.candidate_ref), ("baseline", args.baseline_ref)):
        commit = run(["git", "rev-parse", "--verify", ref + "^{commit}"], repo).stdout.strip()
        require(re.fullmatch(r"[0-9a-f]{40,64}", commit), "Invalid source commit")
        commits[name] = commit
    report = {
        "validated_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "candidate_ref": args.candidate_ref, "candidate_commit": commits["candidate"],
        "baseline_ref": args.baseline_ref, "baseline_commit": commits["baseline"],
        "go": go_version, "platform": platform.system(), "architecture": platform.machine(),
        "checks": [],
        "limits": [
            "Native runtime validation applies only to the recorded platform.",
            "Polling/errors/cancellation use local fixtures in the Go tests, not live provider quota.",
            "Compatibility is proven against the named schema-1 checkpoint, not every past/future binary.",
            "The baseline is an unreleased source checkpoint, not a previous stable release.",
        ],
    }

    def passed(name):
        report["checks"].append(name)
        print("PASS:", name, flush=True)

    with tempfile.TemporaryDirectory(prefix="sky-release-") as temporary:
        root = Path(temporary)
        candidate, baseline = root / "candidate", root / "baseline"
        archive(repo, commits["candidate"], candidate)
        archive(repo, commits["baseline"], baseline)
        version_text = (candidate / "internal/version/version.go").read_text()
        version_match = re.search(r'const Version = "([^"]+)"', version_text)
        require(version_match is not None, "Candidate lacks a canonical version")
        version = version_match.group(1)
        report["version"] = version
        if args.candidate_ref.startswith("v"):
            require(args.candidate_ref == "v" + version, "Candidate tag/version mismatch")
        run([compiler, "test", "-race", "./..."], candidate)
        passed("clean candidate: race suite, fixture polling/errors/quotas and interrupt handling")
        run([compiler, "vet", "./..."], candidate)
        passed("clean candidate: go vet")
        env = os.environ.copy()
        env["CGO_ENABLED"] = "0"
        suffix = ".exe" if os.name == "nt" else ""
        current, previous = root / ("sky-current" + suffix), root / ("sky-previous" + suffix)
        run([compiler, "build", "-o", current, "./cmd/sky"], candidate, env=env)
        run([compiler, "build", "-o", previous, "./cmd/sky"], baseline, env=env)
        passed("CGO-disabled candidate and baseline binaries from clean source archives")
        output = run([current, "--version"], root).stdout.strip()
        require(output == "sky version " + version, "Built CLI has incorrect release version")
        run([current, "watch", "run", "--help"], root)
        passed("candidate version and command entrypoint")

        def cli(binary, directory, *arguments, expected=0):
            result = run([binary, "--json", "watch", "--data-dir", directory, *arguments],
                         root, expected=expected)
            if expected:
                require(not result.stdout and result.stderr.startswith("sky:"),
                        "Failed command did not keep diagnostic output separate")
                return None
            require(not result.stderr, "Successful command wrote a diagnostic")
            return json.loads(result.stdout)

        data, recovered = root / "state", root / "recovered"
        first = cli(current, data, "add", "--registration", "g-test")
        require(first["changed"], "Initial target was not saved")
        duplicate = cli(current, data, "add", "--registration", "G-TEST")
        require(not duplicate["changed"] and duplicate["target"] == first["target"],
                "Duplicate target changed its identity or creation time")
        cli(current, data, "add", "--hex", "ABC123")
        saved = cli(current, data, "list")["targets"]
        require(len(saved) == 2, "Targets did not persist across processes")
        snapshot = root / "snapshot.db"
        cli(current, data, "backup", snapshot)
        require(cli(current, data, "remove", "--registration", "G-TEST")["changed"],
                "Removal was not persisted")
        require(not cli(current, data, "remove", "--registration", "G-TEST")["changed"],
                "Missing removal was not idempotent")
        require(cli(current, recovered, "restore", snapshot)["count"] == 2,
                "Recovery lost targets")
        require(cli(current, recovered, "list")["targets"] == saved,
                "Recovery changed saved values or timestamps")
        passed("real binary: add, duplicate, restart/list, backup, remove and restore")

        before = cli(current, data, "list")["targets"]
        cli(current, data, "restore", snapshot, expected=1)
        cli(current, data, "backup", snapshot, expected=1)
        require(cli(current, data, "list")["targets"] == before,
                "Rejected recovery changed the active database")
        empty_run = run([current, "--json", "watch", "run", "--data-dir", root / "empty",
                         "--max-requests", "1"], root, expected=2)
        require(not empty_run.stdout, "Empty watchlist emitted a live event stream")
        passed("no-overwrite recovery and empty-watchlist guard")

        require(cli(previous, recovered, "list")["targets"] == saved,
                "Schema-1 checkpoint cannot read candidate data")
        cli(previous, recovered, "add", "--registration", "N123AB")
        require(len(cli(current, recovered, "list")["targets"]) == 3,
                "Candidate cannot reopen baseline-written data")
        old_recovery = root / "old-recovery"
        cli(previous, old_recovery, "restore", snapshot)
        require(cli(current, old_recovery, "list")["targets"] == saved,
                "Candidate cannot read a baseline-restored candidate backup")
        old_backup = root / "old-snapshot.db"
        cli(previous, old_recovery, "backup", old_backup)
        new_recovery = root / "new-recovery"
        cli(current, new_recovery, "restore", old_backup)
        require(cli(current, new_recovery, "list")["targets"] == saved,
                "Candidate cannot restore a baseline-generated snapshot")
        passed("schema-1 downgrade/re-upgrade reads/writes and two-way snapshot recovery")
    report["status"] = "passed"
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(report, indent=2) + "\n")
        print("Report:", args.report, flush=True)
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, subprocess.SubprocessError, OSError) as error:
        print("Release validation failed:", error, file=sys.stderr)
        sys.exit(1)
