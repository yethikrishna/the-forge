package integration

import (
	"context"
	"testing"

	"github.com/forge/sword/internal/catalog"
	"github.com/forge/sword/internal/consent"
	"github.com/forge/sword/internal/costlive"
	"github.com/forge/sword/internal/crosstool"
	"github.com/forge/sword/internal/genealogy"
	"github.com/forge/sword/internal/govern"
	"github.com/forge/sword/internal/mcpgateway"
)

// TestGovernanceStackFullFlow exercises the complete governance pipeline:
// consent receipt → genealogy DAG node → governance score → catalog registration → cost tracking.
func TestGovernanceStackFullFlow(t *testing.T) {
	dir := t.TempDir()

	// ── 1. Consent receipt creation ────────────────────────────────────────────
	consentStore, err := consent.NewStore(dir + "/consent")
	if err != nil {
		t.Fatalf("consent.NewStore: %v", err)
	}

	rec, err := consentStore.Grant(
		"user-governance-test",
		"tenant-forge",
		[]consent.Purpose{
			consent.PurposeAgentExecution,
			consent.PurposeCostTracking,
			consent.PurposeAudit,
		},
		[]consent.DataCategory{
			consent.DataSourceCode,
			consent.DataAgentOutput,
			consent.DataMetrics,
		},
		consent.WithDescription("Governance integration test consent"),
		consent.WithSource("test"),
		consent.WithLegalBasis("consent"),
	)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if rec.ID == "" {
		t.Fatal("expected non-empty consent record ID")
	}
	if rec.Status != consent.StatusGranted {
		t.Errorf("expected status granted, got %s", rec.Status)
	}
	if rec.Checksum == "" {
		t.Error("expected non-empty checksum")
	}

	// Verify consent check works for the granted purposes.
	ok, found, err := consentStore.Check("user-governance-test", consent.PurposeAgentExecution)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !ok {
		t.Fatal("expected consent to be granted for agent_execution")
	}
	if found.ID != rec.ID {
		t.Errorf("Check returned wrong record: got %s want %s", found.ID, rec.ID)
	}

	// ── 2. Genealogy DAG node creation ─────────────────────────────────────────
	genealogyStore, err := genealogy.NewStore(dir + "/genealogy")
	if err != nil {
		t.Fatalf("genealogy.NewStore: %v", err)
	}

	// Root node: the human input that triggered the pipeline.
	humanNode, err := genealogyStore.AddNode(genealogy.ProvenanceNode{
		Type:        genealogy.NodeHumanInput,
		Name:        "governance-test-trigger",
		SessionID:   "sess-governance-test",
		Description: "User triggered governance integration test",
		Status:      "success",
		Metadata:    map[string]string{"consent_id": rec.ID},
	})
	if err != nil {
		t.Fatalf("AddNode (human input): %v", err)
	}

	// Agent run node derived from the human input.
	agentNode, err := genealogyStore.AddNode(genealogy.ProvenanceNode{
		Type:      genealogy.NodeAgentRun,
		Name:      "governance-agent-run",
		Agent:     "forge-governance-agent",
		Model:     "claude-3-sonnet",
		SessionID: "sess-governance-test",
		ParentIDs: []string{humanNode.ID},
		TokensIn:  512,
		TokensOut: 256,
		CostUSD:   0.0042,
		Status:    "success",
	})
	if err != nil {
		t.Fatalf("AddNode (agent run): %v", err)
	}

	// Artifact node derived from the agent run.
	artifactNode, err := genealogyStore.AddNode(genealogy.ProvenanceNode{
		Type:      genealogy.NodeArtifact,
		Name:      "governance-report.json",
		Agent:     "forge-governance-agent",
		SessionID: "sess-governance-test",
		FilePath:  "/forge/output/governance-report.json",
		ParentIDs: []string{agentNode.ID},
		Status:    "success",
	})
	if err != nil {
		t.Fatalf("AddNode (artifact): %v", err)
	}

	// Verify ancestry chain: artifact → agent → human.
	ancestry, err := genealogyStore.GetAncestry(artifactNode.ID)
	if err != nil {
		t.Fatalf("GetAncestry: %v", err)
	}
	if len(ancestry.Ancestors) < 2 {
		t.Errorf("expected at least 2 ancestors, got %d", len(ancestry.Ancestors))
	}
	foundHuman := false
	foundAgent := false
	for _, a := range ancestry.Ancestors {
		if a.ID == humanNode.ID {
			foundHuman = true
		}
		if a.ID == agentNode.ID {
			foundAgent = true
		}
	}
	if !foundHuman {
		t.Error("ancestry should include human input node")
	}
	if !foundAgent {
		t.Error("ancestry should include agent run node")
	}

	// Verify DAG stats reflect our nodes.
	dagStats, err := genealogyStore.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if dagStats.TotalNodes < 3 {
		t.Errorf("expected at least 3 nodes, got %d", dagStats.TotalNodes)
	}

	// ── 3. Governance score calculation ────────────────────────────────────────
	governStore, err := govern.NewStore(dir + "/govern")
	if err != nil {
		t.Fatalf("govern.NewStore: %v", err)
	}

	scores := map[govern.Category]int{
		govern.CatSecurity:    88,
		govern.CatCompliance:  92,
		govern.CatAudit:       95,
		govern.CatCost:        80,
		govern.CatAgentTrust:  85,
		govern.CatDataPrivacy: 90,
		govern.CatOps:         78,
		govern.CatAccess:      82,
	}

	findings := []govern.Finding{
		{
			Severity:    "medium",
			Title:       "Cost tracking not enabled for all agents",
			Category:    govern.CatCost,
			Description: "Some agents lack explicit cost tracking configuration.",
			Remediation: "Enable cost tracking middleware for all agents.",
			Status:      "open",
		},
	}

	assessment, err := governStore.Assess(govern.ReportConfig{
		Name:       "Governance Integration Test Assessment",
		Framework:  "SOC2",
		Categories: []govern.Category{
			govern.CatSecurity,
			govern.CatCompliance,
			govern.CatAudit,
			govern.CatCost,
			govern.CatAgentTrust,
			govern.CatDataPrivacy,
			govern.CatOps,
			govern.CatAccess,
		},
		TenantID: "tenant-forge",
	}, scores, findings)
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}

	if assessment.ID == "" {
		t.Fatal("expected non-empty assessment ID")
	}
	if assessment.OverallScore <= 0 || assessment.OverallScore > 100 {
		t.Errorf("OverallScore %d out of valid range [1,100]", assessment.OverallScore)
	}
	if assessment.OverallGrade == "" {
		t.Error("expected non-empty overall grade")
	}

	// Grade should be B or A given high scores.
	if assessment.OverallScore < 80 {
		t.Errorf("expected high score (≥80) given input data, got %d", assessment.OverallScore)
	}

	// Verify findings were recorded.
	if len(assessment.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(assessment.Findings))
	}

	// Export markdown report (audit-ready).
	md, err := governStore.ExportMarkdown(assessment.ID)
	if err != nil {
		t.Fatalf("ExportMarkdown: %v", err)
	}
	if len(md) == 0 {
		t.Error("expected non-empty markdown report")
	}

	// ── 4. Catalog registration ─────────────────────────────────────────────────
	catalogStore, err := catalog.NewStore(dir + "/catalog")
	if err != nil {
		t.Fatalf("catalog.NewStore: %v", err)
	}

	// Register the agent that ran the governance check.
	agentEntry, err := catalogStore.Register(catalog.Entry{
		Type:           catalog.TypeAgent,
		Name:           "governance-agent",
		Namespace:      "forge",
		Description:    "Agent responsible for governance assessments",
		Version:        "1.0.0",
		Owner:          "platform-team",
		Classification: catalog.ClassInternal,
		Tags:           []string{"governance", "compliance", "audit"},
		Labels:         map[string]string{"assessment_id": assessment.ID},
		Metadata:       map[string]string{"consent_id": rec.ID, "session_id": "sess-governance-test"},
	})
	if err != nil {
		t.Fatalf("catalog Register (agent): %v", err)
	}
	if agentEntry.ID == "" {
		t.Fatal("expected non-empty catalog entry ID")
	}
	if agentEntry.Status != catalog.StatusActive {
		t.Errorf("expected active status, got %s", agentEntry.Status)
	}
	if agentEntry.Checksum == "" {
		t.Error("expected non-empty entry checksum")
	}

	// Register the governance report as a data artifact.
	reportEntry, err := catalogStore.Register(catalog.Entry{
		Type:           catalog.TypeDataSource,
		Name:           "governance-report",
		Namespace:      "forge",
		Description:    "Governance assessment report output",
		Version:        "1.0.0",
		Owner:          "platform-team",
		Classification: catalog.ClassConfidential,
		Tags:           []string{"governance", "report"},
		Dependencies:   []string{agentEntry.ID},
		URI:            "/forge/output/governance-report.json",
		Metadata:       map[string]string{"assessment_id": assessment.ID},
	})
	if err != nil {
		t.Fatalf("catalog Register (report): %v", err)
	}

	// Verify dependency chain in catalog.
	deps, err := catalogStore.GetDependencies(reportEntry.ID)
	if err != nil {
		t.Fatalf("GetDependencies: %v", err)
	}
	if len(deps) != 1 || deps[0].ID != agentEntry.ID {
		t.Errorf("expected report to depend on agent entry, got %v", deps)
	}

	// Verify catalog stats.
	catStats := catalogStore.GetStats()
	if catStats.TotalEntries < 2 {
		t.Errorf("expected at least 2 catalog entries, got %d", catStats.TotalEntries)
	}

	// ── 5. Cost tracking ────────────────────────────────────────────────────────
	costTracker, err := costlive.NewLiveTracker(dir+"/costlive", 100.0)
	if err != nil {
		t.Fatalf("costlive.NewLiveTracker: %v", err)
	}

	// Record the agent's token usage.
	costTracker.Record(
		"forge-governance-agent",
		"claude-3-sonnet",
		int(agentNode.TokensIn),
		int(agentNode.TokensOut),
		agentNode.CostUSD,
		"governance_assessment",
	)

	// Record a second call (report generation).
	costTracker.Record(
		"forge-governance-agent",
		"claude-3-sonnet",
		128,
		64,
		0.0012,
		"report_generation",
	)

	liveStats := costTracker.Stats()
	if liveStats.SessionCalls < 2 {
		t.Errorf("expected at least 2 recorded calls, got %d", liveStats.SessionCalls)
	}
	if liveStats.SessionCost <= 0 {
		t.Errorf("expected positive session cost, got %f", liveStats.SessionCost)
	}
	if liveStats.BudgetLimit != 100.0 {
		t.Errorf("expected budget limit 100.0, got %f", liveStats.BudgetLimit)
	}

	// Verify per-agent breakdown is populated.
	agentBreakdown, ok := liveStats.ByAgent["forge-governance-agent"]
	if !ok {
		t.Error("expected agent breakdown for forge-governance-agent")
	} else {
		if agentBreakdown.Calls < 2 {
			t.Errorf("expected at least 2 calls in agent breakdown, got %d", agentBreakdown.Calls)
		}
	}

	_ = agentEntry
	_ = reportEntry
}

