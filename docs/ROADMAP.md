# Sky roadmap

Status: draft delivery plan, grounded in the working prototype. Updated 2026-11-15.

## Product goal

A dependable terminal tool for finding aircraft, following saved aircraft and airline activity, and understanding dated flights. It should work locally, produce useful human and script output, and make data freshness and provider cost visible.

The first usable product is a CLI. A hosted service becomes a separate decision only when continuous alerts, shared credentials, or multiple users need it. Putting the source on GitHub does not require hosting a backend.

## Where we are

The prototype implements registration and ICAO hex lookup, broadcast callsign lookup, nearby search, regional operator-prefix filtering, table/JSON output, timeout/cancellation, and basic upstream error handling. Unit and local HTTP integration tests exist. Live smoke checks succeeded on 2026-11-15; they do not guarantee continuous provider coverage.

Not implemented: saved watchlists, recurring polling, caching, historical storage, complete airline fleets, scheduled flight lookup, notifications, or installable releases. The hardcoded CLI version is 0.1.0; no versioned public release has been made.

Prototype checkpoint: `checkpoint/prototype-2026-11-15`, commit `b0ff308`.

## Delivery sequence

Versions below are proposed targets, not released artifacts. Complete one milestone before expanding the next. Use small issue-sized PRs within each milestone; do not put a whole milestone in one PR. Effort describes relative scope, not promised dates.

| Milestone | Target | User outcome | Effort | Depends on |
| --- | --- | --- | --- | --- |
| M0: Project foundation | 0.1.1 | Reproducible development, visible progress, recoverable changes | Small | Prototype |
| M1: Saved tracking | 0.2.0 | Save aircraft and follow observations without uncontrolled polling | Medium | M0 |
| M2: Local history | 0.3.0 | Inspect and export observations collected on this machine | Medium | M1 |
| M3: Airline and flight context | 0.4.0 | Resolve dated commercial flights and richer operator context | Large; provider-dependent | M2, provider decision |
| M4: Distribution and terminal experience | 0.5.0 | Install supported binaries and use tracking comfortably | Medium | M1–M3 |
| M5: Stable release | 1.0.0 | Rely on documented commands, data contracts, and upgrades | Medium | M4 |

### M0 — Project foundation

Deliver:

- Git repository with prototype checkpoint, plan, decision log, changelog, and recovery instructions.
- Public GitHub remote, MIT licensing, contributor guidance, issue backlog, and milestones; use a project board when permissions allow.
- CI for formatting, tests, race detection, vet, and compilation; automatic checks on PRs.
- Small PR workflow with explicit acceptance criteria and validation evidence.
- Defined output/error contract, provider boundary, and fixture coverage for current commands.

Exit criteria:

- A fresh checkout builds and passes checks with the documented Go versions.
- CI is green on GitHub, the initial issues are linked to milestones, and main requires checks where the account supports it.
- The prototype can be checked out and built in a separate worktree using the recovery guide.
- Current behavior is documented, including regional airline filtering and missing data.

Current status: public repository, MIT licensing, contributor guidance, pushed checkpoints, hosted CI, initial issues, labels and all six milestones are set up. Owner-only branch protection/private-reporting settings remain in #4; contract hardening remains in #5. Projects is optional and currently permission-blocked.

### M1 — Saved tracking

Proposed commands:

```sh
sky watch add --registration G-VBEL
sky watch add --hex 4073ce
sky watch list
sky watch remove --registration G-VBEL
sky watch run --interval 5m --max-requests 100
```

Deliver:

- An application config/data directory with platform-appropriate paths and an override for tests and automation.
- SQLite storage for watchlist targets, uniqueness rules, schema versioning, and backup support.
- One polling scheduler that deduplicates targets and enforces a user-selected request budget.
- Explicit accounting for how many upstream calls each cycle needs; pause when the budget is exhausted.
- Handling of Retry-After in both seconds and HTTP-date form, bounded backoff, interrupt cancellation, and transient failures.
- Change events based on observations; missing coverage must not generate a definitive landed event.
- A documented freshness policy for new, stale, and missing observations.

Exit criteria:

- Watchlists survive restarts; duplicate targets cannot create duplicate scheduled requests.
- A fake clock and local provider prove polling stays within budget, respects 429 cooldowns, and stops cleanly.
- Interrupted writes preserve the last valid watchlist; backup and recovery are exercised.
- `watch run` clearly states that tracking stops when the process exits.

