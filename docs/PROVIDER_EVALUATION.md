# M3 flight-status provider evaluation

Checked: 2026-11-21. Issue #27. **Owner decision: $0/month, evaluation only.** No provider selected, subscription created, key used, paid API called, or commercial data imported. This document records public-source findings and Sky's proposed adoption tests; it does not establish measured coverage or negotiated data rights.

M2 source candidate `v0.3.0-rc.1` is published and PR #26 merged. Existing adsb.lol lookups and local history continue independently. A broadcast callsign/position cannot establish a dated commercial flight's schedule or status.

## Recommendation

Keep the commercial adapter deferred under the $0 budget. Proceed with **#28: provider-neutral dated-flight identities and synthetic offline fixtures**, without exposing a pretend live `sky flight` command. Free credits/trials are not authorization to subscribe, accept terms or consume quota.

If the owner later changes the budget, investigate aviationstack Basic first for bounded dated-flight evaluation; this is a conditional shortlist, not approval or a coverage claim. FlightAware merits comparison if its applicable license permits Sky's use alongside ADS-B. Aviation Edge needs both a larger recurring budget and clarification of client/distribution restrictions. No provider currently passes every adoption gate below.

## Public evidence comparison

USD monthly list prices observed on the date above, excluding taxes, annual commitments and negotiated terms. Recheck before any purchase.

