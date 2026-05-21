// Package openclaw provides node pairing for the Forge org.
// Forge org spans OpenClaw paired nodes — multi-device access with
// persistent context across laptop, phone, tablet, and servers.
package openclaw

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// NodeStatus represents the current state of a paired node.
type NodeStatus string

const (
	NodeOnline  NodeStatus = "online"
	NodeOffline NodeStatus = "offline"
	NodePending NodeStatus = "pending"
	NodeSyncing NodeStatus = "syncing"
)

// Node represents a paired OpenClaw node (device or server).
type Node struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Hostname    string            `json:"hostname"`
	OS          string            `json:"os"` // linux, darwin, android, ios
	Arch        string            `json:"arch"`
	Status      NodeStatus        `json:"status"`
	LastSeen    time.Time         `json:"last_seen"`
	IPAddress   string            `json:"ip_address"`
	TailnetIP   string            `json:"tailnet_ip"` // WireGuard mesh IP
	Capabilities []string         `json:"capabilities"` // browser, docker, gpu, etc.
	Labels      map[string]string `json:"labels"`
	Metadata    map[string]string `json:"metadata"`
}

// NodeManager manages paired nodes via the OpenClaw runtime.
type NodeManager struct {
	bridge *Bridge
	mu     sync.RWMutex
	nodes  map[string]*Node
}

// NewNodeManager creates a new node manager.
func NewNodeManager(bridge *Bridge) *NodeManager {
	return &NodeManager{
		bridge: bridge,
		nodes:  make(map[string]*Node),
	}
}

// List returns all paired nodes.
func (nm *NodeManager) List(ctx context.Context) ([]*Node, error) {
	var nodes []*Node
	if err := nm.bridge.GetJSON(ctx, "/api/nodes", &nodes); err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	nm.mu.Lock()
	for _, n := range nodes {
		nm.nodes[n.ID] = n
	}
	nm.mu.Unlock()
	return nodes, nil
}

// Get returns a specific node by ID or name.
func (nm *NodeManager) Get(ctx context.Context, idOrName string) (*Node, error) {
	nm.mu.RLock()
	for _, n := range nm.nodes {
		if n.ID == idOrName || n.Name == idOrName {
			nm.mu.RUnlock()
			return n, nil
		}
	}
	nm.mu.RUnlock()

	var node Node
	if err := nm.bridge.GetJSON(ctx, "/api/nodes/"+idOrName, &node); err != nil {
		return nil, fmt.Errorf("get node %s: %w", idOrName, err)
	}
	nm.mu.Lock()
	nm.nodes[node.ID] = &node
	nm.mu.Unlock()
	return &node, nil
}

// OnlineNodes returns only nodes that are currently online.
func (nm *NodeManager) OnlineNodes(ctx context.Context) ([]*Node, error) {
	nodes, err := nm.List(ctx)
	if err != nil {
		return nil, err
	}
	var online []*Node
	for _, n := range nodes {
		if n.Status == NodeOnline {
			online = append(online, n)
		}
	}
	return online, nil
}

// HasCapability checks if any online node has a specific capability.
func (nm *NodeManager) HasCapability(ctx context.Context, capability string) (bool, *Node, error) {
	nodes, err := nm.OnlineNodes(ctx)
	if err != nil {
		return false, nil, err
	}
	for _, n := range nodes {
		for _, c := range n.Capabilities {
			if c == capability {
				return true, n, nil
			}
		}
	}
	return false, nil, nil
}

// Execute runs a command on a specific node.
func (nm *NodeManager) Execute(ctx context.Context, nodeID string, command string, opts NodeExecOpts) (string, error) {
	payload := map[string]interface{}{
		"command":  command,
		"workdir":  opts.Workdir,
		"timeout":  opts.Timeout,
		"elevated": opts.Elevated,
		"env":      opts.Env,
	}
	var result struct {
		Output string `json:"output"`
		ExitCode int  `json:"exit_code"`
	}
	if err := nm.bridge.PostJSON(ctx, "/api/nodes/"+nodeID+"/exec", payload, &result); err != nil {
		return "", fmt.Errorf("exec on node %s: %w", nodeID, err)
	}
	if result.ExitCode != 0 {
		return result.Output, fmt.Errorf("exit code %d", result.ExitCode)
	}
	return result.Output, nil
}

// NodeExecOpts configures command execution on a node.
type NodeExecOpts struct {
	Workdir  string            `json:"workdir"`
	Timeout  int               `json:"timeout"` // seconds
	Elevated bool              `json:"elevated"`
	Env      map[string]string `json:"env"`
}

// Pair initiates node pairing (generates a pairing code/QR).
func (nm *NodeManager) Pair(ctx context.Context) (string, error) {
	var result struct {
		Code string `json:"code"`
		URL  string `json:"url"`
	}
	if err := nm.bridge.PostJSON(ctx, "/api/nodes/pair", nil, &result); err != nil {
		return "", fmt.Errorf("initiate pairing: %w", err)
	}
	return result.Code, nil
}

// Unpair removes a node from the paired devices.
func (nm *NodeManager) Unpair(ctx context.Context, nodeID string) error {
	if err := nm.bridge.Delete(ctx, "/api/nodes/"+nodeID); err != nil {
		return fmt.Errorf("unpair node %s: %w", nodeID, err)
	}
	nm.mu.Lock()
	delete(nm.nodes, nodeID)
	nm.mu.Unlock()
	return nil
}

// Sync triggers a sync of all node states.
func (nm *NodeManager) Sync(ctx context.Context) error {
	return nm.bridge.PostJSON(ctx, "/api/nodes/sync", nil, nil)
}

// FindByOS returns nodes matching a specific OS.
func (nm *NodeManager) FindByOS(ctx context.Context, os string) ([]*Node, error) {
	nodes, err := nm.List(ctx)
	if err != nil {
		return nil, err
	}
	var matched []*Node
	for _, n := range nodes {
		if n.OS == os && n.Status == NodeOnline {
			matched = append(matched, n)
		}
	}
	return matched, nil
}
