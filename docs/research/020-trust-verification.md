# 020 — Trust Verification Chain

> Gap: #15 Trust Verification, #166 Memory Manipulation, #41 Evidence Gap

## Problem Statement

How do you know an agent actually did what it said? It says "tests pass" — did they? It says "deployed successfully" — really? It says "researched 10 sources" — are they real? In real companies, PR reviews, QA sign-offs, and compliance audits verify claims. Agents get a free pass. Trust is assumed, not earned.

## Novel Contribution

**Evidence Ledger**: A cryptographically verifiable hash-chain where every agent claim is backed by evidence. Every action leaves a tamper-evident trace. The chain is append-only — nobody, not even the org itself, can modify or delete claims without detection.

### Key Inventions

1. **Evidence Chain**: Hash-linked chain of claims, like a blockchain but for agent trust. Each claim links to the previous via cryptographic hash. Tampering with any claim breaks the chain.
2. **Multi-Type Evidence**: Claims can be backed by command output, file hashes, URLs, metrics, witness observations, screenshots, or cryptographic signatures.
3. **Verification Protocol**: Independent verification of claims. A verifier agent checks evidence against the claim and records confirmed/refuted/partial status.
4. **Trust Score from Evidence**: Trust isn't a vibe — it's mathematically derived from verified claims, evidence quality, and refutation history.
5. **Tamper Detection**: If any claim in the chain is modified after the fact, the chain breaks and tampering is detected and reported.
6. **Audit Reports**: Full audit trail for any agent — total claims, verified, refuted, chain integrity status.

## Go Prototype

See `internal/evidenceledger/evidenceledger.go`:
- `EvidenceLedger` — the main chain, append-only, tamper-detectable
- `Claim` — an agent's claim with attached evidence items
- `EvidenceItem` — proof backing a claim (output, hash, URL, metric, witness, screenshot, signature)
- `VerificationResult` — independent verification of a claim
- `AuditReport` — comprehensive trust audit per agent
- `ChainIntegrity` verification — detects tampering in the hash chain

## Integration Points

- `internal/trust` — trust scores feed from evidence verification
- `internal/auditlog` — audit trail extends evidence chain
- `internal/qualitygate` — quality gates verify claims before merge
- `internal/compliance` — compliance audits use evidence ledger
- `internal/costconscience` — cost claims backed by evidence

## TODO

- [ ] Wire evidence recording into every tool call (browser, exec, API)
- [ ] Build witness verification protocol (Agent B verifies Agent A's claims)
- [ ] Add evidence expiration (old evidence becomes less trustworthy)
- [ ] Implement evidence strength scoring (command output > claim)
- [ ] Build trust dashboard with per-agent chain visualization
- [ ] Add Merkle tree for efficient partial chain verification
- [ ] Implement cross-org evidence sharing (federated trust)
- [ ] Build automated re-verification (periodically re-check old claims)
- [ ] Add evidence compression for long-running agents
