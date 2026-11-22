# Sky 1.x compatibility policy

This policy takes effect with stable `v1.0.0`. The current binary prerelease is the baseline for checking the proposed contract.

| Surface | 1.x commitment |
| --- | --- |
| Commands and flags | Keep documented command paths, flags, argument meanings and defaults compatible for aircraft lookups, watchlists, live watch, and local history. |
| Structured output | Preserve documented fields, types, units, omission rules and meanings. Readers should accept additional fields and handle future enum values. Stream readers should ignore unrecognized event types and still handle the documented lifecycle events. |
| History CSV | Preserve documented columns, ordering and semantics. |
| Exit codes | Preserve the meanings below. |
| Local data | New releases must support documented upgrade paths. Each schema change requires a verified backup before migration and tests for interrupted migration and restore. Downgrades use a compatible snapshot restored into a fresh directory. |

Use the [lookup contract](CLI_CONTRACT.md), [watchlist contract](WATCHLISTS.md), [watch stream contract](WATCH_RUN.md) and [history contract](HISTORY.md) for field-level details. Human-readable output, help text and diagnostic wording may evolve. Provider coverage and availability vary independently of Sky versions.

| Code | Meaning |
| --- | --- |
| 0 | Successful command, including an empty lookup or exhausted watch budget |
| 1 | Other execution failure, such as storage or output failure |
| 2 | Invalid invocation |
| 3 | Provider unavailable or another upstream failure |
| 4 | Terminal rate limit |
| 5 | Provider authorization or request rejection |
| 6 | Invalid provider response |
| 124 | Deadline or terminal timeout |
| 130 | Cancellation |

For watch runs, inspect structured provider-error events as well as process status. Recoverable errors can consume the entire budget and the process can still exit successfully. Use [watch guidance](WATCH_RUN.md) for quotas and [history guidance](HISTORY.md) for retention and pruning.

## Deprecation

Announce a deprecation in the changelog and command documentation with the replacement and earliest planned removal version. Keep the original documented behavior throughout 1.x. Remove or change it in a later major version with migration examples. Keep notices outside the machine-readable result stream. An urgent security or upstream change that makes the contract impossible to preserve requires a release note identifying the affected interface, the impact, and the workaround.
