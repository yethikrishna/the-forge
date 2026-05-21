# 015 — Zero-Config WireGuard Mesh

> Gap: Forge Engine Infrastructure, Multi-Node

## Problem Statement

Forge agents run on multiple nodes (laptop, phone, cloud, edge devices). Connecting them requires manual VPN configuration, port forwarding, and networking expertise. Most users can't do this. Forge needs zero-config mesh networking — install Forge, nodes find each other, encrypted tunnel established. Done.

## Design Decisions

### Why WireGuard

WireGuard is:
- **Tiny codebase** (~4,000 lines vs OpenVPN's ~100,000) → auditable
- **Fast** (kernel-space, modern crypto) → low latency
- **Simple** (no config files, no certificate authorities) → automatable
- **Cross-platform** (Linux, macOS, Windows, iOS, Android) → universal

Perfect for agent mesh networking.

### Zero-Config Discovery

How do nodes find each other without configuration?
1. **Local multicast**: mDNS on LAN (finds nodes on same network)
2. **Coordination server**: Lightweight signaling server (like STUN/TURN for WebRTC)
3. **Bootstrap tokens**: User shares a token, nodes pair using it
4. **Node pairing**: QR code or short code (like WhatsApp Web)

### Full Mesh Topology

Every node connects to every other node directly (no hub-and-spoke):
```
Node A ←→ Node B
  ↕          ↕
Node C ←→ Node D
```

Benefits: no single point of failure, lowest latency, automatic failover.

### NAT Traversal

Most nodes are behind NAT. The system uses:
1. **STUN**: Detect public IP and NAT type
2. **Hole punching**: UDP hole punching for direct P2P (works 80% of the time)
3. **Relay fallback**: For symmetric NAT, use coordination server as relay
4. **UPnP**: Where available, open ports automatically

### Key Management

- Each node generates an Ed25519 keypair on first run
- Public keys exchanged during pairing
- Keys rotated every 24 hours automatically
- Compromised keys can be revoked by any paired node

## API Surface

```go
type WireGuardMesh struct { ... }

// Initialize mesh networking for this node
func NewMesh(nodeID string) (*WireGuardMesh, error)

// Generate a pairing token for another node
func (wm *WireGuardMesh) GeneratePairToken() (string, error)

// Pair with another node using a token
func (wm *WireGuardMesh) Pair(token string) (*Peer, error)

// List all connected peers
func (wm *WireGuardMesh) Peers() ([]Peer, error)

// Get tunnel statistics
func (wm *WireGuardMesh) Stats(peerID string) (*TunnelStats, error)

// Health check on peer connectivity
func (wm *WireGuardMesh) HealthCheck() ([]PeerHealth, error)
```

## Integration Points

- **Forge Engine**: Node pairing infrastructure
- **internal/crossdevice**: Device sync over mesh
- **internal/transfer**: P2P file transfer over mesh
- **internal/sandbox**: Sandbox-to-host communication

## TODO

- [ ] Coordination server implementation (signaling + relay)
- [ ] QR code generation for mobile pairing
- [ ] Mesh visualization in Forge UI
- [ ] Bandwidth optimization (compression, multiplexing)
- [ ] Split tunneling (mesh traffic vs internet traffic)
- [ ] Mesh-wide DNS (resolve agents by name)
- [ ] Federation (mesh-to-mesh for org-to-org connectivity)

## Patent Considerations

**Novel**: Zero-config WireGuard mesh for AI agent nodes with automatic discovery, NAT traversal, and key rotation. The pairing token system that enables node authentication without pre-shared configuration. The health-aware mesh that automatically reconfigures on peer failure.
