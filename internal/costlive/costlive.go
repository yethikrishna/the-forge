// Package costlive provides real-time cost tracking with projected monthly spend.
// It aggregates token usage from tokentracker and forecast data to show
// live burn rate, per-model/per-agent breakdowns, and cost projections.
package costlive

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/forge/sword/internal/persistence"
)

// UsageSnapshot is a single point-in-time usage measurement.
type UsageSnapshot struct {
	Timestamp    time.Time `json:"timestamp"`
	AgentID      string    `json:"agent_id"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	Cost         float64   `json:"cost"`
	Operation    string    `json:"operation"`
}

// LiveStats holds the computed live statistics.
type LiveStats struct {
	// Current session (since tracker started)
	SessionInput    int           `json:"session_input"`
	SessionOutput   int           `json:"session_output"`
	SessionCost     float64       `json:"session_cost"`
	SessionCalls    int           `json:"session_calls"`
	SessionStart    time.Time     `json:"session_start"`
	SessionDuration time.Duration `json:"session_duration"`

	// Today
	TodayInput  int     `json:"today_input"`
	TodayOutput int     `json:"today_output"`
	TodayCost   float64 `json:"today_cost"`
	TodayCalls  int     `json:"today_calls"`

	// This month
	MonthInput  int     `json:"month_input"`
	MonthOutput int     `json:"month_output"`
	MonthCost   float64 `json:"month_cost"`
	MonthCalls  int     `json:"month_calls"`

	// Burn rate (based on last hour or available data)
	TokensPerMinute float64 `json:"tokens_per_minute"`
	CostPerHour     float64 `json:"cost_per_hour"`

	// Projections
	ProjectedMonthly float64 `json:"projected_monthly"`
	ProjectedTokens  int64   `json:"projected_tokens"`
	DaysRemaining    int     `json:"days_remaining"`

	// Breakdowns
	ByModel   map[string]ModelBreakdown `json:"by_model"`
	ByAgent   map[string]AgentBreakdown `json:"by_agent"`
	TopAgents []AgentBreakdown          `json:"top_agents"`

	// Budget
	BudgetLimit     float64 `json:"budget_limit,omitempty"`
	BudgetUsed      float64 `json:"budget_used"`
	BudgetPct       float64 `json:"budget_pct"`
	BudgetRemaining float64 `json:"budget_remaining"`
}

// ModelBreakdown is cost/tokens for a specific model.
type ModelBreakdown struct {
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	Cost         float64 `json:"cost"`
	Calls        int     `json:"calls"`
	PctOfTotal   float64 `json:"pct_of_total"`
}

// AgentBreakdown is cost/tokens for a specific agent.
type AgentBreakdown struct {
	AgentID      string  `json:"agent_id"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	Cost         float64 `json:"cost"`
	Calls        int     `json:"calls"`
	PctOfTotal   float64 `json:"pct_of_total"`
}

// LiveTracker tracks real-time usage and computes live stats.
type LiveTracker struct {
	mu        sync.RWMutex
	dir       string
	snapshots []UsageSnapshot
	budget    float64 // monthly budget, 0 = no budget
	pstore    *persistence.Store
}

// NewLiveTracker creates a new live tracker.
func NewLiveTracker(dir string, monthlyBudget float64) (*LiveTracker, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create costlive dir: %w", err)
	}
	lt := &LiveTracker{
		dir:    dir,
		budget: monthlyBudget,
	}
	lt.load()

	ps, err := persistence.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("costlive: open persistence store: %w", err)
	}
	lt.pstore = ps
	ps.Register("live", func() ([]byte, error) {
		lt.mu.RLock()
		defer lt.mu.RUnlock()
		return json.MarshalIndent(lt.snapshots, "", "  ")
	})
	ps.Register("budget", func() ([]byte, error) {
		lt.mu.RLock()
		defer lt.mu.RUnlock()
		return json.MarshalIndent(map[string]float64{"budget": lt.budget}, "", "  ")
	})
	return lt, nil
}

// Close flushes pending writes and stops the background syncer.
func (lt *LiveTracker) Close() error {
	if lt.pstore != nil {
		return lt.pstore.Close()
	}
	return nil
}

// Flush forces an immediate write of all dirty keys to disk.
func (lt *LiveTracker) Flush() error {
	if lt.pstore != nil {
		return lt.pstore.Flush()
	}
	return nil
}

// Record records a usage event for live tracking.
func (lt *LiveTracker) Record(agentID, model string, inputTokens, outputTokens int, cost float64, operation string) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	lt.snapshots = append(lt.snapshots, UsageSnapshot{
		Timestamp:    time.Now().UTC(),
		AgentID:      agentID,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
		Operation:    operation,
	})
	lt.markDirty()
}

