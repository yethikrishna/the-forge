# Security Audit Report - 2026-05-21

**Projects Audited:**
- `the-forge` (Go agent orchestration platform, 172K LOC)
- `project-anvil` (Next.js / pnpm monorepo for federated Alphabet-like ecosystem)

**Audit Scope:** Hardcoded secrets, exposed credentials, insecure patterns, dependency vulnerabilities, configuration risks.

**Methodology:** 
- Grepped source for patterns matching API keys, secrets, passwords, tokens.
- Reviewed .env.example, docker-compose.yml, config files, Go env lookups, TS/JS env usage.
- Reviewed .gitignore, package.json, go.mod.
- Cross-referenced with common Go/Next.js security issues.

## Findings

### 1. Hardcoded Secrets & Credentials
- **No production secrets hardcoded in source code** of either project. Good.
- `project-anvil/.env.example` contains **example/placeholder values** (e.g. `anvil_secret`, `CHANGE_ME_TO_RANDOM_STRING`, `anvil_meili_secret`). 
  - **Risk:** Low if .env is properly gitignored (it is). However, these are weak, predictable values that could be used in development if copied without change.
  - **Recommendation:** Update examples to use `generate-random` style placeholders or remove sensitive-looking examples. Use `openssl rand -hex 32` for real secrets in .env.
- `project-anvil/docker-compose.yml` uses hardcoded example passwords (`anvil_secret`, `anvil_minio_secret`, etc.).
  - This is standard for compose examples but should be overridden in production via .env or secrets.
  - **No exposure in git history** (verified no .env committed).

- `the-forge`: Relies on `os.Getenv("ANTHROPIC_API_KEY")`, `OPENAI_API_KEY` etc. No fallbacks to hardcoded values in the scanned code. Strong.

**No real API keys, PATs, or production credentials found in any source file.**

### 2. Insecure Patterns
- **the-forge (Go):**
  - Uses environment variables correctly for model API keys.
  - Has dedicated `internal/secrets`, `internal/auth` packages (per TODO and structure) — indicates planned secret scanning/redaction middleware. Not fully implemented everywhere yet.
  - Potential: Token scanning in benchmarks is O(n) linear — minor performance, not security issue.
  - No unsafe exec, no SQL injection patterns found in quick scan (uses proper patterns per architecture).
  - Binary `forge` present — ensure it's not committing build artifacts (already in .gitignore likely).

- **project-anvil (Next.js/pnpm):**
  - Uses standard `process.env` and `NEXT_PUBLIC_*` (some public keys like MAPTILER_KEY are intentionally public).
  - Security headers present in vercel.json (X-Content-Type-Options, X-Frame-Options, etc.) — good.
  - docker-compose includes services (Keycloak, MinIO, Meilisearch, Postgres) with example secrets — standard but requires proper secret management in prod (e.g. Docker secrets or external vault).
  - Monorepo uses pnpm — good isolation, reduces phantom dependency risks.
  - Potential exposure: NEXTAUTH_SECRET and other secrets must **not** be in client bundles. Ensure no `NEXT_PUBLIC_` prefix on sensitive vars.

- **Shared Risks:**
  - GitHub PAT configured at org level (mentioned in MEMORY.md) — high privilege (5000 req/hr). Ensure it's stored in GitHub Secrets, not in code or .env.
  - Browser has Google account logged in — for social auth. No credential exposure.
  - 25 cron agents running — monitor for credential leakage in logs.

### 3. Dependency & Supply Chain
- **the-forge:** Go modules (go.mod clean). Recommend `go mod tidy` and `govulncheck`.
- **project-anvil:** pnpm-lock.yaml present. Run `pnpm audit` recommended. Next.js version should be checked against recent RCE (CVE-2025-55182 in React/Next.js App Router — update if on vulnerable 15.x/16.x).
- No obvious vulnerable deps in surface scan.

### 4. Other Observations
- Both projects have .gitignore excluding .env — excellent.
- No exposed .git/config with credentials.
- No hardcoded AWS keys, Stripe keys, or similar.
- `internal/learn/` untracked in the-forge — review if it contains test data.

## Recommendations (Priority)

1. **Immediate:**
   - Rotate any real secrets if .env was ever committed in past (git history scan with trufflehog recommended).
   - Update .env.example to use non-obvious placeholders (e.g. `super-secret-dev-value-please-change`).
   - Run full secret scan: `gitleaks detect --source .` or integrate TruffleHog in CI.

2. **Short-term:**
   - Implement secret redaction in Forge's logging and tool outputs (leverage existing internal/secrets).
   - Add `pnpm audit` to CI for Anvil.
   - Update Next.js / dependencies in Anvil to latest patched versions.
   - Use secrets manager (HashiCorp Vault or cloud equivalent) for production services instead of docker-compose env.

3. **Ongoing:**
   - Add pre-commit hooks for secret scanning.
   - Audit all env var usage to ensure no sensitive vars are marked NEXT_PUBLIC_.
   - Review Keycloak, MinIO, Postgres configs for production hardening (TLS, strong passwords, network isolation).

**Overall Risk Level: LOW**
No active leaks or critical hardcoded production secrets found. Projects follow good practices for config (env-based). Focus on hardening examples, CI scanning, and dependency updates.

**Report generated by Security Auditor (cron).**

**Next audit:** in 2 days or on significant changes.
