# Stable 1.0 release checklist

Record a commit, CI run or reproduction result for each completed item. Review this checklist against the exact release candidate, then retain its evidence with the release notes.

- [ ] Agree on the 1.x contract and audit documented flags, exit codes, JSON/NDJSON and CSV against current behavior. Run the offline fixture suite. See [compatibility policy](COMPATIBILITY.md).
- [ ] Resolve or explicitly defer remaining roadmap items. Confirm the release scope for commercial flight status and optional interactive watch. Review open issues for data loss, credential leaks and uncontrolled request spending.
- [ ] Build native archives for Linux amd64, macOS arm64 and Windows amd64. Verify each checksum, tagged revision, executable version, notices and extracted-binary smoke test. Exercise README installation on supported platforms and record any live lookup outcome separately from offline tests.
- [ ] Rehearse upgrade from the supported prior version with populated watchlist and history. Back up each database independently, then restore into a fresh directory with the prior binary and compare the recovered datasets. Record which versions were exercised. See [recovery guide](RECOVERY.md).
- [ ] Demonstrate failed-release recovery and, if a migration is included, interrupted-migration recovery. For a release with unchanged schemas, record the schema check and rollback rehearsal as the relevant evidence.
- [ ] Review operator documentation for quotas, timeouts, provider outages, history retention, pruning and backup. Confirm CI fixtures use synthetic data and check archives for unintended files.
- [ ] Prepare and validate a tagged 1.0 release candidate. The candidate publication job targets `v1.0.0-rc.1`. Verify the published assets and review the stable publication trigger separately.
- [ ] Review candidate feedback, publish stable assets and notes when acceptance is complete, and verify downloads. Preserve prior versions and their recovery instructions.

Existing evidence to carry forward includes the [v0.5.0-rc.1 tagged CI run](https://github.com/Mickyj70/sky/actions/runs/45056027376) and the [Linux install and rollback walkthrough](BINARY_INSTALL.md). Candidate-specific checks remain open until exercised against the 1.0 candidate.

See the [start-to-finish acceptance test](ACCEPTANCE_TEST.md) for the published candidate and named-place checks.
