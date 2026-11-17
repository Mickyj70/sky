# CLI and provider contract (alpha)

Applies to aircraft-result commands: `plane`, `callsign`, `nearby`, and `airline`.
The aircraft-result contract was introduced by issue #5. Local `watch` management has a separate [watchlist contract](WATCHLISTS.md). This contract does not add scheduled
flight lookup, recurring polling, persistence, or automatic retries.

## Invocation and output

- Without `--json`, results are human-readable tables. Table spacing and diagnostic
  wording are not a scripting API.
- With `--json`, a successful result writes one JSON object followed by a newline
  to stdout and writes no diagnostics to stderr. Help/version output remains text.
- Query, validation, and provider failures write a `sky:` diagnostic to stderr and
  return a nonzero status. They write no result to stdout. Error messages are for
  people; use exit codes or typed Go errors for classification.
- An output/write failure may leave partial stdout; consumers must check the exit
  code before accepting output. There is no JSON error envelope during alpha.
- Zero observations is a successful lookup, not a flight-status assertion.
- Aircraft are sorted lexically by hex address; optional values are not fabricated.

## JSON shape

Provider-derived aircraft field names are intentionally retained during alpha to
avoid gratuitous changes to existing scripts. New provider adapters map into this
shape; provider request paths do not belong in the CLI or scheduler. Before 1.0,
any incompatible rename/type change must be documented in the changelog and PR.
Consumers should tolerate additional object fields.

Synthetic example:

```json
{
  "source": "adsb.lol",
  "retrieved_at": "2026-11-16T12:00:01Z",
  "snapshot_at": "2026-11-16T12:00:00Z",
  "aircraft": [
    {
      "hex": "000001",
      "flight": "TEST123",
      "r": "G-TEST",
      "t": "A320",
      "alt_baro": "ground",
      "gs": 0,
      "lat": 0,
      "lon": 0,
      "seen": 0,
      "seen_pos": 0
    }
  ]
}
```

| Field | JSON type | Meaning |
| --- | --- | --- |
| `source` | string, required | Provider identity; currently `adsb.lol` |
| `retrieved_at` | string, required | UTC RFC 3339 timestamp when the valid response was decoded; fractional seconds are allowed |
| `snapshot_at` | string, optional | UTC RFC 3339 provider snapshot time, converted from Unix milliseconds; omitted if unknown |
| `aircraft` | array, required | Observed aircraft; empty observations always encode as `[]` |
| `aircraft[].hex` | string, required | Six hexadecimal address characters; a provider `~` prefix denotes a non-ICAO/pseudo address and is preserved |
| `flight` | string, optional | Trimmed broadcast callsign, not a dated scheduled flight identity |
| `r` | string, optional | Registration/tail number |
| `t` | string, optional | Provider aircraft type code |
| `alt_baro` | number or string, optional | Barometric altitude in feet, or the exact string `ground` |
| `gs` | number, optional | Ground speed in knots, including zero |
| `track` | number, optional | Ground track in degrees, including zero |
| `lat` / `lon` | number, optional | Degrees; valid ranges are -90..90 and -180..180 |
| `seen` | number, optional | Seconds since last message at the provider snapshot |
| `seen_pos` | number, optional | Seconds since last position at the provider snapshot |

Absent/null numeric readings remain unknown and are omitted. Null/missing altitude
is omitted. Zero altitude/speed/age/coordinates remain valid numeric readings.
Empty optional strings are omitted. Missing coordinates are independent: do not
assume a complete position just because one coordinate is present.

A missing or non-array `ac` upstream, null/invalid aircraft entries, an unsupported
altitude value, wrong numeric types, negative ages/speed, out-of-range
coordinates/track, or a snapshot timestamp outside supported UTC years 1970–9999 is an invalid response. A
malformed response fails the whole query instead of returning a silently partial
result. Responses larger than 16 MiB are rejected. Additional upstream fields are
ignored, so additive schema changes are tolerated.

`--airborne` selects numeric altitude observations and excludes ground/unknown
altitude. It does not establish authoritative flight status. Airline filtering is
regional broadcast-prefix matching, not complete fleet membership.

## Exit status

| Code | Meaning |
| --- | --- |
| 0 | Success, including empty observations, help and version |
| 1 | Runtime/output failure not otherwise classified |
| 2 | Invalid invocation, flag, identifier, arguments or local query options |
| 3 | Network/body-read failure, HTTP 408/425, upstream 5xx, or other temporary provider unavailability |
| 4 | Provider HTTP 429 rate limit |
| 5 | Provider authorization failure (401/403), other rejected HTTP response, or explicit provider query error |
| 6 | Invalid/malformed/oversized provider response |
| 124 | Request deadline or transport timeout |
| 130 | Context cancellation, including handled Ctrl-C/SIGTERM |

These codes refine the prototype's all-errors-exit-1 behavior. Processes terminated
outside the handled signal/cancellation path may have OS-defined statuses.

## Go provider boundary

`provider.Source.Lookup(context.Context, provider.Query)` is the shared interface.
Queries describe registration, hex, callsign or an area with a radius in
kilometres. The adsb.lol adapter validates queries and handles provider URL paths
and conversion to nautical miles. Embedders can inject a Source through
`cli.NewWithSource`; there is no production environment variable to redirect API
credentials to arbitrary endpoints.

Failures are `*provider.Error`, usable through `errors.As`:

- `Kind`: `rate_limited`, `unavailable`, `unauthorized`, `rejected`,
  `invalid_response`, `canceled`, or `timeout`.
- `StatusCode`: HTTP status when available, zero before any response.
- `RetryAt`: optional absolute UTC cooldown deadline from valid Retry-After seconds
  or HTTP-date; a past date means no further delay. Missing, malformed, negative,
  or unrepresentable values leave it unknown rather than treating it as zero.
- `Retryable()`: true only for throttling, temporary unavailability and timeouts.
  This is classification, not permission to spend requests. The future scheduler
  must still enforce quotas, backoff, deadlines and a finite request budget.
- `Unwrap()`: preserves context/network/body-read causes for `errors.Is` / `As`.

The CLI enforces `--timeout` using a request context and makes one provider call.
There are no implicit retries. Redirects are not followed, including same-origin redirects; their HTTP statuses are rejected-provider failures (exit 5). This prevents additional redirect requests and redirect credential forwarding. Embedders should use `cli.Execute` with a constructed
command and `cli.ExitCode` to retain parsing-versus-runtime classification.

## Verification and rollback

Offline tests cover every query path, empty/mixed/missing readings, invalid schema,
status/error categories, Retry-After date/seconds boundaries, oversized data,
in-flight cancellation and request timeout. Subprocess tests run the application
runner with a local HTTP source and verify exit codes and streams; an independently
built sky binary verifies the entrypoint's help, version and usage errors.

Revert the issue #5 PR to restore prototype behavior. No database/schema migration
is involved. Scripts that adopt the new exit codes should be reverted alongside a
code rollback if they rely on the more detailed statuses.
