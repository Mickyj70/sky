# Live watch output

Implemented by the issue #8 change, on top of the scheduler merged in PR #18. Saved-target management and backup/restore are documented in [WATCHLISTS.md](WATCHLISTS.md).

## Run a bounded session

```sh
sky watch add --registration G-ZBKE
sky watch run --interval 5m --max-requests 100
sky watch run --interval 5m --max-requests 10 --stale-after 1m --json
```

Use `./bin/sky` when running a source build. `--max-requests` is required and must be positive. Interval defaults to five minutes, position staleness to one minute, and each request uses the root `--timeout` value (15 seconds by default).

Targets are loaded once at startup, then the database connection is closed. Restart to pick up watchlist edits. Duplicate normalized targets of the same kind are deduplicated by the scheduler; a registration and hex remain separate queries even when they identify the same aircraft.

The first cycle starts immediately. Later cycles use the scheduler's minimum start-to-start interval. Attempts, including failed or canceled attempts, consume the finite **per-run** budget. A run may finish partway through a cycle. The budget is not a shared daily quota: restarting or running another process starts another allowance, and other commands/users can consume the same provider account/IP allowance. Select the budget/interval against your provider's current limits.

Tracking runs only while this foreground process runs. Ctrl-C cancels requests or waits, emits a stopped event when the output remains writable, and exits 130. There is no daemon, notification integration, or persistent observation history yet.

## States and freshness

State is evaluated at each completed lookup and timestamped with scheduler time. It is not a continuously updated screen while waiting between polls.

| State | Meaning |
| --- | --- |
| `observed` | A record has both coordinates and a known effective position age at or below the threshold |
| `stale` | The best known position age is above the threshold |
| `missing` | A successful lookup returned no aircraft records |
| `unknown` | Position freshness cannot be established, or the provider lookup failed |

Effective position age = `max(0, lookup completion time - provider snapshot time) + seen_pos`. A future snapshot is clamped to zero elapsed snapshot age to avoid negative ages. Missing snapshot time, position age, or either coordinate leaves freshness unknown. If multiple records are returned, the freshest known complete position determines the target-level state. Zero coordinates and zero ages remain valid.

Provider errors never become `missing`. Missing/stale/unknown data never implies landed, canceled, or inactive. `observed` includes provider-reported ground observations; it does not mean airborne. No old result is presented as a current success during an outage.

## Repeat suppression

The first result for each target is emitted. Further observation events require a state change or a meaningful change to aircraft fields such as position, altitude/ground, speed, track, callsign, registration, or type. Provider array order, retrieval/snapshot timestamps, `seen`, and `seen_pos` alone do not repeat an observation. Aging into a stale state still produces an event.

Identical provider failures for a target are suppressed; changed error/cooldown metadata and recovery emit new events. Progress events still report budget consumption after every completed attempt, even when its observation is suppressed.

## JSON streaming contract

`watch run --json` writes **NDJSON**: one compact JSON object per line, not a single JSON document or array. Other commands retain their existing JSON contracts. Normal structured watch events are on stdout. Startup/fatal diagnostics use stderr; stderr is empty on ordinary successful completion.

All events contain:

- `type`, UTC RFC 3339 `at` (optional fractional seconds).
- `requests_per_cycle`, `requests_remaining`, `requests_used`.

Event types:

| Type | Additional content |
| --- | --- |
| `started` | `interval_seconds` and `stale_after_seconds`; no network request has been made yet |
| `observation` | `observation`: target kind/identifier, state, optional position age, and the existing provider result shape |
| `provider_error` | `observation` with state `unknown` and `failure` kind/message/status/retryability/optional Retry-After deadline |
| `progress` | Updated request accounting after a completed attempt |
| `stopped` | `reason`: `budget_exhausted`, `canceled`, `deadline`, `provider_error`, or `error` |

Example event sequence: started → observation → progress → progress → stopped. A suppressed repeat produces progress but no additional observation. A canceled in-flight attempt is still counted in the stopped event, even if no completed observation/progress was produced for it.

A failure before startup (for example invalid flags, an empty watchlist, database failure or an already-canceled context) may produce no event stream. An output failure can leave a partial stream and prevents further requests; a final stopped event cannot be guaranteed when the output is closed.

Budget exhaustion is normal completion (exit 0), including a run whose recoverable attempts all failed. Consumers must inspect provider-error events to assess observation availability. Authorization/rejected requests, malformed responses and other terminal provider errors stop the session and retain the provider exit code. Per-request timeouts are recoverable events; run-level cancellation/deadlines exit 130/124. Output/storage/internal errors exit 1; invalid invocation or empty watchlists exit 2.

## Verification and rollback

Fake-clock tests cover state transitions, duplicate suppression, deterministic budget exhaustion, provider failures and cancellation. Subprocess tests verify NDJSON streams and real interrupt delivery during a wait. HTTP fixtures are local; tests do not consume live flight API quota.

Revert the issue #8 change to remove `watch run` while retaining the saved-target commands and scheduler. There is no SQLite schema change; existing watchlist files and backups remain usable. Compatibility/release validation is issue #9.
