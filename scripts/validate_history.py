"""M2 native-binary acceptance on synthetic data; no provider calls."""
import csv
from contextlib import closing
import hashlib
import io
import json
from pathlib import Path
import sqlite3


def validate_history(root, candidate, baseline, compiler, env, current, previous,
                     run, require, passed, report):
    # Production sources/binaries have already passed clean-archive checks.
    # The helper invokes each revision's real history.Record API. Its source is
    # recorded by hash and is compiled only into separate temporary writers.
    fixture = (candidate / "scripts/history_fixture.go.txt").read_bytes()
    report["history_fixture_sha256"] = hashlib.sha256(fixture).hexdigest()
    writers = []
    for source, label in ((candidate, "candidate"), (baseline, "baseline")):
        helper = source / "cmd/sky-validation-fixture"
        require(not helper.exists(), "Fixture directory unexpectedly exists")
        helper.mkdir()
        (helper / "main.go").write_bytes(fixture)
        output = root / ("history-writer-" + label + (".exe" if current.suffix == ".exe" else ""))
        run([compiler, "build", "-o", output, "./cmd/sky-validation-fixture"], source, env=env)
        writers.append(output)

    def cli(binary, directory, *arguments, expected=0):
        result = run([binary, "--json", "history", "--data-dir", directory, *arguments],
                     root, expected=expected)
        if expected:
            require(not result.stdout and result.stderr.startswith("sky:"),
                    "History failure polluted stdout or lost its diagnostic")
            return None
        require(not result.stderr, "Successful history command wrote a diagnostic")
        return json.loads(result.stdout)

    def write(writer, directory, stamp, target, changed=True):
        result = run([writer, directory, stamp, target], root)
        require(not result.stderr and json.loads(result.stdout)["changed"] == changed,
                "History fixture write/replay has unexpected outcome")

    data = root / "history-state"
    first, second, third = ("2026-11-01T00:00:00Z", "2026-11-01T01:00:00Z", "2026-11-01T02:00:00Z")
    write(writers[0], data, first, "G-TEST")
    require(cli(previous, data, "status")["count"] == 1,
            "Baseline cannot read candidate history")
    write(writers[1], data, second, "G-TEST")
    write(writers[0], data, third, "hex:ABC123")
    write(writers[1], data, first, "G-TEST", changed=False)
    original = cli(current, data)["records"]
    require(len(original) == 3 and cli(previous, data, "status")["count"] == 3,
            "Candidate/baseline writes or persistent replay dedupe lost data")
    passed("history: native candidate/baseline reads/writes and cross-version replay dedupe")

    gathered, offset = [], 0
    while True:
        page = cli(current, data, "--limit", "1", "--offset", str(offset))
        gathered.extend(page["records"])
        if not page["has_more"]:
            break
        offset = page["next_offset"]
    require(gathered == original, "History pages do not reconstruct stored observations")
    bounded = cli(current, data, "--registration", "g-test", "--since", first, "--until", second)
    require(len(bounded["records"]) == 1 and bounded["records"][0] == original[0],
            "UTC encounter bounds or registration normalization differ")
    require(cli(current, data, "--hex", "abc123")["records"] == original[2:],
            "Hex aliases leaked into registration filtering")
    require(cli(current, data, "export", "--format", "json")["records"] == original,
            "JSON export differs from stored observations")
    output = run([current, "history", "export", "--data-dir", data, "--format", "csv"], root)
    require(not output.stderr, "CSV export wrote a diagnostic")
    rows = list(csv.DictReader(io.StringIO(output.stdout)))
    require(len(rows) == len(original), "CSV export row count differs")
    for row, record in zip(rows, original):
        observation = record["observation"]
        result = observation["result"]
        require(row["key"] == record["key"] and row["encounter_at_utc"] == observation["at"]
                and row["snapshot_at_utc"] == result["snapshot_at"]
                and row["retrieved_at_utc"] == result["retrieved_at"]
                and row["state"] == observation["state"]
                and json.loads(row["aircraft_json"]) == result["aircraft"]
                and row["source"] == "adsb.lol" and row["source_license"] == "ODbL 1.0"
                and row["collection"] == "local" and row["page_has_more"] == "false",
                "CSV changed aircraft units/missing values, timestamps, attribution or completeness")
    passed("history: native query pagination, UTC/target filters and JSON/CSV parity/attribution")

    snapshot, old_snapshot = root / "history-new.db", root / "history-old.db"
    cli(current, data, "backup", snapshot)
    cli(previous, data, "backup", old_snapshot)
    restored_old, restored_new = root / "history-restored-old", root / "history-restored-new"
    require(cli(previous, restored_old, "restore", snapshot)["count"] == 3,
            "Baseline cannot restore candidate history backup")
    require(cli(current, restored_old)["records"] == original,
            "Baseline restore changed candidate observations")
    require(cli(current, restored_new, "restore", old_snapshot)["count"] == 3,
            "Candidate cannot restore baseline history backup")
    require(cli(current, restored_new)["records"] == original,
            "Candidate restore changed baseline observations")
    cli(current, data, "backup", snapshot, expected=1)
    cli(current, data, "restore", snapshot, expected=1)
    require(cli(current, data)["records"] == original, "Rejected recovery changed history")
    passed("history: verified two-way snapshots preserve payloads/keys; overwrite refusals preserve active data")

    # Preserve an independently saved watchlist while pruning/restoring history.
    run([current, "watch", "add", "--registration", "G-SAVED", "--data-dir", data], root)
    preview = cli(current, data, "prune", "--before", second)
    require(preview["matched"] == 1 and preview["deleted"] == 0 and not preview["applied"],
            "Prune preview mutated history")
    require(cli(current, data)["records"] == original, "Preview changed observations")
    applied = cli(current, data, "prune", "--before", second, "--apply")
    require(applied["deleted"] == 1 and applied["matched"] == 1 and applied["applied"],
            "Applied prune did not report actual deletion")
    require(cli(current, data)["records"] == original[1:], "Prune boundary or persistence differs")
    recovered = root / "history-after-prune"
    run([current, "watch", "add", "--registration", "G-SAVED", "--data-dir", recovered], root)
    cli(current, recovered, "restore", snapshot)
    require(cli(current, recovered)["records"] == original, "Pre-prune snapshot did not recover rows")
    watch = json.loads(run([current, "--json", "watch", "list", "--data-dir", recovered], root).stdout)
    require(len(watch["targets"]) == 1 and watch["targets"][0]["identifier"] == "G-SAVED",
            "History restore changed saved targets")
    watch_snapshot = root / "independent-watch.db"
    run([current, "watch", "backup", watch_snapshot, "--data-dir", data], root)
    run([current, "watch", "restore", watch_snapshot, "--data-dir", restored_new], root)
    require(cli(current, restored_new)["records"] == original, "Watchlist restore changed history")
    passed("history: prune preview/apply, strict boundary, post-prune recovery and independent watchlist restore")

    # Mutate temporary copies only, retaining bytes before rejected opens.
    for label, mutation in (("unsupported", "PRAGMA user_version=99"),
                            ("unrelated", "PRAGMA application_id=3")):
        directory = root / ("history-" + label)
        cli(current, directory, "restore", snapshot)
        path = directory / "history.db"
        with closing(sqlite3.connect(path)) as db:
            db.execute(mutation)
            db.commit()
        before = path.read_bytes()
        cli(current, directory, "status", expected=1)
        cli(previous, directory, "status", expected=1)
        require(path.read_bytes() == before, "Rejected schema open changed database bytes")
    corrupt = root / "history-corrupt.db"
    # Materialize with SQLite's backup API, never copy a live main file.
    with closing(sqlite3.connect(snapshot)) as source, closing(sqlite3.connect(corrupt)) as destination:
        source.backup(destination)
        destination.execute("UPDATE observations SET payload='{' ")
        destination.commit()
    refused = root / "history-refused"
    cli(current, refused, "restore", corrupt, expected=1)
    require(not (refused / "history.db").exists(), "Corrupt backup was published")
    require(cli(current, data)["records"] == original[1:], "Invalid recovery changed active history")
    passed("history: unsupported schemas are not mutated; corrupt recovery is not published")
