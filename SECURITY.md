# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.5.x   | ✅ Active (current) |
| < 0.5   | ❌ End of life |

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Email: **security@forge.dev** (monitored by the Forge security team)

Include:
- Description of the issue and its impact
- Reproduction steps or proof-of-concept (minimal, no exploits)
- Affected versions
- Any suggested mitigations

We acknowledge within **48 hours** and aim to ship a patch within **7 days** for critical issues.

## Go Runtime Security

Forge tracks Go security releases and upgrades promptly:

| Date | Go Version | Notes |
|------|-----------|-------|
| 2026-05-21 | 1.26.3 | 11 fixes: net/http, crypto/tls, html/template, syscall, FIPS |
| 2026-05-21 | 1.24.3 | Prior baseline |

**Policy**: Forge upgrades to the latest Go patch release within 7 days of availability for any release containing CVE fixes.

## Dependency Management

- Dependencies are pinned in `go.sum` and reviewed via `go mod tidy`.
- `forge doctor` checks for known-stale WAL files and permission issues at startup.
- Audit log: `~/.forge/audit/` records all agent requests, decisions, and governance events.
- Supply-chain: No external CI secrets or tokens are embedded in the binary. All auth is runtime-configured via `~/.forge/config.json`.

## Known Security Controls

| Control | Implementation |
|---------|---------------|
| Auth | Token-based (mcpgateway) — per-client tokens, rate limiting |
| Governance | Consent receipts, catalog registration, audit trails |
| Persistence | WAL + atomic rename — no partial writes exposed |
| Rate limiting | Per-client, per-minute caps in mcpgateway |
| Data isolation | Per-agent `.forge/` directories |
| Local-first | No cloud calls by default with `--local` preset |

## Scope

In scope for vulnerability reports:
- Authentication bypass in mcpgateway
- WAL or persistence data corruption leading to data loss
- Remote code execution via agent inputs
- Governance bypass (consent/catalog/audit)
- Supply-chain issues in `go.mod` dependencies

Out of scope:
- Demo/local-only configurations with no network exposure
- Issues in unsupported versions (< v0.5.0)
- Social engineering

## Disclosure Policy

We follow **coordinated disclosure**: once a fix is shipped, we publish a security advisory on GitHub. Credit is given to reporters who request it.