// TestMCPGatewayGovernanceMiddleware verifies that MCP gateway requests pass
// through the governance middleware with auth, rate limiting, and audit logging.
func TestMCPGatewayGovernanceMiddleware(t *testing.T) {
	dir := t.TempDir()

	gwConfig := mcpgateway.GatewayConfig{
		Name:    "forge-governance-gateway",
		Version: "1.0.0",
		Enabled: true,
		Auth: mcpgateway.AuthConfig{
			Method: mcpgateway.AuthToken,
			Tokens: []string{"valid-token-abc123", "valid-token-xyz789"},
		},
		RateLimit: mcpgateway.RateLimitConfig{
			RequestsPerMinute: 60,
			Burst:             10,
		},
		Validation: []mcpgateway.ValidationRule{
			{
				ToolName: "run_agent",
				Required: []string{"agent_id", "task"},
			},
			{
				ToolName:   "exec",
				Required:   []string{"command"},
				MaxPayload: 4096,
			},
		},
	}

	gw, err := mcpgateway.NewGateway(dir+"/mcpgateway", gwConfig)
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}

	// ── Test 1: Valid request passes all checks ─────────────────────────────
	resp := gw.ProcessRequest(mcpgateway.GatewayRequest{
		ClientID:   "client-cursor-1",
		RemoteAddr: "127.0.0.1:54321",
		Token:      "valid-token-abc123",
		Method:     "tools/call",
		ToolName:   "run_agent",
		Args: map[string]interface{}{
			"agent_id": "forge-governance-agent",
			"task":     "run governance check",
		},
	})
	if !resp.Allowed {
		t.Errorf("valid request should be allowed, got: %s", resp.Reason)
	}
	if resp.RequestID == "" {
		t.Error("expected non-empty request ID in response")
	}

	// ── Test 2: Missing auth token is rejected ──────────────────────────────
	resp = gw.ProcessRequest(mcpgateway.GatewayRequest{
		ClientID:   "client-unauthenticated",
		RemoteAddr: "10.0.0.1:12345",
		Token:      "",
		Method:     "tools/call",
		ToolName:   "run_agent",
		Args: map[string]interface{}{
			"agent_id": "attacker",
			"task":     "do something bad",
		},
	})
	if resp.Allowed {
		t.Error("request without token should be rejected")
	}

	// ── Test 3: Invalid token is rejected ───────────────────────────────────
	resp = gw.ProcessRequest(mcpgateway.GatewayRequest{
		ClientID:   "client-bad-token",
		RemoteAddr: "10.0.0.2:22222",
		Token:      "wrong-token",
		Method:     "tools/call",
		ToolName:   "run_agent",
		Args:       map[string]interface{}{"agent_id": "x", "task": "y"},
	})
	if resp.Allowed {
		t.Error("request with invalid token should be rejected")
	}

	// ── Test 4: Missing required argument fails validation ──────────────────
	resp = gw.ProcessRequest(mcpgateway.GatewayRequest{
		ClientID:   "client-cursor-1",
		RemoteAddr: "127.0.0.1:54321",
		Token:      "valid-token-abc123",
		Method:     "tools/call",
		ToolName:   "run_agent",
		Args: map[string]interface{}{
			// "task" is missing — required field
			"agent_id": "forge-governance-agent",
		},
	})
	if resp.Allowed {
		t.Error("request missing required arg 'task' should be rejected")
	}

	// ── Test 5: Valid second token also works ───────────────────────────────
	resp = gw.ProcessRequest(mcpgateway.GatewayRequest{
		ClientID:   "client-claude-1",
		RemoteAddr: "127.0.0.1:54322",
		Token:      "valid-token-xyz789",
		Method:     "tools/call",
		ToolName:   "exec",
		Args:       map[string]interface{}{"command": "go test ./..."},
	})
	if !resp.Allowed {
		t.Errorf("second valid token should be allowed, got: %s", resp.Reason)
	}

	// ── Verify audit log captures all requests ──────────────────────────────
	auditEntries := gw.GetAudit("", "", 0)
	if len(auditEntries) < 5 {
		t.Errorf("expected at least 5 audit entries, got %d", len(auditEntries))
	}

	// Verify denied entries are logged with the right status codes.
	authFailed := gw.GetAudit("", "auth_failed", 0)
	if len(authFailed) < 2 {
		t.Errorf("expected at least 2 auth_failed audit entries, got %d", len(authFailed))
	}

	validationFailed := gw.GetAudit("", "validation_failed", 0)
	if len(validationFailed) < 1 {
		t.Errorf("expected at least 1 validation_failed audit entry, got %d", len(validationFailed))
	}

	// ── Verify gateway stats ────────────────────────────────────────────────
	stats := gw.Stats()
	if stats.TotalRequests < 5 {
		t.Errorf("expected at least 5 total requests in stats, got %d", stats.TotalRequests)
	}
	if stats.DeniedRequests < 3 {
		t.Errorf("expected at least 3 denied requests in stats, got %d", stats.DeniedRequests)
	}
	if stats.AllowedRequests < 2 {
		t.Errorf("expected at least 2 allowed requests in stats, got %d", stats.AllowedRequests)
	}
}

