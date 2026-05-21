# Forge Compliance Architecture

> Legal gates, audit trails, responsibility chains. Because AI orgs need governance too.

## The Compliance Stack

```
┌─────────────────────────────────────────────────────────────┐
│                    COMPLIANCE LAYERS                         │
│                                                              │
│  Layer 4: LEGAL GATES                                       │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Contract review · IP scanning · Regulatory filing    │   │
│  │ GDPR data handling · Financial commitment approval   │   │
│  │ "Can this agent legally do this?"                     │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  Layer 3: POLICY ENGINE                                     │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Role-based access · Data classification · Retention  │   │
│  │ Consent management · Jurisdiction awareness           │   │
│  │ "Is this action allowed by our policies?"             │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  Layer 2: AUDIT TRAIL                                       │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Immutable ledger · Decision genealogy · Evidence     │   │
│  │ Cryptographic proof · Timeline reconstruction        │   │
│  │ "Can we prove what happened and why?"                 │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  Layer 1: RESPONSIBILITY CHAINS                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Who requested → Who approved → Who executed → Who    │   │
│  │ verified → Who is accountable                         │   │
│  │ "Who is responsible for this outcome?"                │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Layer 1: Responsibility Chains

**Packages**: `internal/lineage/`, `internal/genealogy/`, `internal/witness/`

Every action has a complete chain:

```
Action: "Deploy v2.3.1 to production"
├── Requested by: Agent "Rex" (Engineering Division)
├── Approved by: Agent "Oversight" (Division Head)
├── Code authored by: Agent "Nex" (PR #421)
├── Code reviewed by: Agent "QA" (approved with 2 findings)
├── Tests passed: Yes (47/47, 0 flaky)
├── Security scan: Clean (0 CVEs)
├── Deployed by: Agent "Ops" (rolling deploy, 3 pods)
├── Verified by: Agent "Watchdog" (health checks green for 15min)
└── Human accountable: Indu (org owner, consent given 2026-05-21 20:30)
```

This chain is:
1. Stored in the genealogy DAG (`internal/genealogy/`)
2. Cryptographically signed at each step (`internal/witness/`)
3. Queryable via `forge lineage trace <action-id>`
4. Exportable for external auditors

## Layer 2: Audit Trail

**Packages**: `internal/audit/`, `internal/auditlog/`, `internal/ledger/`

Three complementary audit systems:

### Audit (internal/audit/)
- High-level compliance audit: SOC2, HIPAA, GDPR, ISO 27001
- Generates auditor-ready reports
- Evaluates current posture against framework requirements

### AuditLog (internal/auditlog/)
- Low-level action logging with hash chains
- Every agent action is logged with timestamp, agent ID, action type
- Append-only, tamper-evident
- Can be exported to SIEM systems

### Ledger (internal/ledger/)
- Financial audit trail
- Immutable hash-chained cost records
- Budget enforcement evidence
- Revenue tracking (for billing internal departments)

## Layer 3: Policy Engine

**Package**: `internal/policy/`

```yaml
# Example policies
policies:
  - name: "production-deploy"
    effect: deny
    condition:
      division: engineering
      action: deploy
      environment: production
    unless:
      - code_review_approved: true
      - test_coverage: ">80%"
      - security_scan: clean
      - human_approval: true

  - name: "pii-handling"
    effect: deny
    condition:
      data_classification: pii
    unless:
      - consent_obtained: true
      - encryption: aes-256
      - retention_policy: defined

  - name: "financial-commitment"
    effect: deny
    condition:
      action: commit_funds
      amount: ">1000"
    unless:
      - human_approval: true
      - finance_division_review: true

  - name: "external-communication"
    effect: deny
    condition:
      action: send_external
    unless:
      - communication_reviewed: true
      - division_head_approved: true
```

### Consent Management
**Package**: `internal/consent/`

GDPR-compliant consent tracking:
- What data is collected, for what purpose, with what retention
- Consent receipts (who consented, when, to what)
- Right to withdraw consent (triggers data deletion cascade)
- Consent audit trail for regulators

## Layer 4: Legal Gates

These are hard blocks on specific actions:

| Action | Legal Gate | Implemented By |
|--------|-----------|---------------|
| Deploy to production | Code review + test pass + security scan | `internal/review/`, `internal/quality/` |
| Send external email | Communication review | `internal/guard/` (block rule) |
| Process PII | Consent check + data classification | `internal/consent/`, `internal/secrets/` |
| Make financial commitment | Human approval + finance review | `internal/approval/` |
| Use copyrighted code | IP scan + license check | `internal/sbom/`, `internal/compliance/` |
| Store data in cloud | Data residency check | `internal/residency/` |
| Share data with third party | Data sharing agreement check | `internal/policy/` |
| Modify org structure | Human approval (consent gate) | `internal/consent/` |

## Compliance Frameworks

**Package**: `internal/compliance/`

Supported frameworks with auto-evaluation:

| Framework | Coverage | Report |
|-----------|----------|--------|
| SOC2 Type II | Access control, audit logging, encryption, incident response | `forge compliance report --framework=soc2` |
| HIPAA | PHI handling, access logs, encryption, BAA tracking | `forge compliance report --framework=hipaa` |
| GDPR | Data classification, consent, DPIA, right to erasure | `forge compliance report --framework=gdpr` |
| ISO 27001 | ISMS, risk assessment, access control, incident management | `forge compliance report --framework=iso27001` |

## Implementation Status

| Component | Package | Status |
|-----------|---------|--------|
| Action genealogy | `internal/genealogy/` | ✅ Built |
| Decision lineage | `internal/lineage/` | ✅ Built |
| Cryptographic proof | `internal/witness/` | ✅ Built |
| Compliance reports | `internal/compliance/` | ✅ Built |
| Audit logging | `internal/audit/`, `internal/auditlog/` | ✅ Built |
| Policy engine | `internal/policy/` | ✅ Built |
| Consent management | `internal/consent/` | ✅ Built |
| Data residency | `internal/residency/` | ✅ Built |
| Secret scanning | `internal/secrets/` | ✅ Built |
| SBOM generation | `internal/sbom/` | ✅ Built |
| RBAC | `internal/auth/rbac/` | ✅ Built |
| Approval workflows | `internal/approval/` | ✅ Built |
| Safety guardrails | `internal/guard/` | ✅ Built |

## What's Missing

1. **Policy → guard integration** — policies defined but not auto-enforced by guard
2. **Consent cascade** — withdrawing consent should trigger automated data deletion
3. **Legal document generation** — auto-generate DPAs, BAAs, privacy policies
4. **Jurisdiction detection** — auto-detect applicable laws based on data location
5. **Regulatory change monitoring** — track new regulations and flag impact on org
