// Package resilience provides a unified resilience framework for Forge,
// combining circuit breakers, rate limiting, runaway detection, cost anomaly
// detection, outage handling, and self-healing capabilities.
//
// Sub-packages:
//   - resilience/circuit: Circuit breaker per provider (closed/open/half-open)
//   - resilience/ratelimit: Token bucket rate limiting
//   - resilience/runaway: Agent runaway detection and auto-termination
//   - resilience/anomaly: Cost anomaly detection with budget enforcement
//   - resilience/outage: Provider outage detection with auto-fallback
//   - resilience/selfheal: Self-healing engine for automated recovery
package resilience
