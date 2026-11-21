# ADR 0004: Evaluate providers without selecting a paid backend

Date: 2026-11-21
Status: Proposed; accepted on merge of #27's evaluation PR. Commercial provider selection remains deferred.

## Context

M2 source candidate `v0.3.0-rc.1` supports observed aircraft and local observation history, not authoritative dated commercial flight status. M3 needs service dates, operating/marketing identities, times and provenance. The owner explicitly chose **$0/month, evaluation only** on 2026-11-21.

## Decision

Use the source-backed [provider evaluation](../PROVIDER_EVALUATION.md) as the desk-research baseline. Do not subscribe, create provider accounts, consume trial/API quota, use keys or import commercial data for this stage. No commercial adapter is selected.

Continue with issue #28: provider-neutral dated-flight modeling and synthetic fixtures only. Preserve callsign observations and commercial flight identity as separate concepts. Do not add a fake live `flight` command or reinterpret missing ADS-B observations as flight status.

Future selection requires an explicit budget change, verified plan entitlements and data rights, plus bounded authorized coverage samples. Public marketing claims, an MIT source license, a small trial price or free usage credit do not satisfy those gates. Storage/export and coexistence with existing ADS-B must be verified for the actual license before integration. Keep provider-specific data rights and provenance out of the existing open-data export path unless explicitly compatible.

## Alternatives

Immediate commercial integration would assume unverified plan/rights/coverage and violate the current budget decision. Replacing ADS-B tracking would discard working M0–M2 functionality without supplying confirmed commercial status. Indefinite inactivity is unnecessary: the domain model and synthetic ambiguity/date/codeshare tests can progress for free.

## Consequences and rollback

No runtime, dependency, schema or billing changes. M3 remains incomplete until adoption gates and integration acceptance pass. A later budget/provider decision supersedes this record with evidence rather than silently changing assumptions.

Rollback is a documentation revert; Git/source candidate checkpoints and both local databases remain unaffected. No vendor outreach is authorized by this ADR.