### M2 — Local history

Proposed commands:

```sh
sky history --registration G-VBEL --since 24h
sky history --registration G-VBEL --since 24h --json
sky export --registration G-VBEL --format csv
```

Deliver:

- Timestamped observations with provider identity, provider snapshot time, retrieval time, and aircraft identifiers.
- Dedupe rules for repeated snapshots, indexed time queries, configurable retention, and disk usage visibility.
- Date range filtering and CSV/JSON export with documented units and timezone behavior.
- Explicit distinction between locally collected history and provider historical data.
- Versioned, transactional migrations; automatic pre-migration backups and a documented restore path.

Exit criteria:

- Replaying the same fixture does not create unintended duplicate observations.
- History can be queried offline and exports match stored observations.
- Retention and migration failure/recovery are tested against a real temporary SQLite database.
- A prior compatible release can use restored pre-migration data without schema errors.

### M3 — Airline and flight context

Proposed commands (provider capabilities must be confirmed first):

```sh
sky flight UA123 --date 2026-11-15
sky airline UAL --airport LHR
```

Deliver:

- A written provider decision covering available fields, sample coverage, request pricing, quotas, permitted storage, attribution, and redistribution.
- A provider-neutral aircraft/flight model; isolate provider-specific fields inside adapters.
- IATA/ICAO airline code resolution, dated flight identity, explicit operating/marketing carrier handling, and timezone-aware dates.
- Provider-supported scheduled/estimated/actual times and status; missing values stay unknown.
- If an authoritative fleet dataset is available, expose fleet membership separately from observed callsign activity.

Exit criteria:

- A confirmed provider and budget support the intended functionality; unknown pricing or rights block this integration, not M0–M2.
- Fixtures cover flight-number reuse across dates, codeshares, aircraft changes, missing registration, and conflicting timestamps.
- Existing open-data commands continue to work without the commercial provider key.
- Documentation distinguishes observed position, scheduled flight status, and fleet membership.

### M4 — Distribution and terminal experience

Deliver:

- Release binaries and SHA-256 checksums for Linux/macOS/Windows on supported architectures.
- A selected SQLite implementation and verified cross-platform build story before promising binaries.
- Build-time version metadata, shell completion documentation, installation and upgrade instructions.
- Optional interactive watch screen that uses the same scheduler and budget as noninteractive commands.
- Accessible plain-text output, predictable noninteractive behavior, and no secrets in logs or release assets.

Exit criteria:

- Each advertised platform is built automatically; available runners smoke-test binaries and persistence.
- Release version matches `sky --version`; checksums and asset names are correct.
- The prior release remains available, and downgrade/restore instructions are exercised.
- A user can install and complete an aircraft lookup using only the README.

### M5 — Stable 1.0

Deliver:

- A supported JSON schema and documented exit codes, command compatibility, and deprecation policy.
- Tested install, upgrade, backup, restore, and downgrade paths from supported prior versions.
- Operational guidance for quotas, retention, provider outages, credentials, and coverage limitations.
- A reviewed release checklist and no unresolved release-blocking issues.

Exit criteria:

- M0–M4 acceptance criteria are satisfied; any intentionally deferred capability is clearly marked.
- Critical flows pass offline tests; optional live provider checks have recorded dates and results.
- Recovery from a failed code release and a failed database migration has been demonstrated.
- No known data-loss, credential-leakage, or budget-overrun defect remains open.

## What we defer

A web dashboard, hosted multi-user service, mobile app, raw ADS-B decoding, full global fleet ingestion, and external notification integrations are outside the first delivery sequence. Reconsider only after the CLI tracking workflow works well and there is a concrete use case.

## Planning rules

1. Acceptance criteria determine completion; writing code alone does not close a milestone.
2. `docs/STATUS.md` records the current focus and next task. Issues track individual changes once GitHub is set up.
3. Record architectural decisions under `docs/decisions/` before changes with broad compatibility or data implications.
4. Every completed issue includes evidence and a rollback note. Data migrations need backups before release.
5. Tags identify tested checkpoints. GitHub is the off-machine copy of code; local databases need a separate backup process.
6. Set calendar estimates after the M1 storage/scheduler design and the M3 provider evaluation, rather than inventing dates now.