| Provider | Advertised capability / entitlement | Published cost and quota | Unresolved for Sky |
| --- | --- | --- | --- |
| [FlightAware AeroAPI](https://www.flightaware.com/commercial/aeroapi/) | Flight status/current data; historical product access is tier-dependent. Personal excludes historical flight data; Standard includes it. | Personal has no minimum and up to $5 monthly usage credit; Standard $100 minimum/month. Personal rate: 10 result sets/minute. `/flights/{ident}` lists $0.005/result set; a set is 15 records, not necessarily one HTTP request. | Date window/endpoint entitlement and sample completeness; applicable license for each contributor/user; combining sources and exports. |
| [aviationstack pricing](https://aviationstack.com/pricing) and [endpoint documentation](https://docs.apilayer.com/aviationstack/docs/api-documentation) | Dated historical `/v1/flights?flight_date=` is Basic+; schedules/future endpoints and entity catalogs are documented. | Free: 100 requests/month, non-commercial real-time use. Basic: $49.99/month, 10,000 requests; $0.019996/additional call. Paid quota is not a hard spending cap. | Plan-specific coverage, operating/marketing identity, exact date/time semantics, burst limits and contractual cache/storage/export permissions. |
| [Aviation Edge pricing](https://aviation-edge.com/premium-api/) and [historical schedules](https://aviation-edge.com/historical-flight-schedules-api/) | Historical dates/ranges, status, scheduled/estimated/actual times, IATA/ICAO airline identities and codeshare examples. | Developer: $7 initial month, then $299/month for 30,000 calls; automatic monthly renewal. Free limited access advertised separately; no sufficient dated-flight entitlement verified. | Recurring cost, per-second limits/overage behavior and written confirmation that the intended open-source client/storage/export use is allowed. |

FlightAware's [billing explanation](https://support.flightaware.com/hc/en-us/articles/33213921155351-Where-Can-I-Request-A-Quote-For-AeroAPI) says returned pages/result sets affect charges and customers must monitor usage. Do not equate Sky's request count with billable result sets.

aviationstack's [FAQ](https://aviationstack.com/faq) describes a three-month rolling historical window. Treat this as an advertised scope, not evidence that a particular Lagos, London or US flight is complete. Its [overage documentation](https://aviationstack.com/billing-overages-documentation) describes continued service with extra charges rather than assuming quota exhaustion stops billing.

Aviation Edge's historical example contains timestamps without UTC offsets. An adapter must verify their timezone semantics instead of appending `Z`. A codeshare field existing in a sample does not prove every carrier/leg populates it.

## Storage, export and open-source constraints

- [FlightAware Personal license, January 2025](https://www.flightaware.com/commercial/aeroapi/AeroAPI_Personal_License_Jan2025.pdf): permits personal/academic retrieval/storage, limits raw retention to 30 days, restricts transfer, and requires prior written permission for use with/backfill to another real-time or near-real-time provider. **Our assessment:** existing adsb.lol support makes that last restriction a material unresolved gate; separate tables alone do not prove permission. Verify the agreement actually offered for the selected plan.
- [Aviation Edge terms, effective 2025-06-30](https://aviation-edge.com/api-terms-of-service/): restrict data distribution, derivative databases, key sharing and products/API clients that substantially replicate its service for third-party use. **Our assessment:** do not assume an MIT CLI or JSON export is permitted; obtain applicable confirmation before implementation.
- [aviationstack's terms link](https://aviationstack.com/terms) redirected to general IDERA website terms when checked. [APILayer /terms](https://apilayer.com/terms) returned 404. These do not establish a product-specific grant for perpetual local history or redistribution. Mark permissions **unverified**, not allowed. Paid “commercial use” on a pricing table does not resolve every storage/export right.

These are engineering adoption gates, not a claim to have reviewed a customer's signed contract. Distribute Sky code under MIT, not provider datasets or keys. Use synthetic public fixtures. Each user must hold their own applicable access rights; a maintainer key must never be embedded in an open-source binary, CI or release asset. Do not place commercial responses into the existing ODbL observation/export pipeline merely because the schemas look similar.

## Workload and budget model

The following are planning scenarios, not measured workloads or quotes:

| Scenario | Arithmetic (30-day month) | Consequence |
| --- | --- | --- |
| Manual lookup | 10/day × 30 = 300 calls/month, assuming one page and no retries | Exceeds aviationstack's advertised 100 free calls; dated history is separately paid. At FlightAware's listed $0.005 rate, 300 one-set results would be $1.50 before credits/minimums/other endpoint costs; this does not establish date entitlement or a hard cap. |
| Follow ten flights | 10 × 12 polls/hour × 24 × 30 = 86,400 attempted lookups | Exceeds the listed Basic/Developer included quotas. Avoid carrying M1 polling defaults into a commercial adapter. |
| aviationstack Basic at 12,000 calls | $49.99 + 2,000 × $0.019996 = $89.982, about $89.98 | Overage matters even with a subscription; this assumes unchanged published rates and no other charges. |

Retries, pagination, identity resolution and concurrent processes add usage. A future adapter needs endpoint-specific accounting, finite page limits, an explicit per-command request budget and clear provider-billing limitations. Local budgets cannot guarantee an account-wide dollar cap across other clients. Under today's **$0 decision**, none of these scenarios is executed and no trial starts.

## Verification before adopting any provider

1. Record owner-approved budget, plan, recurring price, overages/renewal and an account-side spending control where available. A later budget change must be explicit.
2. Obtain applicable terms/confirmation for CLI distribution, personal/commercial users, cache duration, persistent history, JSON/CSV output, redistribution, attribution and coexistence with ADS-B. Record the exact plan/document/version and answer, without secrets. Do not email vendors without owner authorization.
3. Confirm endpoint/base URL and authentication from the chosen account's current docs. Public APILayer pages currently show differing host/endpoint examples; do not copy example credentials or execute documentation “Run request” links. FlightAware's interactive reference did not yield usable endpoint detail in this check.
4. Only after authorized access, run a bounded sample: 20 dated service instances, 40 provider attempts maximum, with an explicit page/billing bound. Proposed split: 6 Lagos-related, 6 London-related, 6 US domestic and 2 multi-leg/codeshare cases; include recent past, today and supported future dates. This is a proposed protocol, not completed coverage evidence.
5. For every case, record requested service date/departure timezone, provider identity, operating/marketing numbers, legs, registration/nulls, scheduled/estimated/actual gate/runway times, status, retrieval time, missing fields, pages and error/quota signals. Check an authorized airline/airport reference for the same leg/date; record discrepancies rather than inventing a truth score. Document sample ground-truth limitations and unavailable cancellations/aircraft changes.
6. Pass synthetic regressions for number reuse across dates, midnight/DST, cancellation, codeshares, multiple legs, registration changes, unsupported dates and conflicting timestamps. Define ambiguous/missing/unsupported outcomes before live CLI integration.

## Current decision and delivery order

**Desk research complete; commercial adoption deferred.** Cost/permissions/sample gates remain unresolved and are not M3 completion. The owner's $0 choice is the decisive constraint, independently of the other gaps.

- #28 can build a provider-neutral domain model and fixture tests offline, without credentials, schema changes, new billing or a live flight command.
- A later adapter/adoption issue requires the gates above, an updated [decision record](decisions/0004-flight-status-provider.md), and explicit permission for any account creation, spending or vendor outreach.
- Keep M2 candidate feedback separate from M3 research. Stable 0.3.0 and prebuilt distribution are not implied by this evaluation.
