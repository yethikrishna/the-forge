# 016 — MicroVM Sandboxing

> Gap: Forge Engine Infrastructure, Security Gap #36

## Problem Statement

Agents execute arbitrary code. Today that runs in Docker containers or directly on the host — both have escape risks. Forge needs lightweight, fast-booting, strongly-isolated sandboxes. MicroVMs provide VM-level isolation with container-like speed. The perfect execution environment for untrusted agent work.

## Design Decisions

### Why MicroVM, Not Docker

Docker shares the host kernel. Container escapes are real (CVE-2024-1086, CVE-2022-0492, etc.). MicroVMs run their own kernel:
- **Strong isolation**: Separate kernel = kernel exploits don't escape
- **Fast boot**: ~125ms (vs 5-30s for traditional VMs)
- **Tiny footprint**: ~5MB RAM overhead per VM
- **Deterministic**: Same VM image = same behavior

### Resource Limits

Every MicroVM has enforced limits:
```go
type ResourceLimits struct {
    CPUs       float64       // CPU cores (fractional allowed)
    MemoryMB   int           // RAM in MB
    DiskMB     int           // Disk quota in MB
    Network    NetworkPolicy // allow/deny per-host
    Processes  int           // max concurrent processes
    TimeLimit  time.Duration // max execution time
}
```

If a VM exceeds limits, it's killed. No exceptions.

### Snapshot Support

Save VM state for:
- **Resume later**: Pause expensive computation, resume when needed
- **Clone**: Fork a VM to explore parallel approaches
- **Rollback**: Restore to a known-good state after error
- **Template**: Create a VM template, instantiate multiple copies

### Network Isolation

Each VM gets its own network namespace:
- No access to host network
- No access to other VMs (unless explicitly allowed)
- Egress whitelist (only approved external hosts)
- DNS isolation (can't poison host DNS)

## API Surface

```go
type MicroVM struct { ... }

// Create a new MicroVM with specified limits
func NewMicroVM(config VMConfig) (*MicroVM, error)

// Start the VM
func (vm *MicroVM) Start() error

// Stop the VM (graceful shutdown)
func (vm *MicroVM) Stop() error

// Kill the VM (immediate termination)
func (vm *MicroVM) Kill() error

// Execute a command inside the VM
func (vm *MicroVM) Exec(cmd string, args ...string) (*ExecResult, error)

// Snapshot the VM state
func (vm *MicroVM) Snapshot(name string) error

// Restore from a snapshot
func (vm *MicroVM) Restore(snapshot string) error

// Get resource usage
func (vm *MicroVM) Usage() (*ResourceUsage, error)
```

## Integration Points

- **internal/cost**: VM resource usage tracked as cost
- **internal/trust**: Untrusted code runs in VMs
- **internal/qualitygate**: Code execution gates use VMs
- **internal/stuck**: VM resource limits prevent infinite loops
- **internal/change**: Isolated execution for testing changes

## TODO

- [ ] Firecracker integration (AWS's MicroVM runtime)
- [ ] GPU passthrough for ML workloads
- [ ] Shared filesystem between host and VM (9p/virtiofs)
- [ ] VM pooling (pre-boot VMs for instant start)
- [ ] Template marketplace (pre-built VM images for common tasks)
- [ ] Metrics export (Prometheus-compatible)

## Patent Considerations

**Novel**: The resource-limited MicroVM sandbox with automatic snapshot/restore for AI agent code execution. The network isolation model with per-VM namespaces and egress whitelisting. The VM pooling system that maintains pre-booted VMs for sub-millisecond task start.
