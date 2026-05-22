# 022 — Legal Compliance Gates

> Gap: #24 Legal and Compliance, #39 Contractual Commitment, #71 Regulatory Filing, #74 Compliance Audit

## Problem Statement

Who's responsible when an AI agent violates GDPR? When it uses copyrighted code? When it makes a financial commitment? In real companies: legal review, compliance checks, approval workflows. Agents operate in a legal vacuum. No policy engine. No risk classification. No veto power.

## Novel Contribution

**Policy Engine with Enforcement**: Every action is classified by risk level, checked against compliance policies, and either approved, blocked, or escalated. The legal division has veto power over any action that creates liability. This is NOT a suggestion system — it's enforcement infrastructure.

### Key Inventions

1. **Risk Classification**: Every action is classified by domain (data handling, communication, financial, IP, etc.) and risk level (none → critical). The risk determines the gate.
2. **Policy Rules**: Configurable compliance policies with keyword triggers, domain matching, and required approvers. Default policies cover GDPR, financial commitments, IP contamination, data exfiltration, public communications, and regulatory filings.
3. **Gate Decisions**: Approved (auto), Blocked (hard stop), Escalated (needs review), Deferred (needs more info), Exempted (emergency override). Not just "warn and proceed."
4. **Approval Workflow**: Blocked/escalated actions require explicit approval from the right role (legal, human, division_head). Multiple approvals for critical actions.
5. **Audit Trail**: Every gate check, approval, rejection, and exemption is logged with full context. Legally defensible evidence of compliance.
6. **Emergency Exemptions**: For production incidents, actions can be exempted with documented justification. The exemption is audited retroactively.

## Go Prototype

See `internal/legalgate/legalgate.go`:
- `LegalGate` — the policy engine with enforcement
- `ComplianceAction` — action being evaluated
- `PolicyRule` — configurable compliance rules with triggers
- `GateResult` — the gate's decision with matched rules and reasons
- `Approval` — approval workflow with role-based verification
- `AuditEntry` — full compliance audit trail
- 10 default policies covering GDPR, financial, IP, data handling, deployment, regulatory

## Integration Points

- `internal/compliance` — compliance reporting
- `internal/policy` — policy definitions
- `internal/consent` — consent management
- `internal/evidenceledger` — compliance evidence chain
- `internal/auditlog` — audit trail persistence

## TODO

- [ ] Wire legal gate into every external action (email, API, deployment)
- [ ] Build compliance dashboard with risk heat map
- [ ] Add jurisdiction-aware policies (different rules per country)
- [ ] Implement policy versioning and change tracking
- [ ] Build regulatory reporting (GDPR, SOC 2, HIPAA templates)
- [ ] Add policy suggestion engine (learn from blocked actions)
- [ ] Implement cross-org policy sharing (industry compliance templates)
- [ ] Build legal division agent that reviews escalated actions
- [ ] Add policy testing framework (verify policies work as intended)
- [ ] Implement retroactive compliance audit (check past actions against new policies)