// TestCrossToolBridgeThroughMCPGatewayWithAudit verifies that the cross-tool bridge
// routes requests through the MCP gateway and the request is audited correctly.
func TestCrossToolBridgeThroughMCPGatewayWithAudit(t *testing.T) {
	dir := t.TempDir()

	// ── Set up MCP gateway (no auth for bridge tests) ───────────────────────
	gwConfig := mcpgateway.GatewayConfig{
		Name:    "forge-bridge-gateway",
		Version: "1.0.0",
		Enabled: true,
		Auth: mcpgateway.AuthConfig{
			Method: mcpgateway.AuthNone,
		},
		RateLimit: mcpgateway.RateLimitConfig{
			RequestsPerMinute: 100,
			Burst:             20,
		},
	}

	gw, err := mcpgateway.NewGateway(dir+"/mcpgateway", gwConfig)
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}

	// ── Set up cross-tool bridge ────────────────────────────────────────────
	cb, err := crosstool.NewCrossBridge(dir + "/crosstool")
	if err != nil {
		t.Fatalf("NewCrossBridge: %v", err)
	}

	// Register Claude as an external tool.
	claudeInfo, err := cb.Register(crosstool.ToolClaude, crosstool.ClaudeConfig{
		APIKey: "test-api-key",
		Model:  "claude-3-sonnet",
	})
	if err != nil {
		t.Fatalf("Register claude: %v", err)
	}
	if !claudeInfo.Connected {
		t.Error("expected Claude tool to be connected")
	}

	// Register Cursor as an external tool.
	cursorInfo, err := cb.Register(crosstool.ToolCursor, crosstool.CursorConfig{
		Endpoint:  "http://localhost:9999",
		Workspace: "/workspace",
	})
	if err != nil {
		t.Fatalf("Register cursor: %v", err)
	}
	if !cursorInfo.Connected {
		t.Error("expected Cursor tool to be connected")
	}

	// ── Simulate a bridge request through the gateway ───────────────────────
	// Step 1: Client calls gateway with a "bridge" method to Cursor.
	bridgeReq := mcpgateway.GatewayRequest{
		ClientID:   "forge-main",
		RemoteAddr: "127.0.0.1:0",
		Method:     "bridge/route",
		ToolName:   "",
		Args: map[string]interface{}{
			"target": "cursor",
			"method": "code_edit",
			"file":   "main.go",
		},
	}

	gwResp := gw.ProcessRequest(bridgeReq)
	if !gwResp.Allowed {
		t.Fatalf("bridge request through gateway should be allowed, got: %s", gwResp.Reason)
	}

	// Step 2: Gateway approved — now the bridge sends to Cursor.
	// The bridge will fail to actually connect (no real Cursor running),
	// but we want to verify that the bridge message is recorded.
	ctx := context.Background()
	bridgeMsg, err := cb.SendTo(ctx, crosstool.ToolCursor, "code_edit", map[string]interface{}{
		"file":    "main.go",
		"content": "package main",
	})
	// Expected to fail (no real Cursor endpoint), but message should be in history.
	if bridgeMsg == nil {
		t.Fatal("expected bridge message to be returned even on network error")
	}
	if bridgeMsg.ID == "" {
		t.Error("expected non-empty bridge message ID")
	}
	if bridgeMsg.To != crosstool.ToolCursor {
		t.Errorf("expected message to Cursor, got %s", bridgeMsg.To)
	}
	// err is expected here (Cursor not running), just check it's from HTTP
	_ = err

	// ── Verify bridge history records the message ────────────────────────────
	history := cb.History(crosstool.ToolCursor, 0)
	if len(history) == 0 {
		t.Error("expected at least one message in bridge history")
	}
	latest := history[0] // most recent first
	if latest.Method != "code_edit" {
		t.Errorf("expected method 'code_edit', got %s", latest.Method)
	}

	// ── Verify gateway audit captured the bridge route call ─────────────────
	audit := gw.GetAudit("forge-main", "", 0)
	if len(audit) == 0 {
		t.Error("expected gateway audit entry for forge-main")
	}
	if audit[0].Method != "bridge/route" {
		t.Errorf("expected method 'bridge/route' in audit, got %s", audit[0].Method)
	}
	if audit[0].StatusCode != "ok" {
		t.Errorf("expected ok status in audit, got %s", audit[0].StatusCode)
	}

	// ── Verify bridge stats ─────────────────────────────────────────────────
	bridgeStats := cb.Stats()
	if bridgeStats.RegisteredTools != 2 {
		t.Errorf("expected 2 registered tools, got %d", bridgeStats.RegisteredTools)
	}
	if bridgeStats.TotalMessages < 1 {
		t.Errorf("expected at least 1 bridge message in stats, got %d", bridgeStats.TotalMessages)
	}

	// ── Verify capability translation ───────────────────────────────────────
	translated := crosstool.TranslateCapability(crosstool.ToolCursor, "forge", "code_edit")
	if translated != "patch" {
		t.Errorf("expected 'patch' for cursor→forge code_edit translation, got %s", translated)
	}

	// Unknown capability should pass through unchanged.
	passthrough := crosstool.TranslateCapability(crosstool.ToolCursor, "forge", "unknown_capability")
	if passthrough != "unknown_capability" {
		t.Errorf("expected passthrough for unknown capability, got %s", passthrough)
	}

	// ── Unregister a tool and verify ────────────────────────────────────────
	if err := cb.Unregister(crosstool.ToolClaude); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	_, connected := cb.Get(crosstool.ToolClaude)
	if connected {
		t.Error("Claude should not be in bridge after unregister")
	}

	afterStats := cb.Stats()
	if afterStats.RegisteredTools != 1 {
		t.Errorf("expected 1 registered tool after unregister, got %d", afterStats.RegisteredTools)
	}
}
