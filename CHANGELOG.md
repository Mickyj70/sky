# Changelog

Only actual releases receive versioned sections. The CLI currently reports 0.1.0 as a prototype identifier; no public release has been published.

## Unreleased

### Added

- Go/Cobra CLI with registration/ICAO hex lookup, callsign lookup, nearby search, and regional airline callsign filtering.
- Table and JSON output including provider snapshot/retrieval timestamps and missing-reading handling.
- API key configuration, timeout/cancellation, rate-limit errors, and local HTTP tests.
- Roadmap, initial backlog, current status, architectural decision, contribution/recovery guidance, and GitHub CI/issue/PR templates.
- Local Git checkpoints for the working prototype and planning foundation.

- MIT code license, public contribution/security policies, fork workflow, Dependabot, and canonical GitHub module path.

### Changed

- Query-based injectable provider boundary, with endpoint construction isolated from CLI commands.
- Typed provider errors and Retry-After deadlines for future bounded polling.
- Detailed exit codes and an explicit alpha JSON contract (provider field names retained).
- Invalid/oversized provider responses fail explicitly; unknown readings remain absent.
- Offline HTTP fixtures, cancellation/error-boundary coverage, subprocess stream/exit checks, and real-binary entrypoint checks.
- Checked-in desired main-branch protection configuration; applying it remains blocked by repository administration permissions.
