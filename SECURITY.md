# Security policy

Sky is pre-release software. Security fixes currently target the latest `main` branch; no released version has a separate support commitment.

## Report a vulnerability

Private vulnerability reporting is currently disabled; enabling it is tracked in [#4](https://github.com/Mickyj70/sky/issues/4). Once enabled, use [GitHub's private vulnerability reporting](https://github.com/Mickyj70/sky/security/advisories/new). Include affected revision, reproduction steps, impact, and a minimal sanitized example. Do not include working API keys, personal information, or collected aircraft histories.

If private reporting is unavailable, contact GitHub Support for a safe reporting route rather than opening a public issue with exploit details. Do not post credentials or detailed exploit instructions in public issues or pull requests.

Maintainers will assess reports as capacity allows; there is currently no guaranteed response time, security SLA, or bounty program.

## Credentials and provider data

- Supply provider credentials through the documented environment variable; never commit `.env` files or API keys.
- Tests use local HTTP fixtures and must not require live credentials.
- Git history does not back up ignored local data. Future persistent storage must have a tested backup/restore path.
- Provider coverage, freshness, and errors must not be treated as authoritative aircraft operational status.