// Stats computes the current live statistics.
func (lt *LiveTracker) Stats() *LiveStats {
	lt.mu.RLock()
	defer lt.mu.RUnlock()

	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	stats := &LiveStats{
		SessionStart: now,
		ByModel:      make(map[string]ModelBreakdown),
		ByAgent:      make(map[string]AgentBreakdown),
		BudgetLimit:  lt.budget,
	}

	// Track earliest timestamp for session start
	earliest := now

	// Last hour for burn rate
	oneHourAgo := now.Add(-time.Hour)
	var recentTokens int
	var recentCount int

	for _, s := range lt.snapshots {
		// Session totals (all loaded snapshots)
		stats.SessionInput += s.InputTokens
		stats.SessionOutput += s.OutputTokens
		stats.SessionCost += s.Cost
		stats.SessionCalls++

		if s.Timestamp.Before(earliest) {
			earliest = s.Timestamp
		}

		// Today
		if !s.Timestamp.Before(todayStart) {
			stats.TodayInput += s.InputTokens
			stats.TodayOutput += s.OutputTokens
			stats.TodayCost += s.Cost
			stats.TodayCalls++
		}

		// This month
		if !s.Timestamp.Before(monthStart) {
			stats.MonthInput += s.InputTokens
			stats.MonthOutput += s.OutputTokens
			stats.MonthCost += s.Cost
			stats.MonthCalls++
		}

		// Recent (last hour)
		if !s.Timestamp.Before(oneHourAgo) {
			recentTokens += s.InputTokens + s.OutputTokens
			recentCount++
		}

		// Per-model breakdown (monthly)
		if !s.Timestamp.Before(monthStart) {
			mb := stats.ByModel[s.Model]
			mb.Model = s.Model
			mb.InputTokens += s.InputTokens
			mb.OutputTokens += s.OutputTokens
			mb.TotalTokens += s.InputTokens + s.OutputTokens
			mb.Cost += s.Cost
			mb.Calls++
			stats.ByModel[s.Model] = mb
		}

		// Per-agent breakdown (monthly)
		if !s.Timestamp.Before(monthStart) {
			ab := stats.ByAgent[s.AgentID]
			ab.AgentID = s.AgentID
			ab.InputTokens += s.InputTokens
			ab.OutputTokens += s.OutputTokens
			ab.TotalTokens += s.InputTokens + s.OutputTokens
			ab.Cost += s.Cost
			ab.Calls++
			stats.ByAgent[s.AgentID] = ab
		}
	}

	stats.SessionStart = earliest
	stats.SessionDuration = now.Sub(earliest)
	if stats.SessionDuration < time.Second {
		stats.SessionDuration = time.Second
	}

	// Burn rate from last hour (or session if < 1 hour)
	if recentCount > 0 {
		minutesElapsed := now.Sub(oneHourAgo).Minutes()
		if minutesElapsed < 1 {
			minutesElapsed = 1
		}
		stats.TokensPerMinute = float64(recentTokens) / minutesElapsed

		var recentCost float64
		for _, s := range lt.snapshots {
			if !s.Timestamp.Before(oneHourAgo) {
				recentCost += s.Cost
			}
		}
		stats.CostPerHour = recentCost // already 1-hour window
	} else if stats.SessionDuration > 0 {
		// Use session data if no recent activity
		minutesElapsed := stats.SessionDuration.Minutes()
		if minutesElapsed < 1 {
			minutesElapsed = 1
		}
		stats.TokensPerMinute = float64(stats.SessionInput+stats.SessionOutput) / minutesElapsed
		stats.CostPerHour = stats.SessionCost / stats.SessionDuration.Hours()
	}

	// Monthly projection
	daysInMonth := daysInMonth(now.Year(), int(now.Month()))
	dayOfMonth := now.Day()
	stats.DaysRemaining = daysInMonth - dayOfMonth

	if dayOfMonth > 0 && stats.MonthCost > 0 {
		dailyAvg := stats.MonthCost / float64(dayOfMonth)
		stats.ProjectedMonthly = dailyAvg * float64(daysInMonth)
		stats.ProjectedTokens = int64(float64(stats.MonthInput+stats.MonthOutput) / float64(dayOfMonth) * float64(daysInMonth))
	}

	// Compute percentages for breakdowns
	totalMonthlyTokens := stats.MonthInput + stats.MonthOutput
	totalMonthlyCost := stats.MonthCost

	for model, mb := range stats.ByModel {
		if totalMonthlyTokens > 0 {
			mb.PctOfTotal = float64(mb.TotalTokens) / float64(totalMonthlyTokens) * 100
		}
		stats.ByModel[model] = mb
	}

	for agent, ab := range stats.ByAgent {
		if totalMonthlyCost > 0 {
			ab.PctOfTotal = ab.Cost / totalMonthlyCost * 100
		}
		stats.ByAgent[agent] = ab
	}

	// Top agents by cost
	var agents []AgentBreakdown
	for _, ab := range stats.ByAgent {
		agents = append(agents, ab)
	}
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Cost > agents[j].Cost
	})
	if len(agents) > 10 {
		agents = agents[:10]
	}
	stats.TopAgents = agents

	// Budget
	if lt.budget > 0 {
		stats.BudgetUsed = stats.MonthCost
		stats.BudgetPct = stats.MonthCost / lt.budget * 100
		stats.BudgetRemaining = lt.budget - stats.MonthCost
	}

	return stats
}

// TopModels returns models sorted by cost (descending).
func (s *LiveStats) TopModels() []ModelBreakdown {
	var models []ModelBreakdown
	for _, mb := range s.ByModel {
		models = append(models, mb)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Cost > models[j].Cost
	})
	return models
}

// FormatLiveStats formats live stats for terminal output.
func FormatLiveStats(s *LiveStats) string {
	var sb strings.Builder

	// Header
	sb.WriteString("╭─────────────────────────────────────────────────╮\n")
	sb.WriteString("│           FORGE COST LIVE — Real-Time           │\n")
	sb.WriteString("╰─────────────────────────────────────────────────╯\n")
	sb.WriteString("\n")

	// Today
	fmt.Fprintf(&sb, "  Today:  %s tokens (%d in + %d out), %d calls, $%.4f\n",
		formatNumber(s.TodayInput+s.TodayOutput), s.TodayInput, s.TodayOutput, s.TodayCalls, s.TodayCost)

	// This Month
	fmt.Fprintf(&sb, "  Month:  %s tokens (%d in + %d out), %d calls, $%.4f\n",
		formatNumber(s.MonthInput+s.MonthOutput), s.MonthInput, s.MonthOutput, s.MonthCalls, s.MonthCost)

	// Burn Rate
	sb.WriteString("\n  ── Burn Rate ──\n")
	fmt.Fprintf(&sb, "  Tokens/min:  %.0f\n", s.TokensPerMinute)
	fmt.Fprintf(&sb, "  Cost/hour:   $%.4f\n", s.CostPerHour)

	// Projection
	sb.WriteString("\n  ── Monthly Projection ──\n")
	fmt.Fprintf(&sb, "  Projected:   $%.2f (%s tokens)\n", s.ProjectedMonthly, formatNumber(int(s.ProjectedTokens)))
	fmt.Fprintf(&sb, "  Days left:   %d\n", s.DaysRemaining)

	// Budget
	if s.BudgetLimit > 0 {
		sb.WriteString("\n  ── Budget ──\n")
		bar := progressBar(s.BudgetPct, 30)
		fmt.Fprintf(&sb, "  %s %.1f%%\n", bar, s.BudgetPct)
		fmt.Fprintf(&sb, "  Used: $%.4f / $%.2f (remaining: $%.4f)\n",
			s.BudgetUsed, s.BudgetLimit, s.BudgetRemaining)
		if s.BudgetPct >= 100 {
			sb.WriteString("  ⚠  BUDGET EXCEEDED\n")
		} else if s.BudgetPct >= 80 {
			sb.WriteString("  ⚠  Approaching budget limit\n")
		}
	}

	// Per-model breakdown
	models := s.TopModels()
	if len(models) > 0 {
		sb.WriteString("\n  ── By Model (this month) ──\n")
		fmt.Fprintf(&sb, "  %-25s %10s %10s %10s\n", "Model", "Tokens", "Calls", "Cost")
		fmt.Fprintf(&sb, "  %s\n", repeat("─", 58))
		for _, m := range models {
			fmt.Fprintf(&sb, "  %-25s %10s %10d $%8.4f\n",
				truncate(m.Model, 25), formatNumber(m.TotalTokens), m.Calls, m.Cost)
		}
	}

	// Top agents
	if len(s.TopAgents) > 0 {
		sb.WriteString("\n  ── Top Agents (this month) ──\n")
		fmt.Fprintf(&sb, "  %-25s %10s %10s %10s\n", "Agent", "Tokens", "Calls", "Cost")
		fmt.Fprintf(&sb, "  %s\n", repeat("─", 58))
		for _, a := range s.TopAgents {
			fmt.Fprintf(&sb, "  %-25s %10s %10d $%8.4f\n",
				truncate(a.AgentID, 25), formatNumber(a.TotalTokens), a.Calls, a.Cost)
		}
	}

	return sb.String()
}

// FormatLiveStatsJSON formats live stats as JSON.
func FormatLiveStatsJSON(s *LiveStats) (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SetBudget updates the monthly budget.
func (lt *LiveTracker) SetBudget(amount float64) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.budget = amount
	lt.markDirty()
}

// GetBudget returns the current budget setting.
func (lt *LiveTracker) GetBudget() float64 {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	return lt.budget
}

func (lt *LiveTracker) load() {
	data, err := os.ReadFile(filepath.Join(lt.dir, "live.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &lt.snapshots)

	bdata, err := os.ReadFile(filepath.Join(lt.dir, "budget.json"))
	if err == nil {
		json.Unmarshal(bdata, &lt)
	}
}

// markDirty tells the persistence store that both live and budget keys need flushing.
// Must be called with lt.mu held (write lock).
func (lt *LiveTracker) markDirty() {
	if lt.pstore != nil {
		lt.pstore.Dirty("live")
		lt.pstore.Dirty("budget")
	}
}

func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result string
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func progressBar(pct float64, width int) string {
	if pct > 100 {
		pct = 100
	}
	filled := int(math.Round(pct / 100 * float64(width)))
	bar := "["
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"
	return bar
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
